package config

import (
	base_config "github.com/pinax-network/golang-base/config"
)

type ChainConfig struct {
	Name string `yaml:"name" json:"name" mapstructure:"name" validate:"required"`
	// FinalityLag is the number of slots a block must be behind head before
	// it is reported as `finalized` in beacon API responses. Pick a value
	// at least 2 epochs (Ethereum/Sepolia/Hoodi: 64, Gnosis: 32); choose
	// larger for extra safety margin during non-finality risk.
	FinalityLag uint64 `yaml:"finality_lag" json:"finality_lag" mapstructure:"finality_lag" validate:"required"`
}

type SinkConfig struct {
	Address string `yaml:"address" json:"address" mapstructure:"address" validate:"required"`
}

type Config struct {
	Application *base_config.ApplicationConfig `yaml:"application" json:"application" mapstructure:"application" validate:"required"`
	Sink        *SinkConfig                    `yaml:"sink" json:"sink" mapstructure:"sink" validate:"required"`
	Chain       *ChainConfig                   `yaml:"chain" json:"chain" mapstructure:"chain" validate:"required"`
}
