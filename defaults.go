package browserscale

// Defaults the SDK applies automatically before sending a request. Any field
// not listed here is left unset on the wire and follows the browserscale API's
// server-side default.

const (
	// DefaultWaitTimeoutMs is the timeout used by Wait when no
	// browserscale.Timeout(ms) option is passed.
	DefaultWaitTimeoutMs = 30000.0

	// DefaultVisible is the visibility flag baked into browserscale.CSS(...) and
	// browserscale.JS(...). For JS expressions returning a non-Element value
	// (boolean, string, number, plain object) the flag is a no-op.
	// Use .Visible(false) on the returned Locator to opt out.
	DefaultVisible = true

	// DefaultSteadyMs is the steady-time (in ms) baked into browserscale.CSS(...) and
	// browserscale.JS(...). For JS expressions returning a non-Element value the
	// value is a no-op. Use .Steady(ms) (or .Steady(0) to disable) on the
	// returned Locator to override.
	DefaultSteadyMs = 500.0
)
