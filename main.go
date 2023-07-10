package main

import (
	"github.com/eosnationftw/eosn-base-api/log"
	"golang-service-template/server"
	"sync"
)

func main() {

	app := server.Initialize()
	wg := &sync.WaitGroup{}

	if app.Config.Application.HttpHost != "" {
		httpServer := server.SetupHttpServer(app)
		wg.Add(1)
		go httpServer.Run(wg)
	} else {
		log.Warn("no config for application.http_host available, not starting Prometheus metric exporter")
	}

	if app.Config.Application.GrpcHost != "" {
		grpcServer := server.SetupGrpcServer(app)
		wg.Add(1)
		go grpcServer.Run(wg)
	}

	wg.Wait()
}
