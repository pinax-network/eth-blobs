package server

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/eosnationftw/eosn-base-api/log"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"google.golang.org/grpc"
)

type GrpcServer struct {
	Server *grpc.Server
	App    *App
}

func (s *GrpcServer) Initialize() {

	grpc_zap.ReplaceGrpcLoggerV2(log.ZapLogger)

	grpcZapOptions := []grpc_zap.Option{
		grpc_zap.WithLevels(func(code codes.Code) zapcore.Level {
			// This logs successful responses to the debug level, which avoids flooding the logs with empty success
			// messages.
			if code == codes.OK {
				return zap.DebugLevel
			}
			return grpc_zap.DefaultCodeToLevel(code)
		}),
	}

	// init middlewares and server
	s.Server = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_zap.UnaryServerInterceptor(log.ZapLogger, grpcZapOptions...),
			grpc_recovery.UnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_zap.StreamServerInterceptor(log.ZapLogger, grpcZapOptions...),
			grpc_recovery.StreamServerInterceptor(),
		),
	)

	// init endpoints
	grpc_health_v1.RegisterHealthServer(s.Server, health.NewServer())
	reflection.Register(s.Server)
}

func (s *GrpcServer) Run(wg *sync.WaitGroup) {

	addr := fmt.Sprintf(s.App.Config.Application.GrpcHost)
	lis, err := net.Listen("tcp", addr)
	log.FatalIfError("failed to listen on GRPC address", err)
	log.SugaredLogger.Infof("start listening for grpc requests on %s", addr)

	go func() {
		err = s.Server.Serve(lis)
		if !errors.Is(err, net.ErrClosed) {
			log.FatalIfError("failed to start grpc server", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.SugaredLogger.Infof("Shutting down GRPC server...")
	closed := make(chan bool)
	go func() {
		s.Server.GracefulStop()
		closed <- true
	}()

	select {
	case <-time.After(60 * time.Second):
		log.Fatal("GRPC Server was forced to shut down")
	case <-closed:
		log.SugaredLogger.Infof("GRPC Server was gracefully shut down")
	}

	wg.Done()
}

func SetupGrpcServer(app *App) *GrpcServer {
	server := &GrpcServer{App: app}
	server.Initialize()

	return server
}

func (s *GrpcServer) Close() {
}
