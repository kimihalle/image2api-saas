package router

import (
	"testing"

	"backend/internal/config"
)

func TestRouterBuildsWithoutDuplicateRoutes(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("router construction panicked: %v", recovered)
		}
	}()
	engine := New(&config.Config{AppEnv: "test", CORSOrigins: []string{"http://localhost"}}, nil, Handlers{})
	if engine == nil || len(engine.Routes()) == 0 {
		t.Fatal("router did not register routes")
	}
}

func TestRemovedOpenPlatformRoutesStayUnregistered(t *testing.T) {
	engine := New(&config.Config{AppEnv: "test", CORSOrigins: []string{"http://localhost"}}, nil, Handlers{})
	routes := make(map[string]bool)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	removed := []string{
		"GET /admin/api/api-platform/usage",
		"GET /admin/api/webhooks",
		"GET /admin/api/webhook-deliveries",
		"GET /admin/api/workflows",
		"GET /admin/api/api-analytics",
	}
	for _, route := range removed {
		if routes[route] {
			t.Fatalf("removed open platform route is still registered: %s", route)
		}
	}

	for _, route := range []string{"POST /v1/images/generations", "POST /v1/videos", "GET /v1/models"} {
		if !routes[route] {
			t.Fatalf("core OpenAI-compatible route was removed: %s", route)
		}
	}
}
