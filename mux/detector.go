package mux

import (
	"errors"
	"net/http"

	botdetector "github.com/badboyd/krakend-botdetector"
	krakend "github.com/badboyd/krakend-botdetector/krakend"
	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/logging"
	"github.com/luraproject/lura/proxy"
	krakendmux "github.com/luraproject/lura/router/mux"
)

type middleware struct {
	f botdetector.DetectorFunc
}

// Register checks the configuration and, if required, registers a bot detector middleware at the gin engine
func NewMiddleware(cfg config.ServiceConfig, l logging.Logger) *middleware {
	detectorCfg, err := krakend.ParseConfig(cfg.ExtraConfig)
	if err == krakend.ErrNoConfig {
		l.Debug("botdetector middleware: ", err.Error())
		return nil
	}
	if err != nil {
		l.Warning("botdetector middleware: ", err.Error())
		return nil
	}
	d, err := botdetector.New(detectorCfg)
	if err != nil {
		l.Warning("botdetector middleware: unable to createt the LRU detector:", err.Error())
		return nil
	}
	return &middleware{d}
}

// New checks the configuration and, if required, wraps the handler factory with a bot detector middleware
func New(hf krakendmux.HandlerFactory, l logging.Logger) krakendmux.HandlerFactory {
	return func(cfg *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
		next := hf(cfg, p)

		detectorCfg, err := krakend.ParseConfig(cfg.ExtraConfig)
		if err == krakend.ErrNoConfig {
			l.Debug("botdetector: ", err.Error())
			return next
		}
		if err != nil {
			l.Warning("botdetector: ", err.Error())
			return next
		}

		d, err := botdetector.New(detectorCfg)
		if err != nil {
			l.Warning("botdetector: unable to create the LRU detector:", err.Error())
			return next
		}
		return handler(d, next)
	}
}

func (m *middleware) Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.f(r) {
			http.Error(w, errBotRejected.Error(), http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func handler(f botdetector.DetectorFunc, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if f(r) {
			http.Error(w, errBotRejected.Error(), http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

var errBotRejected = errors.New("bot rejected")
