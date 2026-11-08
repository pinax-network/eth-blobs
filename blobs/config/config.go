package config

import (
	"time"

	base_config "github.com/pinax-network/golang-base/config"
)

type ChainConfig struct {
	Name string `yaml:"name" json:"name" mapstructure:"name" validate:"required"`
	// FinalityLag is the number of slots a block must be behind head before
	// it is reported as `finalized` in beacon API responses. When nil (key
	// absent in config), DefaultFinalityLag is used. Set explicitly to 0
	// to always report finalized=true (e.g. for chains where finality
	// isn't tracked, or for testing). Typical values: Ethereum mainnet 64,
	// Gnosis 32.
	FinalityLag *uint64 `yaml:"finality_lag" json:"finality_lag" mapstructure:"finality_lag"`
}

// DefaultFinalityLag is 2 epochs on Ethereum mainnet — used when
// chain.finality_lag is unset in the config.
const DefaultFinalityLag uint64 = 64

// StoreConfig is where the sink writes. Credentials come from the backend's
// environment (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY,
// GOOGLE_APPLICATION_CREDENTIALS, ...), never from this file.
type StoreConfig struct {
	// URL is a dstore URL, e.g. s3://host:9000/blobs-eth?region=none&insecure=true,
	// gs://bucket/prefix or file:///var/lib/blobs. See store.Open.
	URL string `yaml:"url" json:"url" mapstructure:"url" validate:"required"`
	// Timeout bounds each store operation. Zero uses store.DefaultTimeout.
	Timeout time.Duration `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
	// HeadTTL bounds how long the API reuses a fetched head before asking
	// the store again. Zero uses DefaultHeadTTL.
	HeadTTL time.Duration `yaml:"head_ttl" json:"head_ttl" mapstructure:"head_ttl"`
}

// DefaultHeadTTL keeps head lookups to a few per slot even under probe load.
const DefaultHeadTTL = 2 * time.Second

type Config struct {
	Application *base_config.ApplicationConfig `yaml:"application" json:"application" mapstructure:"application" validate:"required"`
	Store       *StoreConfig                   `yaml:"store" json:"store" mapstructure:"store" validate:"required"`
	Chain       *ChainConfig                   `yaml:"chain" json:"chain" mapstructure:"chain" validate:"required"`
}
