package api

import (
	"net/http"
	"net/url"
	"os"
	"strings"
)

var defaultAllowedOrigins = []string{
	"https://maverick-sst.github.io",
	"http://localhost:5173",
	"http://127.0.0.1:5173",
}

// cors normalizes configured values to browser origins. A browser Origin never
// includes a path, so accepting a dashboard URL with a path (for example,
// https://example.github.io/MyApp/) must still compare by scheme and host.
func (s *Server) cors(next http.Handler) http.Handler {
	allowed := configuredOrigins(os.Getenv("OCF_ALLOWED_ORIGINS"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			normalized := normalizeOrigin(origin)
			if normalized == "" {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
			if _, ok := allowed[normalized]; !ok {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", normalized)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func configuredOrigins(value string) map[string]struct{} {
	origins := make(map[string]struct{})
	if strings.TrimSpace(value) == "" {
		for _, origin := range defaultAllowedOrigins {
			origins[origin] = struct{}{}
		}
		return origins
	}

	for _, candidate := range strings.Split(value, ",") {
		if origin := normalizeOrigin(candidate); origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return origins
}

func normalizeOrigin(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	return scheme + "://" + strings.ToLower(parsed.Host)
}
