package middleware

import "net/http"

// Chain composes middleware in the order given, so
// Chain(Logging, Recover)(handler) runs Logging first, then Recover, then
// the handler — matching how the list reads top to bottom.
func Chain(mws ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			final = mws[i](final)
		}
		return final
	}
}
