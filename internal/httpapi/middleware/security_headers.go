package middleware

import "net/http"

// SecurityHeaders sets response headers that cost nothing and close off
// common browser-side attack vectors (MIME sniffing, clickjacking via
// framing, leaking the API's URLs as a Referer to third parties). Applied
// globally since none of these interfere with a JSON API or its probes.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
