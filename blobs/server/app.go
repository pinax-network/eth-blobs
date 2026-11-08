package server

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	base_config "github.com/pinax-network/golang-base/config"
	"github.com/pinax-network/golang-base/log"

	"github.com/pinax-network/blob-service/blobs/config"
	"github.com/pinax-network/blob-service/blobs/store"
)

type App struct {
	Config *config.Config
	Store  store.Store
}

// Initialize loads the config file, sets up logging and connects to the store.
func Initialize(ctx context.Context, configPath string, debug bool) (*App, error) {
	if err := log.InitializeGlobalLogger(debug); err != nil {
		return nil, fmt.Errorf("initialize logger: %w", err)
	}

	var cfg config.Config
	if err := base_config.Load(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("load config %s: %w", configPath, err)
	}

	st, err := store.Open(cfg.Store.URL, cfg.Store.Timeout)
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}

	// debug flag overrides application setting
	if debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(string(cfg.Application.GinMode))
	}

	return &App{Config: &cfg, Store: st}, nil
}
