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
