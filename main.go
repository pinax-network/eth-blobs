//go:generate swag init --output "./swagger" --parseDependency --parseDepth 3
package main

import (
	"blob-service/server"
	"sync"

	_ "blob-service/swagger"

	"github.com/eosnationftw/eosn-base-api/log"
)

//	@title			Ethereum Consensus Layer Blobs REST API
//	@version		1.0
//	@description	This is the API for Ethereum Blobs.

//	@host		localhost:8080
//	@schemes	http https
//	@BasePath	/v1

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
