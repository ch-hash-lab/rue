package rue

import (
	"net/http"
	"testing"
)

// TestRouter_ParamWithDifferentSuffixes tests that routes like /:id/sync and /:id/toggle
// can coexist and be matched correctly. This was a bug where only the first registered
// route would match, and subsequent routes with different suffixes would fail.
// Validates: Requirements 1.3, 1.6
func TestRouter_ParamWithDifferentSuffixes(t *testing.T) {
	router := newRouter()
	handler := HandlersChain{func(c *Context) {}}

	// Register routes in order - both should work
	router.addRoute(http.MethodGet, "/:id/sync", handler)
	router.addRoute(http.MethodGet, "/:id/toggle", handler)

	tests := []struct {
		path      string
		wantFound bool
		wantPath  string
	}{
		{"/123/sync", true, "/:id/sync"},
		{"/123/toggle", true, "/:id/toggle"},
		{"/abc/sync", true, "/:id/sync"},
		{"/abc/toggle", true, "/:id/toggle"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			var params Params
			_, fullPath, found := router.getValue(http.MethodGet, tt.path, &params)
			if found != tt.wantFound {
				t.Errorf("Found = %v, want %v", found, tt.wantFound)
			}
			if found && fullPath != tt.wantPath {
				t.Errorf("FullPath = %s, want %s", fullPath, tt.wantPath)
			}
		})
	}
}

// TestRouter_ParamWithMultipleSuffixes tests more complex scenarios with multiple
// different suffixes after a parameter segment.
func TestRouter_ParamWithMultipleSuffixes(t *testing.T) {
	router := newRouter()
	handler := HandlersChain{func(c *Context) {}}

	// Register multiple routes with different suffixes
	router.addRoute(http.MethodGet, "/users/:id/profile", handler)
	router.addRoute(http.MethodGet, "/users/:id/settings", handler)
	router.addRoute(http.MethodGet, "/users/:id/posts", handler)
	router.addRoute(http.MethodGet, "/users/:id/followers", handler)

	tests := []struct {
		path       string
		wantFound  bool
		wantPath   string
		wantParams Params
	}{
		{"/users/123/profile", true, "/users/:id/profile", Params{{Key: "id", Value: "123"}}},
		{"/users/123/settings", true, "/users/:id/settings", Params{{Key: "id", Value: "123"}}},
		{"/users/456/posts", true, "/users/:id/posts", Params{{Key: "id", Value: "456"}}},
		{"/users/abc/followers", true, "/users/:id/followers", Params{{Key: "id", Value: "abc"}}},
		{"/users/123/unknown", false, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			var params Params
			_, fullPath, found := router.getValue(http.MethodGet, tt.path, &params)
			if found != tt.wantFound {
				t.Errorf("Found = %v, want %v", found, tt.wantFound)
			}
			if found {
				if fullPath != tt.wantPath {
					t.Errorf("FullPath = %s, want %s", fullPath, tt.wantPath)
				}
				if len(params) != len(tt.wantParams) {
					t.Errorf("Params length = %d, want %d", len(params), len(tt.wantParams))
				}
				for i, p := range tt.wantParams {
					if i < len(params) && (params[i].Key != p.Key || params[i].Value != p.Value) {
						t.Errorf("Param[%d] = %v, want %v", i, params[i], p)
					}
				}
			}
		})
	}
}
