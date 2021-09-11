package gin

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	krakend "github.com/badboyd/krakend-botdetector/krakend"
	"github.com/gin-gonic/gin"
	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/logging"
	"github.com/luraproject/lura/proxy"
	krakendgin "github.com/luraproject/lura/router/gin"
)

func TestRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.RedirectTrailingSlash = false

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

	Register(cfg, logging.NoOp, engine)

	engine.GET("/", func(c *gin.Context) {
		c.String(200, "hi!")
	})

	if err := testDetection(engine); err != nil {
		t.Error(err)
	}
}

func TestNew(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.RedirectTrailingSlash = false

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

	proxyfunc := func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
		return &proxy.Response{IsComplete: true}, nil
	}

	engine.GET("/", New(krakendgin.EndpointHandler, logging.NoOp)(cfg, proxyfunc))

	if err := testDetection(engine); err != nil {
		t.Error(err)
	}
}

func testDetection(engine *gin.Engine) error {
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
			log.Println(w.Result().StatusCode)
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
			log.Println(w.Result().StatusCode)
			return fmt.Errorf("the req #%d has not been detected as a bot: %s", i, ua)
		}
	}
	return nil
}
