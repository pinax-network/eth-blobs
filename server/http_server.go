package server

import (
	"blob-service/flags"
	"context"
	"fmt"
	"github.com/eosnationftw/eosn-base-api/log"
	"github.com/eosnationftw/eosn-base-api/metrics"
	"github.com/eosnationftw/eosn-base-api/response"
	"github.com/friendsofgo/errors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type HttpServer struct {
	Router *gin.Engine
	App    *App
}

func (s *HttpServer) Initialize() {

	s.Router = gin.New()

	// logging
	s.Router.Use(ginzap.Ginzap(log.ZapLogger, time.RFC3339, true))

	// prometheus metrics
	prometheusExporter := metrics.NewPrometheusExporter(s.Router, "/metrics")
	s.Router.Use(prometheusExporter.Instrument())

	s.Router.GET("/version", Version)
}

func (s *HttpServer) Run(wg *sync.WaitGroup) {
	addr := fmt.Sprintf(s.App.Config.Application.HttpHost)
	log.SugaredLogger.Infof("start listening for http requests on %s", addr)

	srv := http.Server{Addr: addr, Handler: s.Router}
	go func() {
		err := srv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			log.FatalIfError("failed to start server", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.SugaredLogger.Infof("Shutting down REST server...")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := srv.Shutdown(ctx)
	log.FatalIfError("REST Server forced to shut down ungracefully:", err)
	log.SugaredLogger.Infof("REST Server was gracefully shut down")

	wg.Done()
}

func SetupHttpServer(app *App) *HttpServer {
	server := &HttpServer{App: app}
	server.Initialize()

	return server
}

func (s *HttpServer) Close() {
}

type VersionResponse struct {
	Version  string          `json:"version"`
	Commit   string          `json:"commit"`
	Features []flags.Feature `json:"enabled_features" swaggertype:"array,string"`
}

func Version(c *gin.Context) {
	response.OkDataResponse(c, &response.ApiDataResponse{Data: &VersionResponse{
		Version:  flags.GetVersion(),
		Commit:   flags.GetShortCommit(),
		Features: flags.GetEnabledFeatures(),
	}})
}
