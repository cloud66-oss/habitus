package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/cloud66/habitus/build"
)

type server struct {
	builder *build.Builder
}

func (s *server) StartServer() error {
	api := rest.NewApi()

	router, err := rest.MakeRouter(
		// system
		&rest.Route{"GET", "/v1/ping", s.ping},
		&rest.Route{"GET", "/v1/version", s.version},

		// v1
		&rest.Route{"GET", "/v1/secrets/:type/:name", s.serveSecret},
	)

	if err != nil {
		return err
	}

	api.SetApp(router)

	go func() {
		s.builder.Conf.Logger.Info("Starting API on %d", s.builder.Conf.ApiPort)

		// 192.168.99.1
		if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", s.builder.Conf.ApiPort), api.MakeHandler()); err != nil {
			s.builder.Conf.Logger.Error("Failed to start API %s", err.Error())
			os.Exit(2)
		}

	}()

	return nil
}

func (a *server) ping(w rest.ResponseWriter, r *rest.Request) {
	w.WriteJson("ok")
}

func (a *server) version(w rest.ResponseWriter, r *rest.Request) {
	w.WriteJson(VERSION)
}

func (a *server) serveSecret(w rest.ResponseWriter, r *rest.Request) {
	// get the provider
	provider := a.builder.Build.SecretProviders[r.PathParam("type")]
	result, err := provider.GetSecret(r.PathParam("name"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.(http.ResponseWriter).Write([]byte(result))
}
