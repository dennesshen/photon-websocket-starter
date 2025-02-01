package websocket

import "github.com/dennesshen/photon-core-starter/configuration"

func init() {
	configuration.Register(&config)
}

type Config struct {
	Websocket struct {
		BasePath           string `mapstructure:"base-path"`
		ContextPath        string `mapstructure:"context-path"`
		PingIntervalSecond int64  `mapstructure:"ping-interval-second"`
	} `mapstructure:"websocket"`
}

var config Config
