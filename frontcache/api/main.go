package main

import (
	"context"
	"net/http"
	"os"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
	"v2.staffjoy.com/apidocs"
	"v2.staffjoy.com/environments"
	"v2.staffjoy.com/frontcache"
	"v2.staffjoy.com/healthcheck"
)

const (
	ServiceName = "frontcache"
)

var (
	logger *logrus.Entry
	config environments.Config
)

// Setup environment, logger, etc
func init() {
	// Set the ENV environment variable to control dev/stage/prod behavior
	var err error
	config, err = environments.GetConfig(os.Getenv(environments.EnvVar))
	if err != nil {
		panic("Unable to determine configuration")
	}
	logger = config.GetLogger(ServiceName)
}

func run() error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := http.NewServeMux()

	mux.HandleFunc(healthcheck.HEALTHPATH, healthcheck.Handler)
	apidocs.Serve(mux, logger)

	// Custom runtime option to emit empty fields (like false bools)
	gwmux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{OrigName: true, EmitDefaults: true}))
	opts := []grpc.DialOption{grpc.WithInsecure()}
	errEndPoint := RegisterFrontCacheServiceHandlerFromEndpoint(ctx, gwmux, frontcache.Endpoint, opts)
	if errEndPoint != nil {
		return errEndPoint
	}
	mux.Handle("/", gwmux)

	apiServerPort := ":" + os.Getenv("FRONTCACHE_API_SERVICE_PORT")
	return http.ListenAndServe(apiServerPort, mux)
}

func main() {
	logger.Debugf("Initialized frontcacheapi environment %s", config.Name)

	if err := run(); err != nil {
		logger.Fatal(err)
	}
}
