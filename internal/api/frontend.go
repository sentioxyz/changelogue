package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// dynamicRoutes maps URL path patterns to their static export HTML files.
// Next.js static export generates these with placeholder param "0".
// The order matters: more specific patterns must come first.
var dynamicRoutes = []struct {
	// segments is the split URL pattern where "*" matches any single segment.
	segments []string
	// file is the path to the pre-rendered HTML relative to webDir.
	file string
}{
	{[]string{"projects", "*", "semantic-releases", "*"}, "projects/0/semantic-releases/0.html"},
	{[]string{"projects", "*", "agent"}, "projects/0/agent.html"},
	{[]string{"projects", "*"}, "projects/0.html"},
	{[]string{"releases", "*"}, "releases/0.html"},
}

// matchDynamicRoute checks if the URL path matches a dynamic route pattern
// and returns the corresponding HTML file path, or empty string if no match.
func matchDynamicRoute(urlPath string) string {
	parts := strings.Split(strings.Trim(urlPath, "/"), "/")
	for _, route := range dynamicRoutes {
		if len(parts) != len(route.segments) {
			continue
		}
		match := true
		for i, seg := range route.segments {
			if seg != "*" && seg != parts[i] {
				match = false
				break
			}
		}
		if match {
			return route.file
		}
	}
	return ""
}

// RegisterFrontend serves the Next.js static export from webDir.
// It handles dynamic routes by mapping them to their pre-rendered HTML shells,
// which then fetch real data client-side via SWR.
func RegisterFrontend(mux *http.ServeMux, webDir string) {
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try the exact path first (static assets like JS, CSS, images)
		path := filepath.Join(webDir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		// Try path + ".html" (Next.js static export convention for static pages)
		if info, err := os.Stat(path + ".html"); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path+".html")
			return
		}
		// Try dynamic route patterns (e.g. /projects/{id} → projects/0.html)
		if file := matchDynamicRoute(r.URL.Path); file != "" {
			http.ServeFile(w, r, filepath.Join(webDir, file))
			return
		}
		// Fall back to index.html for unmatched routes
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	}))
}
