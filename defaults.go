package browserscale

// The SDK no longer applies its own defaults: every optional field is left
// unset on the wire and the browserscale API supplies the default, so there is
// one place that defines them. The constants below record the current
// server-side values for reference and are kept only for compatibility.

const (
	// DefaultWaitTimeoutMs is the timeout the API applies to Wait when no
	// browserscale.Timeout(ms) option is passed.
	//
	// Deprecated: the API owns this value; the SDK no longer sends it.
	DefaultWaitTimeoutMs = 30000.0

	// DefaultVisible is the visibility requirement the API applies to a wait
	// condition that does not set one. Use .Visible(false) on a Locator to wait
	// for DOM presence alone.
	//
	// Deprecated: the API owns this value; the SDK no longer sends it.
	DefaultVisible = true

	// DefaultSteadyMs is the steady-time (in ms) the API applies to a wait
	// condition that does not set one. Use .Steady(0) on a Locator to match the
	// instant the element is found.
	//
	// Deprecated: the API owns this value; the SDK no longer sends it.
	DefaultSteadyMs = 500.0
)
