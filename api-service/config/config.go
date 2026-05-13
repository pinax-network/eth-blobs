package config

import (
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

type SinkConfig struct {
	Address string `yaml:"address" json:"address" mapstructure:"address" validate:"required"`
}

type Config struct {
	Application *base_config.ApplicationConfig `yaml:"application" json:"application" mapstructure:"application" validate:"required"`
	Sink        *SinkConfig                    `yaml:"sink" json:"sink" mapstructure:"sink" validate:"required"`
	Chain       *ChainConfig                   `yaml:"chain" json:"chain" mapstructure:"chain" validate:"required"`
}
