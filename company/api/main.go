package main

import (
	"net/http"
	"os"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"v2.staffjoy.com/apidocs"
	"v2.staffjoy.com/company"
	"v2.staffjoy.com/environments"
	"v2.staffjoy.com/healthcheck"

	rice "github.com/GeertJohan/go.rice"
)

const (
	// ServiceName identifies this app in logs
	ServiceName = "company"
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

	// find swagger rice.Box
	swaggerBox, err := rice.FindBox("swagger")
	if err != nil {
		panic(err)
	}

	mux.HandleFunc("/swagger.json", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")

		// get json
		tmpl, err := swaggerBox.Bytes("company.swagger.json")

		if err != nil {
			panic(err)
		}

		res.Write(tmpl)
	})

	mux.HandleFunc(healthcheck.HEALTHPATH, healthcheck.Handler)
	apidocs.Serve(mux, logger)

	// Custom runtime option to emit empty fields (like false bools)
	gwmux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{OrigName: true, EmitDefaults: true}))
	opts := []grpc.DialOption{grpc.WithInsecure()}
	if err := RegisterCompanyServiceHandlerFromEndpoint(ctx, gwmux, company.Endpoint, opts); err != nil {
		return err
	}
	mux.Handle("/", gwmux)
	// return http.ListenAndServe(":80", mux)

	apiServerPort := ":" + os.Getenv("COMPANY_API_SERVICE_PORT")
	return http.ListenAndServe(apiServerPort, mux)
}

func main() {
	logger.Debugf("Initialized companyapi environment %s", config.Name)

	if err := run(); err != nil {
		logger.Fatal(err)
	}
}
