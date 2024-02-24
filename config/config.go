package config

import (
	base_config "github.com/eosnationftw/eosn-base-api/config"
)

type Config struct {
	Application *base_config.ApplicationConfig `yaml:"application" json:"application" mapstructure:"application" validate:"required"`
	SinkAddress string                         `yaml:"sink_address" json:"sink_address" mapstructure:"sink_address" validate:"required"`
}
