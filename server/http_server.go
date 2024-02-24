package server

import (
	"blob-service/flags"
	pbbmsrv "blob-service/pb"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/eosnationftw/eosn-base-api/helper"
	"github.com/eosnationftw/eosn-base-api/log"
	"github.com/eosnationftw/eosn-base-api/metrics"
	"github.com/eosnationftw/eosn-base-api/middleware"
	"github.com/eosnationftw/eosn-base-api/response"
	"github.com/friendsofgo/errors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/proto"
	pbkv "github.com/streamingfast/substreams-sink-kv/pb/substreams/sink/kv/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	NOT_FOUND_BLOBS = "not_found_blobs" // no blobs found
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
	prometheusExporter := metrics.NewPrometheusExporter(s.Router, "/metrics")
	s.Router.Use(prometheusExporter.Instrument())

	s.Router.GET("/version", Version)
	s.Router.GET("/blobs/by_slot/:slot", s.BlobsBySlot)
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

type BlobsResponse struct {
	Blobs []*pbbmsrv.Blob `json:"blobs"`
}

func (s *HttpServer) BlobsBySlot(c *gin.Context) {

	slot := c.Param("slot")

	prefix := "slot:" + slot + ":"

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := s.App.SinkClient.GetByPrefix(ctx, &pbkv.GetByPrefixRequest{Prefix: prefix})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			helper.ReportPublicErrorAndAbort(c, response.GatewayTimeout, err)
			return
		}
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			helper.ReportPublicErrorAndAbort(c, response.NewApiErrorNotFound(NOT_FOUND_BLOBS), err)
			return
		}
		helper.ReportPublicErrorAndAbort(c, response.BadGateway, err)
		return
	}

	blobs := &pbbmsrv.Blobs{}
	for _, kv := range resp.KeyValues {

		blob := &pbbmsrv.Blob{}
		err = proto.Unmarshal(kv.Value, blob)
		if err != nil {
			helper.ReportPublicErrorAndAbort(c, response.InternalServerError, err)
			return
		}
		blobs.Blobs = append(blobs.Blobs, blob)
	}

	response.OkDataResponse(c, &response.ApiDataResponse{Data: &BlobsResponse{
		Blobs: blobs.Blobs,
	}})
}
