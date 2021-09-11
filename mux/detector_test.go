package mux

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	krakend "github.com/badboyd/krakend-botdetector/krakend"
	gorilla "github.com/gorilla/mux"
	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/logging"
	"github.com/luraproject/lura/proxy"
	krakendmux "github.com/luraproject/lura/router/mux"
)

func TestRegister(t *testing.T) {
	cfg := config.ServiceConfig{
		Endpoints: []*config.EndpointConfig{
			{
				Endpoint: "/",
				Method:   "GET",
				Timeout:  1 * time.Second,
			},
		},
		ExtraConfig: config.ExtraConfig{
			krakend.Namespace: map[string]interface{}{
				"denylist":  []interface{}{"a", "b"},
				"allowlist": []interface{}{"c", "Pingdom.com_bot_version_1.1"},
				"patterns": []interface{}{
					`(Pingdom.com_bot_version_)(\d+)\.(\d+)`,
					`(facebookexternalhit)/(\d+)\.(\d+)`,
				},
			},
		},
	}

	engine := gorilla.NewRouter()

	bdMw := NewMiddleware(cfg, logging.NoOp)
	if bdMw != nil {
		engine.Use(bdMw.Handler)
	}

	engine.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("hi!"))
	}).Methods("GET")

	if err := testDetection(engine); err != nil {
		t.Error(err)
	}
}

func TestNew(t *testing.T) {
	cfg := &config.EndpointConfig{
		Endpoint: "/",
		Method:   "GET",
		Timeout:  1 * time.Second,
		ExtraConfig: config.ExtraConfig{
			krakend.Namespace: map[string]interface{}{
				"denylist":  []interface{}{"a", "b"},
				"allowlist": []interface{}{"c", "Pingdom.com_bot_version_1.1"},
				"patterns": []interface{}{
					`(Pingdom.com_bot_version_)(\d+)\.(\d+)`,
					`(facebookexternalhit)/(\d+)\.(\d+)`,
				},
			},
		},
	}
	engine := gorilla.NewRouter()
	proxyfunc := func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
		return &proxy.Response{IsComplete: true}, nil
	}

	engine.HandleFunc("/", New(krakendmux.EndpointHandler, logging.NoOp)(cfg, proxyfunc)).Methods("GET")

	if err := testDetection(engine); err != nil {
		t.Error(err)
	}
}

func testDetection(engine *gorilla.Router) error {
	for i, ua := range []string{
		"abcd",
		"",
		"c",
		"Pingdom.com_bot_version_1.1",
	} {
		req, _ := http.NewRequest("GET", "http://example.com/", nil)
		req.Header.Add("User-Agent", ua)

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Result().StatusCode != 200 {
			return fmt.Errorf("the req #%d has been detected as a bot: %s", i, ua)
		}
	}

	for i, ua := range []string{
		"a",
		"b",
		"facebookexternalhit/1.1",
		"Pingdom.com_bot_version_1.2",
	} {
		req, _ := http.NewRequest("GET", "http://example.com/", nil)
		req.Header.Add("User-Agent", ua)

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusForbidden {
			return fmt.Errorf("the req #%d has not been detected as a bot: %s", i, ua)
		}
	}
	return nil
}
