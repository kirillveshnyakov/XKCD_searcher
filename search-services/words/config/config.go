package config

import (
	"log"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Address  string `yaml:"words_address" env:"WORDS_ADDRESS" env-default:"localhost:80"`
	LogLevel string `yaml:"log_level" env:"LOG_LEVEL" env-default:"DEBUG"`
}

func MustLoad(configPath string) Config {
	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("cannot read config %q: %s", configPath, err)
	}
	return cfg
}
