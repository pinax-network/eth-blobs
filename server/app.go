package server

import (
	"flag"
	base_config "github.com/eosnationftw/eosn-base-api/config"
	"github.com/eosnationftw/eosn-base-api/log"
	"github.com/gin-gonic/gin"
	"golang-service-template/config"
)

type App struct {
	Config *config.Config
}

func Initialize() *App {

	app := &App{}

	debugPtr := flag.Bool("debug", false, "debug mode")
	configPtr := flag.String("config", "./config.yaml", "config file path")
	flag.Parse()

	err := log.InitializeLogger(*debugPtr)
	log.FatalIfError("failed to initialize logger", err)

	var AppConfig config.Config
	err = base_config.Load(*configPtr, &AppConfig)
	log.FatalIfError("failed to load config file", err)
	app.Config = &AppConfig

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
