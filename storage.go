package browserscale

import (
	"context"

	"github.com/browserscale/browserscale-go/generated"
)

// StorageItem is a single localStorage key/value pair.
type StorageItem struct {
	Key   string
	Value string
}

// StorageOriginEntry groups the localStorage entries of one origin
// (e.g. "https://example.com"). GetStorage returns these and SetStorage
// accepts the same shape, so a dump can be fed back verbatim.
type StorageOriginEntry struct {
	Origin string
	Items  []StorageItem
}

// GetStorage returns the localStorage contents of this session's browser
// context, grouped by origin.
//
// The storage database is read directly in the browser process, so no
// page needs to be open. Only first-party localStorage is included —
// sessionStorage is per-tab and not covered.
//
// @param origin - if non-empty, only this origin is returned
//   (e.g. "https://example.com"); empty string returns all origins
//
// @returns []StorageOriginEntry, one per origin with localStorage data
//
// @throws UNKNOWN_ERROR - the storage could not be read
//
// @example
//
//	storage, err := browser.GetStorage(ctx, "")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, e := range storage {
//	    for _, item := range e.Items {
//	        fmt.Println(e.Origin, item.Key, "=", item.Value)
//	    }
//	}
func (c *CloudBrowser) GetStorage(ctx context.Context, origin string) ([]StorageOriginEntry, error) {
	resp, err := c.client.GetStorage(ctx, &generated.GetStorageRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Origin: strPtr(origin),
	})
	if err != nil {
		return nil, err
	}
	return storageFromProto(resp.Storage), nil
}

// SetStorage writes localStorage entries into the browser context,
// grouped by origin.
//
// Accepts the same structure GetStorage returns, so a dump can be fed
// back verbatim. Existing keys are overwritten. Works without any open
// page; pages that are already open will not observe the writes until
// they reload.
//
// @param storage - entries to write, grouped by origin
//
// @throws UNKNOWN_ERROR - the storage could not be written
//
// @example
//
//	_ = browser.SetStorage(ctx, []browserscale.StorageOriginEntry{
//	    {
//	        Origin: "https://example.com",
//	        Items: []browserscale.StorageItem{
//	            {Key: "token", Value: "abc123"},
//	            {Key: "theme", Value: "dark"},
//	        },
//	    },
//	})
func (c *CloudBrowser) SetStorage(ctx context.Context, storage []StorageOriginEntry) error {
	_, err := c.client.SetStorage(ctx, &generated.SetStorageRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Storage: storageToProto(storage),
	})
	return err
}

// ClearStorage deletes localStorage in the browser context.
//
// @param origin - if non-empty, only this origin's storage is deleted
//   (e.g. "https://example.com"); empty string deletes all origins
//
// @throws UNKNOWN_ERROR - the storage could not be cleared
//
// @example
//
//	// Wipe one origin.
//	_ = browser.ClearStorage(ctx, "https://example.com")
//
//	// Wipe everything.
//	_ = browser.ClearStorage(ctx, "")
func (c *CloudBrowser) ClearStorage(ctx context.Context, origin string) error {
	_, err := c.client.ClearStorage(ctx, &generated.ClearStorageRequest{
		SessionId: c.sessionId, ApiKey: c.apiKey,
		Origin: strPtr(origin),
	})
	return err
}
