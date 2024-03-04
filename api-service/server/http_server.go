package server

import (
	"blob-service/controllers"
	"blob-service/services"
	"blob-service/swagger"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/eosnationftw/eosn-base-api/helper"
	"github.com/eosnationftw/eosn-base-api/log"
	"github.com/eosnationftw/eosn-base-api/metrics"
	"github.com/eosnationftw/eosn-base-api/middleware"
	"github.com/eosnationftw/eosn-base-api/response"
	"github.com/friendsofgo/errors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
)

type HttpServer struct {
	Router *gin.Engine
	App    *App
}

func (s *HttpServer) Initialize() {

	swagger.SwaggerInfo.Host = s.App.Config.Application.Domain
	swagger.SwaggerInfo.Title = s.App.Config.Chain.Name + " Blobs REST API"
	swagger.SwaggerInfo.Description = "Use this API to get " + s.App.Config.Chain.Name + " EIP-4844 blobs as a drop-in replacement for Consensus Layer clients API."

	s.Router = gin.New()

	// error handling
	s.Router.Use(middleware.Recovery(true))
	s.Router.Use(middleware.Errors())

	// logging
	s.Router.Use(ginzap.Ginzap(log.ZapLogger, time.RFC3339, true))

	// prometheus metrics
	prometheusExporter := metrics.NewPrometheusExporter(s.Router, "/metrics")
	s.Router.Use(prometheusExporter.Instrument())

	s.Router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	s.Router.GET("/version", controllers.Version)
	s.Router.NoRoute(NoRoute)
	s.Router.NoMethod(NoMethod)

	blobsService := services.NewBlobsService(s.App.SinkClient)
	blobsController := controllers.NewBlobsController(blobsService)
	healthController := controllers.NewHealthController(blobsService)

	v1 := s.Router.Group("/eth/v1")
	v1.GET("beacon/blob_sidecars/:block_id", blobsController.BlobsByBlockId)

	s.Router.GET("/health", healthController.Health)

	s.Router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
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

func NoRoute(c *gin.Context) {
	path := c.Request.URL.Path
	helper.ReportPublicErrorAndAbort(c, response.RouteNotFound, fmt.Sprintf("path not found: %s %s", c.Request.Method, path))
}

func NoMethod(c *gin.Context) {
	helper.ReportPublicErrorAndAbort(c, response.MethodNotAllowed, fmt.Sprintf("method not allowed '%s'", c.Request.Method))
}
