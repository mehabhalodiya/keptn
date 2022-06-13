// This file is safe to edit. Once it exists it will not be overwritten

package restapi

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/benbjohnson/clock"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"

	"github.com/keptn/keptn/api/handlers"
	custommiddleware "github.com/keptn/keptn/api/middleware"
	"github.com/keptn/keptn/api/models"
	"github.com/keptn/keptn/api/restapi/operations"
	"github.com/keptn/keptn/api/restapi/operations/auth"
	"github.com/keptn/keptn/api/restapi/operations/event"
	"github.com/keptn/keptn/api/restapi/operations/metadata"
)

//go:generate swagger generate server --target ../../api --name Keptn --spec ../swagger.yaml --principal models.Principal

const envVarLogLevel = "LOG_LEVEL"

type EnvConfig struct {
	MaxAuthEnabled           bool    `envconfig:"MAX_AUTH_ENABLED" default:"true"`
	MaxAuthRequestsPerSecond float64 `envconfig:"MAX_AUTH_REQUESTS_PER_SECOND" default:"1"`
	MaxAuthRequestBurst      int     `envconfig:"MAX_AUTH_REQUESTS_BURST" default:"2"`
	OAuthEnabled             bool    `envconfig:"OAUTH_ENABLED" default:"false"`
	OAuthPrefix              string  `envconfig:"OAUTH_PREFIX" default:"keptn:"`
}

func configureFlags(api *operations.KeptnAPI) {
	// api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{ ... }
}

func getEnvConfig() (*EnvConfig, error) {
	env := &EnvConfig{}
	if err := envconfig.Process("", env); err != nil {
		return nil, err
	}
	return env, nil
}

func configureAPI(api *operations.KeptnAPI) http.Handler {
	env, err := getEnvConfig()
	if err != nil {
		log.WithError(err).Error("Failed to process env var")
		os.Exit(1)
	}

	/// configure the api here
	api.ServeError = errors.ServeError

	// Set your custom logger if needed. Default one is log.Printf
	// Expected interface func(string, ...interface{})
	//
	// Example:
	// api.Logger = log.Printf

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()

	// Applies when the "x-token" header is set
	tokenValidator := &custommiddleware.BasicTokenValidator{}
	api.KeyAuth = tokenValidator.ValidateToken

	// Set your custom authorizer if needed. Default one is security.Authorized()
	// Expected interface runtime.Authorizer
	//
	// Example:
	// api.APIAuthorizer = security.Authorized()
	api.AuthAuthHandler = auth.AuthHandlerFunc(func(params auth.AuthParams, principal *models.Principal) middleware.Responder {
		return auth.NewAuthOK()
	})

	api.EventPostEventHandler = event.PostEventHandlerFunc(handlers.PostEventHandlerFunc)
	//api.EventGetEventHandler = event.GetEventHandlerFunc(handlers.GetEventHandlerFunc)

	// Metadata endpoint
	api.MetadataMetadataHandler = metadata.MetadataHandlerFunc(handlers.GetMetadataHandlerFunc)

	//api.EvaluationTriggerEvaluationHandler = evaluation.TriggerEvaluationHandlerFunc(handlers.TriggerEvaluationHandlerFunc)

	if env.MaxAuthEnabled {
		rateLimiter := custommiddleware.NewRateLimiter(env.MaxAuthRequestsPerSecond, env.MaxAuthRequestBurst, tokenValidator, clock.New())
		api.AddMiddlewareFor(http.MethodPost, "/auth", rateLimiter.Handle)
	}

	api.ServerShutdown = func() {}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares), env)
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix"
func configureServer(s *http.Server, scheme, addr string) {
	log.SetLevel(log.InfoLevel)

	if os.Getenv(envVarLogLevel) != "" {
		logLevel, err := log.ParseLevel(os.Getenv(envVarLogLevel))
		if err != nil {
			log.WithError(err).Error("could not parse log level provided by 'LOG_LEVEL' env var")
		} else {
			log.SetLevel(logLevel)
		}
	}
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics
func setupGlobalMiddleware(handler http.Handler, env *EnvConfig) http.Handler {

	inMemoryIndex := ""

	if env.OAuthEnabled {
		var input string
		if inMemoryIndex == "" {
			b, err := ioutil.ReadFile("swagger-ui/index.html")
			if err != nil {
				fmt.Printf("Failed to set OAuth conf in index.html %v\n", err)
			} else {
				input = string(b)
			}
		} else {
			input = inMemoryIndex
		}
		inMemoryIndex = strings.Replace(input, "const oauth_prefix = \"\";", "const oauth_prefix = \""+env.OAuthPrefix+"\";", -1)
		inMemoryIndex = strings.Replace(inMemoryIndex, "const oauth_enabled = false;", "const oauth_enabled = true;", -1)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Index(r.URL.Path, "/swagger-ui/") == 0 {
			if (strings.HasSuffix(r.URL.Path, "/swagger-ui/") || strings.HasSuffix(r.URL.Path, "/swagger-ui/index.html")) && inMemoryIndex != "" {
				w.Write([]byte(inMemoryIndex))
				return
			}
			http.StripPrefix("/swagger-ui/", http.FileServer(http.Dir("swagger-ui"))).ServeHTTP(w, r)
			return
		}
		if strings.Index(r.URL.Path, "/health") == 0 {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
