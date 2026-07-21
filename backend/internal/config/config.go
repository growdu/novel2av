// Package config loads runtime settings from environment + config file.
//
// Priority: flags > env > config file > defaults. Backed by Viper.
package config

import (
	"strings"

	"github.com/spf13/viper"
)

type S3Config struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
}

type Config struct {
	AppEnv      string  `mapstructure:"app_env"`
	HTTPAddr    string  `mapstructure:"http_addr"`
	LogLevel    string  `mapstructure:"log_level"`
	DBURL       string  `mapstructure:"db_url"`
	RedisURL    string  `mapstructure:"redis_url"`
	RedisAIURL  string  `mapstructure:"redis_ai_url"`
	S3          S3Config `mapstructure:"s3"`
	OTLPEndpoint string `mapstructure:"otlp_endpoint"`
}

// Load reads configuration from env / file. Required values cause no error here;
// callers may validate per-deployment.
func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("app_env", "dev")
	v.SetDefault("http_addr", ":8080")
	v.SetDefault("log_level", "info")
	v.SetDefault("db_url", "postgres://novel2av:novel2av@localhost:5432/novel2av?sslmode=disable")
	v.SetDefault("redis_url", "redis://localhost:6379/0")
	v.SetDefault("redis_ai_url", "redis://localhost:6379/1")

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
