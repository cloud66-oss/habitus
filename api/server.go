package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/cloud66/habitus/configuration"
	"github.com/cloud66/habitus/secrets"
)

var (
	VERSION string = "dev"
)

type Server struct {
	Conf            *configuration.Server
	secretProviders map[string]secrets.SecretProvider
}

func (s *Server) StartServer(version string) error {
	VERSION = version
	secret_api := rest.NewApi()

	s.secretProviders = secrets.GetProviders()

	if s.Conf.UseAuthenticatedSecretServer {
		secret_api.Use(&rest.AuthBasicMiddleware{
			Realm: "Habitus secret service",
			Authenticator: func(userId string, password string) bool {
				if userId == s.Conf.AuthenticatedSecretServerUser && password == s.Conf.AuthenticatedSecretServerPassword {
					return true
				}
				return false
			},
		})
	}

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

	secret_api.SetApp(router)

	go func() {
		s.Conf.Logger.Infof("Starting API on %d", s.Conf.ApiPort)

		// 192.168.99.1
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", s.Conf.ApiBinding, s.Conf.ApiPort), secret_api.MakeHandler()); err != nil {
			s.Conf.Logger.Errorf("Failed to start API %s", err.Error())
			os.Exit(2)
		}

	}()

	return nil
}

func (a *Server) ping(w rest.ResponseWriter, r *rest.Request) {
	w.WriteJson("ok")
}

func (a *Server) version(w rest.ResponseWriter, r *rest.Request) {
	w.WriteJson(VERSION)
}

func (a *Server) serveSecret(w rest.ResponseWriter, r *rest.Request) {
	// get the provider
	provider := a.secretProviders[r.PathParam("type")]
	result, err := provider.GetSecret(r.PathParam("name"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.(http.ResponseWriter).Write([]byte(result))
}
