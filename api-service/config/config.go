package config

import (
	base_config "github.com/pinax-network/golang-base/config"
)

type ChainConfig struct {
	Name string `yaml:"name" json:"name" mapstructure:"name" validate:"required"`
}

type SinkConfig struct {
	Address string `yaml:"address" json:"address" mapstructure:"address" validate:"required"`
}

type Config struct {
	Application *base_config.ApplicationConfig `yaml:"application" json:"application" mapstructure:"application" validate:"required"`
	Sink        *SinkConfig                    `yaml:"sink" json:"sink" mapstructure:"sink" validate:"required"`
	Chain       *ChainConfig                   `yaml:"chain" json:"chain" mapstructure:"chain" validate:"required"`
}
