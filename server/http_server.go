package server

import (
	"blob-service/controllers"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	pbkv "github.com/streamingfast/substreams-sink-kv/pb/substreams/sink/kv/v1"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

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

	s.Router = gin.New()

	// error handling
	s.Router.Use(middleware.Recovery(true))
	s.Router.Use(middleware.Errors())

	// logging
	s.Router.Use(ginzap.Ginzap(log.ZapLogger, time.RFC3339, true))

	// prometheus metrics
	prometheusExporter := metrics.NewPrometheusExporter(s.Router, "/v1/metrics")
	s.Router.Use(prometheusExporter.Instrument())

	s.Router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	s.Router.GET("/version", controllers.Version)
	s.Router.NoRoute(NoRoute)
	s.Router.NoMethod(NoMethod)

	blobsController := controllers.NewBlobsController(s.App.SinkClient)

	v1 := s.Router.Group("/v1")
	v1.GET("/blobs/by_slot/:slot", blobsController.BlobsBySlot)
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

func ConnectToSinkServer(sinkServerAddress string) pbkv.KvClient {
	conn, err := grpc.Dial(
		sinkServerAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1024*1024*1024),
			grpc.WaitForReady(true),
		),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                5 * time.Minute, // send pings every ... when there is no activity
			Timeout:             5 * time.Second, // wait that amount of time for ping ack before considering the connection dead
			PermitWithoutStream: false,
		}),
	)
	if err != nil {
		log.Fatalf("failed to connect to the sink server: %v", err)
	}

	return pbkv.NewKvClient(conn)
}
