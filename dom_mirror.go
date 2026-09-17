package browserscale

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/browserscale/browserscale-go/generated"
)

// ──────────────────────────────────────────────────────────────────────
// DOM mirror
// ──────────────────────────────────────────────────────────────────────

// DomNode is a node in the mirrored page, in CDP's DOM.Node shape — the same
// shape [CloudBrowser.GetDOM] returns, with two additions the mirror needs and
// a caller usually wants anyway: DomNode.FrameId and DomNode.ContentFrameId.
//
// An <iframe> is an ordinary element here. The document it hosts is its one
// entry in Children, present once the element has been expanded, and nothing
// about walking the tree has to know a process boundary runs through it.
//
// Nodes are immutable once handed out. The mirror applies a change by replacing
// the nodes from the root down to the one that moved, so a tree you took from
// [DomMirror.Root] stays a consistent snapshot while the mirror moves on, and
// unchanged subtrees keep their identity. Do not modify them.
type DomNode struct {
	NodeId        int32  `json:"nodeId"`
	BackendNodeId int32  `json:"backendNodeId"`
	NodeType      int32  `json:"nodeType"`
	NodeName      string `json:"nodeName"`
	LocalName     string `json:"localName,omitempty"`
	NodeValue     string `json:"nodeValue,omitempty"`

	// Attributes is flat [name, value, name, value, ...], as CDP sends it.
	Attributes []string `json:"attributes,omitempty"`

	// ChildNodeCount is the total children in the page, whether or not they
	// are in Children. An <iframe> reports 1: the document it hosts.
	ChildNodeCount int32 `json:"childNodeCount,omitempty"`

	// Children is non-nil once the node has been expanded — empty and non-nil
	// for a node that is expanded and has none.
	Children []*DomNode `json:"children,omitempty"`

	// ShadowRoots holds author shadow roots, when the mirror was started with
	// DomMirrorOptions.Pierce.
	ShadowRoots []*DomNode `json:"shadowRoots,omitempty"`

	// FrameId is the frame this node lives in. Always set.
	//
	// Together with BackendNodeId this is the node's address: node ids are
	// handed out per renderer and restart per frame, so two frames can and do
	// use the same one, and the id on its own is ambiguous across a page.
	FrameId string `json:"frameId"`

	// ContentFrameId is set on an element that hosts a frame (<iframe>,
	// <frame>, <object>): the frame it hosts, which is a different frame from
	// FrameId and is the one its child document's ids belong to.
	ContentFrameId string `json:"contentFrameId,omitempty"`
}

// rawDomNode is the shape the browser sends, before the mirror normalizes it.
type rawDomNode struct {
	NodeId         int32         `json:"nodeId"`
	BackendNodeId  int32         `json:"backendNodeId"`
	NodeType       int32         `json:"nodeType"`
	NodeName       string        `json:"nodeName"`
	LocalName      string        `json:"localName"`
	NodeValue      string        `json:"nodeValue"`
	Attributes     []string      `json:"attributes"`
	ChildNodeCount int32         `json:"childNodeCount"`
	Children       []*rawDomNode `json:"children"`
	ShadowRoots    []*rawDomNode `json:"shadowRoots"`

	// FrameId on a document is its own frame; on a frame owner element it is
	// the frame it HOSTS. One name for two relationships, which is why the
	// mirror splits it into FrameId and ContentFrameId before handing a node
	// out.
	FrameId string `json:"frameId"`
}

// domNodeTypeDocument is CDP's Node.DOCUMENT_NODE. A document is where a
// node-id address space begins, which is the only thing the mirror needs the
// node type for.
const domNodeTypeDocument int32 = 9

// DomSnapshot is the opening snapshot: the main frame's document. Child frames
// are not in it — their documents are fetched by expanding the <iframe>
// elements that host them, which is also what starts mirroring them.
type DomSnapshot struct {
	// Root is the main frame's document as CDP DOM.Node-shaped JSON, the same
	// format GetDOM returns.
	Root string
	// FrameId is the page's main frame.
	FrameId string
	// Seq is the page sequence this snapshot is the baseline for. Every update
	// after it carries a higher one.
	Seq uint64
}

// DomChildren is the reply to [CloudBrowser.GetDomChildren].
type DomChildren struct {
	// Children is a JSON array of DOM.Node. Empty for an id the mirror never
	// handed out, which is also what a stale id from before a resync looks
	// like.
	Children string
	// Seq is the page sequence this payload is valid as of.
	Seq uint64
}

// DomPath is the reply to [CloudBrowser.RevealDomNode].
type DomPath struct {
	// Path is a JSON array of DOM.Node, the main document first, each carrying
	// one level of children, crossing into frames at the document nodes along
	// it. Empty if the node is not on the page.
	Path string
	// Seq is the page sequence this payload is valid as of.
	Seq uint64
}

// DomMirrorOptions configures [CloudBrowser.MirrorDom] and
// [CloudBrowser.StartDomMirror].
type DomMirrorOptions struct {
	// Depth is how many levels to serialize up front. 0 uses the server
	// default of 2 — #document → <html> → <head>/<body>, enough to draw a
	// collapsed tree. -1 walks everything and gives up what the mirror is for.
	Depth int32

	// Pierce descends into author shadow roots. Fixed for the life of the
	// mirror.
	Pierce bool
}

// DomChangeHandler is called after the mirrored tree changed, including once
// for the opening snapshot. Read the new state from [DomMirror.Root].
//
// Calls are serialized on a goroutine the SDK owns, so the handler needs no
// locking of its own, and it is safe to call back into the mirror from it.
// Calls are also coalesced: several changes in quick succession may produce a
// single call, which always sees the newest tree. Treat it as "something moved,
// re-read the root" rather than as one call per edit.
type DomChangeHandler func(*DomMirror)

// DomResyncHandler is called when the mirror had to be rebuilt, after the new
// tree is already in place. Rebuilding is automatic; this exists to tell a user
// why their expanded nodes collapsed.
//
// The reason is one of "documentReplaced", "overflow", "rendererGone",
// "slowReader" or "manual", and is worth treating as an open set.
type DomResyncHandler func(reason string)

// How hard to try to rebuild the page after the browser voids it. A void
// usually means the page is navigating, and the window where the old document
// is gone and the new one cannot be serialized yet is short.
const (
	domResyncAttempts   = 4
	domResyncRetryDelay = 150 * time.Millisecond
)

type domSlotKind uint8

const (
	domSlotChildren domSlotKind = iota
	domSlotShadowRoots
)

// domSlot is where a node hangs in its parent, which is what lets a change to
// one node replace the chain of copies above it.
type domSlot struct {
	parent string
	kind   domSlotKind
}

// domKey addresses a node. Node ids are renderer-local, so the frame is part of
// the address.
func domKey(frameId string, backendNodeId int32) string {
	return frameId + "#" + strconv.FormatInt(int64(backendNodeId), 10)
}

// domEdit is one entry of a DomUpdate batch. The shapes are documented on the
// domUpdate event in WRC.pdl; this is the only place that decodes them, so all
// variants share one struct.
type domEdit struct {
	Type           string      `json:"type"`
	ParentId       int32       `json:"parentId"`
	PreviousNodeId int32       `json:"previousNodeId"`
	NodeId         int32       `json:"nodeId"`
	Node           *rawDomNode `json:"node"`
	Count          int32       `json:"count"`
	Order          []int32     `json:"order"`
	Attributes     []string    `json:"attributes"`
	Value          string      `json:"value"`
}

// target is the node an edit is about, which for the two child-list entries is
// the PARENT: that is the node whose state they change, and whose watermark
// therefore decides whether they have already been folded in.
func (e *domEdit) target() int32 {
	switch e.Type {
	case "childNodeInserted", "childNodeRemoved":
		return e.ParentId
	default:
		return e.NodeId
	}
}

// DomMirror is a live copy of a page's DOM, across every frame in it, returned
// by [CloudBrowser.MirrorDom].
//
// The browser sends the top of the tree once and from then on only what changed
// in the part you expanded, so a page that churns inside a collapsed subtree
// costs one number per batch instead of a re-serialized document.
//
// It is one tree. An <iframe> is an element whose one child is the document it
// hosts; expanding it fetches that document and starts mirroring the frame,
// collapsing it stops again, and a frame navigating arrives as its owner's
// child being replaced.
//
// Node ids restart per frame, so a node's address is the pair
// (DomNode.FrameId, DomNode.BackendNodeId) and never the id alone.
//
// Every method is safe to call from any goroutine.
type DomMirror struct {
	browser  *CloudBrowser
	stream   generated.Browser_StreamDomEventsClient
	cancel   context.CancelFunc
	options  DomMirrorOptions
	onChange DomChangeHandler
	onResync DomResyncHandler

	// ctx covers the background calls the mirror makes on its own, which is
	// the resync after the browser voids the page. Cancelled by Stop.
	ctx context.Context

	// finished is closed once the reader goroutine has returned.
	finished chan struct{}

	// notifyCh carries change notifications to the single goroutine that runs
	// onChange. Capacity 1: a pending notification already covers whatever
	// happened since, because the handler re-reads the root.
	notifyCh   chan struct{}
	notifyDone chan struct{}

	mu       sync.Mutex
	nodes    map[string]*DomNode
	slots    map[string]domSlot
	expanded map[string]bool

	// asOf records, per node, the page sequence its current state was defined
	// at, by a read payload or by an edit.
	//
	// Reads and events reach a client over two different channels — a unary
	// call and a stream — so an event can turn up that the read reply already
	// folded in. Applying it twice would duplicate an insertion, which is the
	// one entry type that is not idempotent. Comparing against the node's own
	// watermark rather than a single page-wide one keeps that from silently
	// discarding a change to an unrelated part of the tree.
	asOf map[string]uint64

	seq       uint64
	root      *DomNode
	mainFrame string

	// ready is false until a snapshot exists to apply events to. Events that
	// arrive before it are held and replayed, because a batch that lands
	// between subscribing and the snapshot arriving would otherwise be applied
	// to nothing.
	ready   bool
	pending []*generated.DomEvent

	// resyncNeeded is set by the apply path, which runs under mu and therefore
	// cannot do the RPC a resync needs. The reader picks it up after
	// unlocking.
	resyncNeeded string

	stopped bool
	err     error
}

// MirrorDom starts mirroring the session's page and returns a live copy of its
// DOM.
//
// One mirror covers the whole page as ONE tree. An <iframe> is an ordinary
// element whose single child is the document it hosts; expanding it fetches that
// document and starts mirroring the frame, however deeply nested and whether or
// not it is cross-origin. Unlike the inlining [CloudBrowser.GetDOM] does, these
// regions stay live — and frames nobody opened cost nothing.
//
// This replaces polling [CloudBrowser.GetDOMHash] and re-fetching GetDOM: the
// browser reports changes to the part you actually expanded instead of
// re-serializing the document so you can hash it.
//
// The subscription is established before the snapshot is taken, so no change
// between the two is lost. Call [DomMirror.Stop] when done — it stops the mirror
// server-side, which a cancelled context alone does not.
//
// @param opts - initial depth and whether to pierce shadow roots
// @param onChange - called after every change, including the first snapshot;
//
//	see [DomChangeHandler] for the coalescing and threading rules
//
// @param onResync - called when the copy had to be rebuilt; may be nil
//
// @returns *DomMirror holding the tree
//
// @throws UNKNOWN_ERROR - onChange is nil, or the mirror could not be started
//
// @example
//
//	mirror, err := browser.MirrorDom(ctx, browserscale.DomMirrorOptions{Pierce: true},
//	    func(m *browserscale.DomMirror) {
//	        render(m.Root())
//	    }, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer mirror.Stop(ctx)
//
//	body := mirror.Node(mirror.MainFrameId(), bodyId)
//	_ = mirror.Expand(ctx, body, 0)
func (c *CloudBrowser) MirrorDom(ctx context.Context, opts DomMirrorOptions, onChange DomChangeHandler, onResync DomResyncHandler) (*DomMirror, error) {
	if onChange == nil {
		return nil, errors.New("browserscale.MirrorDom: onChange must not be nil")
	}
	streamCtx, cancel := context.WithCancel(ctx)
	stream, err := c.client.StreamDomEvents(streamCtx, &generated.StreamDomEventsRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	// Opening a stream does not wait for the server to start handling it. The
	// server sends its headers once subscribed; blocking for them makes the
	// order deterministic. This matters more here than for network capture: a
	// batch missed before the subscription exists is not replayed, so the tree
	// would be silently wrong rather than merely short.
	if _, err := stream.Header(); err != nil {
		cancel()
		return nil, err
	}

	m := &DomMirror{
		browser:    c,
		stream:     stream,
		cancel:     cancel,
		options:    opts,
		onChange:   onChange,
		onResync:   onResync,
		ctx:        streamCtx,
		finished:   make(chan struct{}),
		notifyCh:   make(chan struct{}, 1),
		notifyDone: make(chan struct{}),
		nodes:      make(map[string]*DomNode),
		slots:      make(map[string]domSlot),
		expanded:   make(map[string]bool),
		asOf:       make(map[string]uint64),
	}
	go m.pump()
	go m.notifier()

	// Snapshot only now: taken before the subscription exists, changes between
	// the two would be lost with nothing to indicate it.
	snapshot, err := c.StartDomMirror(ctx, opts)
	if err != nil {
		cancel()
		<-m.finished
		<-m.notifyDone
		return nil, err
	}
	if err := m.install(snapshot); err != nil {
		cancel()
		<-m.finished
		<-m.notifyDone
		return nil, err
	}
	m.notify()
	return m, nil
}

// StartDomMirror starts (or restarts) the page's mirror and returns the main
// document, without subscribing to changes. [CloudBrowser.MirrorDom] is what
// you normally want; this is the raw command.
//
// Calling it again restarts the mirror, which is also the recovery path after a
// resync. Subscribe before calling it: changes between the snapshot and the
// subscription are not replayed.
//
// @param opts - initial depth and whether to pierce shadow roots
//
// @returns *DomSnapshot with the main document, its frame and the baseline
//
//	sequence
//
// @throws UNKNOWN_ERROR - the mirror could not be started
func (c *CloudBrowser) StartDomMirror(ctx context.Context, opts DomMirrorOptions) (*DomSnapshot, error) {
	req := &generated.StartDomMirrorRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Depth: intPtr(opts.Depth),
	}
	if opts.Pierce {
		pierce := true
		req.Pierce = &pierce
	}
	resp, err := c.client.StartDomMirror(ctx, req)
	if err != nil {
		return nil, err
	}
	return &DomSnapshot{Root: resp.Root, FrameId: resp.FrameId, Seq: resp.Seq}, nil
}

// StopDomMirror stops the page's mirror, every frame of it. Idempotent.
//
// @throws UNKNOWN_ERROR - the mirror could not be stopped
func (c *CloudBrowser) StopDomMirror(ctx context.Context) error {
	_, err := c.client.StopDomMirror(ctx, &generated.StopDomMirrorRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
	})
	return err
}

// GetDomChildren fetches a node's children and starts reporting changes inside
// them. [DomMirror.Expand] calls this and folds the result into the tree.
//
// On an <iframe>/<frame>/<object> the one child is the document it hosts, and
// this call is what starts mirroring that frame.
//
// @param backendNodeId - the node to open
// @param frameId - the frame its id belongs to; empty targets the main frame
// @param depth - levels below the node; 0 uses the server default of 1
//
// @returns *DomChildren with the child list as JSON and the sequence it is
//
//	valid as of
//
// @throws UNKNOWN_ERROR - the children could not be read
func (c *CloudBrowser) GetDomChildren(ctx context.Context, backendNodeId int32, frameId string, depth int32) (*DomChildren, error) {
	resp, err := c.client.GetDomChildren(ctx, &generated.GetDomChildrenRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		BackendNodeId: backendNodeId,
		FrameId:       strPtr(frameId),
		Depth:         intPtr(depth),
	})
	if err != nil {
		return nil, err
	}
	return &DomChildren{Children: resp.Children, Seq: resp.Seq}, nil
}

// ReleaseDomSubtree stops reporting changes inside a node, and inside any frame
// below it. [DomMirror.Collapse] calls this.
//
// Skipping it is not an error, it is a slow leak: the browser's revealed set
// only grows, and eventually it is no longer filtering anything.
//
// @param backendNodeId - the node to close
// @param frameId - the frame its id belongs to; empty targets the main frame
//
// @throws UNKNOWN_ERROR - the subtree could not be released
func (c *CloudBrowser) ReleaseDomSubtree(ctx context.Context, backendNodeId int32, frameId string) error {
	_, err := c.client.ReleaseDomSubtree(ctx, &generated.ReleaseDomSubtreeRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		BackendNodeId: backendNodeId,
		FrameId:       strPtr(frameId),
	})
	return err
}

// RevealDomNode returns the chain from the main document down to a node, each
// ancestor with its own children, crossing into frames where it has to and
// starting the ones it passes through. [DomMirror.Reveal] calls this and
// splices it in.
//
// @param backendNodeId - the node to reach
// @param frameId - the frame its id belongs to; empty targets the main frame
//
// @returns *DomPath with the ancestor chain as JSON and the sequence it is
//
//	valid as of
//
// @throws UNKNOWN_ERROR - the path could not be built
func (c *CloudBrowser) RevealDomNode(ctx context.Context, backendNodeId int32, frameId string) (*DomPath, error) {
	resp, err := c.client.RevealDomNode(ctx, &generated.RevealDomNodeRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		BackendNodeId: backendNodeId,
		FrameId:       strPtr(frameId),
	})
	if err != nil {
		return nil, err
	}
	return &DomPath{Path: resp.Path, Seq: resp.Seq}, nil
}

// GetDomRevision returns a frame's mutation counter, incremented on every
// change the document sees. O(1) in the browser and the change detector to poll
// if you are not consuming mirror events.
//
// Prefer it over [CloudBrowser.GetDOMHash], which serializes the whole tree
// just to hash it. The two answer different questions: a hash compares content,
// a revision only says whether this document moved since you last asked. The
// counter is meaningful only within the current document.
//
// @param frameId - the frame to ask; empty targets the main frame
//
// @returns uint64 monotonic counter
//
// @throws UNKNOWN_ERROR - the revision could not be read
func (c *CloudBrowser) GetDomRevision(ctx context.Context, frameId string) (uint64, error) {
	resp, err := c.client.GetDomRevision(ctx, &generated.GetDomRevisionRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		FrameId: strPtr(frameId),
	})
	if err != nil {
		return 0, err
	}
	return resp.Revision, nil
}

// ── reads ─────────────────────────────────────────────────────────────

// Root returns the main frame's document, or nil before the first snapshot
// arrived.
//
// The result is an immutable snapshot: the mirror will not modify the nodes it
// hands out, so it stays consistent to walk while the page keeps changing.
func (m *DomMirror) Root() *DomNode {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.root
}

// MainFrameId returns the page's main frame.
func (m *DomMirror) MainFrameId() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mainFrame
}

// FrameIds returns every frame with a document in the tree, main frame first. A
// frame whose <iframe> has not been expanded is not mirrored and not listed.
func (m *DomMirror) FrameIds() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	seen := make(map[string]bool, len(m.nodes))
	others := make([]string, 0, 4)
	for _, node := range m.nodes {
		if node.NodeType != domNodeTypeDocument || node.FrameId == m.mainFrame || seen[node.FrameId] {
			continue
		}
		seen[node.FrameId] = true
		others = append(others, node.FrameId)
	}
	sort.Strings(others)

	out := make([]string, 0, len(others)+1)
	if m.mainFrame != "" {
		out = append(out, m.mainFrame)
	}
	return append(out, others...)
}

// Seq returns the page sequence of the last change applied. One clock for the
// whole page: a change in an out-of-process iframe and one in the main document
// are ordered against each other.
func (m *DomMirror) Seq() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.seq
}

// Node looks up a node by its address, or nil if the mirror does not hold it.
func (m *DomMirror) Node(frameId string, backendNodeId int32) *DomNode {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nodes[domKey(frameId, backendNodeId)]
}

// IsExpanded reports whether this node's children are known. Changes inside a
// node that is not expanded arrive only as an updated ChildNodeCount.
func (m *DomMirror) IsExpanded(node *DomNode) bool {
	if node == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.expanded[domKey(node.FrameId, node.BackendNodeId)]
}

// Err reports why the mirror ended. It returns nil while it is still running,
// and after a clean stop, a cancelled context, or the session ending normally.
func (m *DomMirror) Err() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.err
}

// ── writes ────────────────────────────────────────────────────────────

// Expand fetches a node's children and starts reporting changes inside them.
// This is what a tree view calls when the user opens a node.
//
// On an <iframe> the one child is the document it hosts, and this call is what
// starts mirroring that frame. Nothing about the result says a process boundary
// was crossed; it is a child list like any other.
//
// A node that left the tree while the call was in flight is not an error and
// changes nothing.
//
// @param node - the node to open, from [DomMirror.Root] or [DomMirror.Node]
// @param depth - levels below the node; 0 uses the server default of 1
//
// @throws UNKNOWN_ERROR - the children could not be read
func (m *DomMirror) Expand(ctx context.Context, node *DomNode, depth int32) error {
	if node == nil {
		return errors.New("browserscale.DomMirror.Expand: node must not be nil")
	}
	if m.isStopped() {
		return nil
	}
	frame := node.FrameId
	key := domKey(frame, node.BackendNodeId)

	reply, err := m.browser.GetDomChildren(ctx, node.BackendNodeId, frame, depth)
	if err != nil {
		return err
	}
	kids, err := parseDomNodes(reply.Children)
	if err != nil {
		return err
	}

	m.mu.Lock()
	// The node can be gone by the time the reply lands — a resync or a removal
	// in between rebuilt the part of the tree this describes.
	if _, ok := m.nodes[key]; !ok {
		m.mu.Unlock()
		return nil
	}
	fresh := m.touch(key)
	m.dropChildren(fresh)
	list := make([]*DomNode, 0, len(kids))
	for _, kid := range kids {
		list = append(list, m.adopt(kid, &domSlot{parent: key, kind: domSlotChildren}, frame, reply.Seq))
	}
	fresh.Children = list
	fresh.ChildNodeCount = int32(len(list))
	m.expanded[key] = true
	m.mark(key, reply.Seq)
	m.mu.Unlock()

	m.notify()
	return nil
}

// Collapse stops reporting changes inside a node, called when the user closes
// it. The node itself stays in the tree and keeps reporting its child count. A
// child frame below it stops being mirrored too.
//
// @param node - the node to close
//
// @throws UNKNOWN_ERROR - the subtree could not be released
func (m *DomMirror) Collapse(ctx context.Context, node *DomNode) error {
	if node == nil {
		return errors.New("browserscale.DomMirror.Collapse: node must not be nil")
	}
	if m.isStopped() {
		return nil
	}
	key := domKey(node.FrameId, node.BackendNodeId)
	if err := m.browser.ReleaseDomSubtree(ctx, node.BackendNodeId, node.FrameId); err != nil {
		return err
	}

	m.mu.Lock()
	if _, ok := m.nodes[key]; !ok {
		m.mu.Unlock()
		return nil
	}
	fresh := m.touch(key)
	m.dropChildren(fresh)
	fresh.Children = nil
	delete(m.expanded, key)
	m.mu.Unlock()

	m.notify()
	return nil
}

// Reveal brings a node into the tree together with its ancestors and their
// siblings, and starts reporting changes along that path.
//
// Use it to focus a node you do not hold — an [CloudBrowser.InspectAtPosition]
// hit, say. You cannot walk up to it yourself: it is not in your tree, so there
// is nothing to walk from.
//
// The node may be in a frame nobody opened, and that works: the chain comes back
// crossing the frame boundaries it has to, and those frames start being
// mirrored, exactly as if you had expanded your way there by hand.
//
// @param backendNodeId - the node to reach
// @param frameId - the frame its id belongs to; empty targets the main frame
//
// @returns []*DomNode the ancestor chain, the main document first, or nil if the
//
//	node is not on the page
//
// @throws UNKNOWN_ERROR - the path could not be built
func (m *DomMirror) Reveal(ctx context.Context, backendNodeId int32, frameId string) ([]*DomNode, error) {
	if m.isStopped() {
		return nil, nil
	}
	frame := frameId
	if frame == "" {
		frame = m.MainFrameId()
	}
	reply, err := m.browser.RevealDomNode(ctx, backendNodeId, frame)
	if err != nil {
		return nil, err
	}
	path, err := parseDomNodes(reply.Path)
	if err != nil {
		return nil, err
	}
	if len(path) == 0 {
		return nil, nil
	}

	m.mu.Lock()
	// The chain starts at the main document, which the mirror already holds, so
	// it is merged level by level rather than re-rooted. Re-rooting would throw
	// away the rest of the page, including every other frame opened into it.
	keys := make([]string, 0, len(path))
	space := m.mainFrame
	above := ""
	for _, step := range path {
		// A document names its own frame, and everything after it counts in
		// that frame's ids. This is the boundary, and it is the only place the
		// address space changes.
		if step.NodeType == domNodeTypeDocument && step.FrameId != "" {
			space = step.FrameId
		}
		key := domKey(space, step.BackendNodeId)

		if _, ok := m.nodes[key]; !ok {
			// The only step that can be missing is a hosted document: every
			// other one arrived in the child list of the step above it. Hanging
			// it off its owner is what crossing into the frame means here.
			if above == "" {
				break
			}
			if _, ok := m.nodes[above]; !ok {
				break
			}
			owner := m.touch(above)
			m.dropChildren(owner)
			owner.Children = []*DomNode{
				m.adopt(step, &domSlot{parent: above, kind: domSlotChildren}, space, reply.Seq),
			}
			owner.ChildNodeCount = 1
			m.expanded[above] = true
			m.mark(above, reply.Seq)
		}

		m.mergeChildren(key, step, space, reply.Seq)
		keys = append(keys, key)
		above = key
	}

	out := make([]*DomNode, 0, len(keys))
	for _, key := range keys {
		if node, ok := m.nodes[key]; ok {
			out = append(out, node)
		}
	}
	m.mu.Unlock()

	m.notify()
	return out, nil
}

// Resync throws away the local copy of the whole page and fetches a fresh one.
// It happens automatically whenever the browser says the copy is void, so you
// rarely need to call it.
//
// @throws UNKNOWN_ERROR - the page could not be re-read; the mirror then ends
func (m *DomMirror) Resync(ctx context.Context) error {
	return m.resync(ctx, "manual")
}

// Wait blocks until the mirror ends — [DomMirror.Stop], a cancelled context, a
// dead session or a transport failure — and returns [DomMirror.Err].
func (m *DomMirror) Wait() error {
	<-m.finished
	return m.Err()
}

// Stop stops mirroring and detaches the reader. Idempotent, and safe to defer.
//
// Once it returns, the change handler is no longer running and everything it
// wrote is visible to the calling goroutine.
//
// To stop from inside the handler, call [CloudBrowser.StopDomMirror] instead:
// Stop waits for the handler to return, so calling it from there would wait on
// itself until ctx expires.
//
// ctx covers the call that stops the mirror server-side, so pass a live one: the
// context the mirror was created with may already be cancelled by the time you
// stop.
//
// @throws UNKNOWN_ERROR - the mirror could not be stopped server-side; the local
//
//	reader is shut down regardless
func (m *DomMirror) Stop(ctx context.Context) error {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return nil
	}
	m.stopped = true
	m.mu.Unlock()

	err := m.browser.StopDomMirror(ctx)
	m.cancel()
	select {
	case <-m.finished:
	case <-ctx.Done():
	}
	select {
	case <-m.notifyDone:
	case <-ctx.Done():
	}
	return err
}

func (m *DomMirror) isStopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
}

// ── event stream ──────────────────────────────────────────────────────

// pump reads the gRPC stream and folds each batch into the tree.
func (m *DomMirror) pump() {
	defer close(m.finished)
	for {
		event, err := m.stream.Recv()
		if err != nil {
			if !isStreamEnd(err) {
				m.fail(err)
			}
			return
		}

		// A resync is handled before the batches behind it, so those are
		// applied to the tree it produces rather than to the one it just
		// invalidated.
		if r := event.GetResync(); r != nil {
			if err := m.resync(m.ctx, r.Reason); err != nil {
				m.fail(err)
				// Ending the mirror is the point: a caller watching Wait can
				// say the tree is frozen, where staying quietly broken looks
				// exactly like a page on which nothing is happening.
				m.cancel()
				return
			}
			continue
		}

		m.mu.Lock()
		changed := m.handle(event)
		reason := m.resyncNeeded
		m.resyncNeeded = ""
		m.mu.Unlock()

		if reason != "" {
			if err := m.resync(m.ctx, reason); err != nil {
				m.fail(err)
				m.cancel()
				return
			}
			continue
		}
		if changed {
			m.notify()
		}
	}
}

// notifier runs onChange, one call at a time, off the reader goroutine. Running
// it here rather than inline is what makes the handler safe to call back into
// the mirror from.
func (m *DomMirror) notifier() {
	defer close(m.notifyDone)
	for {
		select {
		case <-m.notifyCh:
			m.onChange(m)
		case <-m.finished:
			// Deliver the last pending notification, so the final state of the
			// tree is never left unreported.
			select {
			case <-m.notifyCh:
				m.onChange(m)
			default:
			}
			return
		}
	}
}

func (m *DomMirror) notify() {
	select {
	case m.notifyCh <- struct{}{}:
	default:
		// A notification is already queued; it will see this change too.
	}
}

func (m *DomMirror) fail(err error) {
	m.mu.Lock()
	if m.err == nil && !m.stopped {
		m.err = err
	}
	m.mu.Unlock()
}

// handle folds one batch into the tree. Runs under mu and returns whether
// anything changed.
func (m *DomMirror) handle(event *generated.DomEvent) bool {
	if !m.ready {
		m.pending = append(m.pending, event)
		return false
	}
	batch := event.GetUpdate()
	if batch == nil {
		return false
	}
	frame := batch.FrameId
	if frame == "" {
		frame = m.mainFrame
	}
	seq := batch.Seq

	var entries []*domEdit
	if err := json.Unmarshal([]byte(orEmptyArray(batch.Edits)), &entries); err != nil {
		m.resyncNeeded = "overflow"
		return false
	}

	// Which entries to keep is decided against the watermarks as they stand
	// BEFORE the batch, and the watermarks are moved only afterwards. Entries
	// within a batch share its sequence, and a frame swap is exactly a removal
	// and an insertion on the same owner in one batch — judging the second
	// against a watermark the first just raised would drop it.
	keep := make([]*domEdit, 0, len(entries))
	touched := make([]string, 0, len(entries))
	for _, entry := range entries {
		key := domKey(frame, entry.target())
		if w, ok := m.asOf[key]; ok && w >= seq {
			continue
		}
		keep = append(keep, entry)
		touched = append(touched, key)
	}
	for _, entry := range keep {
		m.apply(entry, frame, seq)
	}
	for _, key := range touched {
		m.mark(key, seq)
	}

	if seq > m.seq {
		m.seq = seq
	}
	return len(keep) > 0
}

// apply folds one entry into the tree. Runs under mu.
func (m *DomMirror) apply(entry *domEdit, frame string, seq uint64) {
	switch entry.Type {
	case "childNodeInserted":
		if entry.Node == nil {
			return
		}
		parentKey := domKey(frame, entry.ParentId)
		if _, ok := m.nodes[parentKey]; !ok {
			return
		}
		fresh := m.touch(parentKey)
		// The browser only reports insertions for nodes it expanded. If we do
		// not have a child list for one, it had none when we last saw it, so an
		// empty list is the right thing to grow from.
		kids := fresh.Children
		node := m.adopt(entry.Node, &domSlot{parent: parentKey, kind: domSlotChildren}, frame, seq)
		at := 0
		if entry.PreviousNodeId != 0 {
			for i, c := range kids {
				if c.BackendNodeId == entry.PreviousNodeId {
					at = i + 1
					break
				}
			}
		}
		kids = append(kids, nil)
		copy(kids[at+1:], kids[at:])
		kids[at] = node
		fresh.Children = kids
		fresh.ChildNodeCount = int32(len(kids))
		m.expanded[parentKey] = true

	case "childNodeRemoved":
		parentKey := domKey(frame, entry.ParentId)
		parent := m.nodes[parentKey]
		if parent == nil || parent.Children == nil {
			return
		}
		fresh := m.touch(parentKey)
		kids := fresh.Children
		// Matched on the id alone, not the pair. When the child is a hosted
		// document the id is in ITS frame, not the parent's — but a frame owner
		// has exactly one child, so there is nothing to confuse it with.
		at := -1
		for i, c := range kids {
			if c.BackendNodeId == entry.NodeId {
				at = i
				break
			}
		}
		if at < 0 {
			return
		}
		m.forget(kids[at])
		kids = append(kids[:at], kids[at+1:]...)
		fresh.Children = kids
		fresh.ChildNodeCount = int32(len(kids))

	case "childNodeCountUpdated":
		key := domKey(frame, entry.NodeId)
		if _, ok := m.nodes[key]; !ok {
			return
		}
		m.touch(key).ChildNodeCount = entry.Count

	case "childListReordered":
		key := domKey(frame, entry.NodeId)
		parent := m.nodes[key]
		if parent == nil || parent.Children == nil {
			return
		}
		fresh := m.touch(key)
		have := make(map[int32]*DomNode, len(fresh.Children))
		for _, c := range fresh.Children {
			have[c.BackendNodeId] = c
		}
		next := make([]*DomNode, 0, len(entry.Order))
		for _, id := range entry.Order {
			if child, ok := have[id]; ok {
				next = append(next, child)
			}
		}
		// The browser sends the complete order, so a mismatch means our copy
		// drifted. Keeping the strays would hide that; rebuilding is the only
		// honest option.
		if len(next) != len(have) {
			m.resyncNeeded = "overflow"
			return
		}
		fresh.Children = next

	case "attributesUpdated":
		key := domKey(frame, entry.NodeId)
		if _, ok := m.nodes[key]; !ok {
			return
		}
		m.touch(key).Attributes = append([]string(nil), entry.Attributes...)

	case "characterDataModified":
		key := domKey(frame, entry.NodeId)
		if _, ok := m.nodes[key]; !ok {
			return
		}
		m.touch(key).NodeValue = entry.Value
	}
}

// ── snapshot installation ─────────────────────────────────────────────

// install replaces the tree with a fresh snapshot and replays whatever arrived
// while it was in flight.
func (m *DomMirror) install(snapshot *DomSnapshot) error {
	var parsed *rawDomNode
	if snapshot.Root != "" {
		parsed = new(rawDomNode)
		if err := json.Unmarshal([]byte(snapshot.Root), parsed); err != nil {
			return err
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.nodes = make(map[string]*DomNode)
	m.slots = make(map[string]domSlot)
	m.expanded = make(map[string]bool)
	m.asOf = make(map[string]uint64)
	m.root = nil

	if snapshot.FrameId != "" {
		m.mainFrame = snapshot.FrameId
	}
	m.seq = snapshot.Seq
	if parsed != nil {
		m.root = m.adopt(parsed, nil, m.mainFrame, snapshot.Seq)
	}
	m.ready = true

	// Anything buffered while the snapshot was in flight is either already in
	// it (seq <= snapshot) or genuinely newer. Either way the watermark check in
	// handle decides, so replaying the buffer here is safe.
	buffered := m.pending
	m.pending = nil
	for _, event := range buffered {
		m.handle(event)
	}
	// A replayed batch asking for a resync is dropped rather than honoured: the
	// tree it wanted rebuilt is the one we just built.
	m.resyncNeeded = ""
	return nil
}

// resync rebuilds the page after the browser voids it.
func (m *DomMirror) resync(ctx context.Context, reason string) error {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return nil
	}
	m.ready = false
	opts := m.options
	m.mu.Unlock()

	// A resync that fails leaves nothing usable behind: the local copy is
	// already declared void and no more events will make sense against it. The
	// common cause is transient — the page was voided because it is navigating,
	// and the new document is not there yet to be serialized — so this retries
	// before treating it as fatal.
	var snapshot *DomSnapshot
	var last error
	for attempt := 0; attempt < domResyncAttempts; attempt++ {
		if m.isStopped() {
			return nil
		}
		if attempt > 0 {
			select {
			case <-time.After(domResyncRetryDelay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		s, err := m.browser.StartDomMirror(ctx, opts)
		if err == nil {
			snapshot = s
			break
		}
		last = err
	}
	if snapshot == nil {
		return last
	}
	if err := m.install(snapshot); err != nil {
		return err
	}

	if m.onResync != nil {
		m.onResync(reason)
	}
	m.notify()
	return nil
}

// ── tree bookkeeping ──────────────────────────────────────────────────

// mark records that a node's state is current as of seq. Never moves back.
func (m *DomMirror) mark(key string, seq uint64) {
	if w, ok := m.asOf[key]; !ok || seq > w {
		m.asOf[key] = seq
	}
	if seq > m.seq {
		m.seq = seq
	}
}

// touch replaces key and every ancestor with copies, so the path from the root
// to the changed node has new identities and nothing else does. It returns the
// fresh copy of key, which the caller then edits in place — which is safe only
// because every caller holds mu for the whole edit, so no reader can observe a
// copy mid-change.
//
// The walk crosses frame boundaries without noticing them: a document is a child
// of the <iframe> hosting it like any other, so a change deep inside an
// out-of-process frame still produces a new root.
func (m *DomMirror) touch(key string) *DomNode {
	original := m.nodes[key]
	if original == nil {
		return nil
	}
	clone := *original
	if original.Children != nil {
		clone.Children = append([]*DomNode(nil), original.Children...)
	}
	if original.ShadowRoots != nil {
		clone.ShadowRoots = append([]*DomNode(nil), original.ShadowRoots...)
	}
	fresh := &clone
	m.nodes[key] = fresh

	childKey := key
	childNode := fresh
	for {
		slot, ok := m.slots[childKey]
		if !ok {
			m.root = childNode
			return fresh
		}
		parent := m.nodes[slot.parent]
		if parent == nil {
			return fresh
		}
		parentClone := *parent

		list := parent.Children
		if slot.kind == domSlotShadowRoots {
			list = parent.ShadowRoots
		}
		list = append([]*DomNode(nil), list...)
		for i, n := range list {
			if n.BackendNodeId == childNode.BackendNodeId && n.FrameId == childNode.FrameId {
				list[i] = childNode
				break
			}
		}
		if slot.kind == domSlotShadowRoots {
			parentClone.ShadowRoots = list
		} else {
			parentClone.Children = list
		}

		m.nodes[slot.parent] = &parentClone
		childKey = slot.parent
		childNode = &parentClone
	}
}

// adopt registers a payload subtree and returns the copy that lives in the tree.
//
// frame is the id space the payload's ids belong to, and it changes here and
// nowhere else: a document node names its own frame, and everything below it
// counts in that frame. That is the whole of what crossing into an iframe means
// to a client.
func (m *DomMirror) adopt(node *rawDomNode, slot *domSlot, frame string, seq uint64) *DomNode {
	// The browser puts the frame's own id on a document and the HOSTED frame's
	// id on a frame owner element. Same field, two relationships — which is
	// which is decided by the node type, and only one of them can be called
	// FrameId without making "which frame is this node in" ambiguous.
	isDocument := node.NodeType == domNodeTypeDocument
	own := frame
	if isDocument && node.FrameId != "" {
		own = node.FrameId
	}

	key := domKey(own, node.BackendNodeId)
	out := &DomNode{
		NodeId:         node.NodeId,
		BackendNodeId:  node.BackendNodeId,
		NodeType:       node.NodeType,
		NodeName:       node.NodeName,
		LocalName:      node.LocalName,
		NodeValue:      node.NodeValue,
		Attributes:     node.Attributes,
		ChildNodeCount: node.ChildNodeCount,
		FrameId:        own,
	}
	if !isDocument && node.FrameId != "" && node.FrameId != own {
		out.ContentFrameId = node.FrameId
	}

	m.nodes[key] = out
	m.mark(key, seq)
	if slot != nil {
		m.slots[key] = *slot
	} else {
		delete(m.slots, key)
	}

	if node.Children != nil {
		kids := make([]*DomNode, 0, len(node.Children))
		for _, child := range node.Children {
			kids = append(kids, m.adopt(child, &domSlot{parent: key, kind: domSlotChildren}, own, seq))
		}
		out.Children = kids
		m.expanded[key] = true
	} else {
		delete(m.expanded, key)
	}

	if node.ShadowRoots != nil {
		roots := make([]*DomNode, 0, len(node.ShadowRoots))
		for _, root := range node.ShadowRoots {
			roots = append(roots, m.adopt(root, &domSlot{parent: key, kind: domSlotShadowRoots}, own, seq))
		}
		out.ShadowRoots = roots
	}

	return out
}

// mergeChildren replaces a node's child list from a fresh payload for the same
// node.
func (m *DomMirror) mergeChildren(key string, payload *rawDomNode, frame string, seq uint64) {
	if payload.Children == nil {
		return
	}
	if _, ok := m.nodes[key]; !ok {
		return
	}
	fresh := m.touch(key)
	m.dropChildren(fresh)
	kids := make([]*DomNode, 0, len(payload.Children))
	for _, child := range payload.Children {
		kids = append(kids, m.adopt(child, &domSlot{parent: key, kind: domSlotChildren}, frame, seq))
	}
	fresh.Children = kids
	fresh.ChildNodeCount = int32(len(kids))
	m.expanded[key] = true
	m.mark(key, seq)
}

func (m *DomMirror) dropChildren(node *DomNode) {
	if node == nil {
		return
	}
	for _, child := range node.Children {
		m.forget(child)
	}
}

// forget removes a subtree from the index. It descends through hosted documents
// like through anything else — they are children, and a frame stops being
// mirrored exactly when the element hosting it stops being expanded.
func (m *DomMirror) forget(node *DomNode) {
	key := domKey(node.FrameId, node.BackendNodeId)
	delete(m.nodes, key)
	delete(m.slots, key)
	delete(m.expanded, key)
	delete(m.asOf, key)
	for _, child := range node.Children {
		m.forget(child)
	}
	for _, root := range node.ShadowRoots {
		m.forget(root)
	}
}

// parseDomNodes decodes a JSON array of DOM.Node, treating an empty payload as
// an empty array — which is what an id the mirror never handed out returns.
func parseDomNodes(payload string) ([]*rawDomNode, error) {
	var nodes []*rawDomNode
	if err := json.Unmarshal([]byte(orEmptyArray(payload)), &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

func orEmptyArray(payload string) string {
	if payload == "" {
		return "[]"
	}
	return payload
}
