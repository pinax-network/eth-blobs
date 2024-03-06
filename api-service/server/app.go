package server

import (
	"blob-service/config"
	"blob-service/internal"
	"flag"

	"github.com/gin-gonic/gin"
	base_config "github.com/pinax-network/golang-base/config"
	"github.com/pinax-network/golang-base/log"
	pbkv "github.com/streamingfast/substreams-sink-kv/pb/substreams/sink/kv/v1"
)

type App struct {
	Config     *config.Config
	SinkClient pbkv.KvClient
}

func Initialize() *App {

	app := &App{}

	debugPtr := flag.Bool("debug", false, "debug mode")
	configPtr := flag.String("config", "./config.yaml", "config file path")
	flag.Parse()

	err := log.InitializeGlobalLogger(*debugPtr)
	log.FatalIfError("failed to initialize logger", err)

	var AppConfig config.Config
	err = base_config.Load(*configPtr, &AppConfig)
	log.FatalIfError("failed to load config file", err)
	app.Config = &AppConfig

	app.SinkClient = internal.ConnectToSinkServer(app.Config.Sink.Address)

	// debug flag overrides application setting
	if *debugPtr {
		gin.SetMode("debug")
	} else {
		gin.SetMode(string(app.Config.Application.GinMode))
	}

	return app
}

func (a *App) Close() {
}
