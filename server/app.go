package server

import (
	"blob-service/config"
	"flag"

	base_config "github.com/eosnationftw/eosn-base-api/config"
	"github.com/eosnationftw/eosn-base-api/log"
	"github.com/gin-gonic/gin"
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

	app.SinkClient = ConnectToSinkServer(app.Config.SinkAddress)

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
