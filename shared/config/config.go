package config

import (
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App   AppConfig
	HTTP  HTTPConfig
	GRPC  GRPCConfig
	Mongo MongoConfig
	Redis RedisConfig
	JWT   JWTConfig
	OAuth OAuthConfig
	AWS   AWSConfig
}

type AppConfig struct {
	Name string `mapstructure:"name"`
}

type HTTPConfig struct {
	Port string `mapstructure:"port"`
}

type GRPCConfig struct {
	Port string `mapstructure:"port"`
}

type MongoConfig struct {
	URI string `mapstructure:"mongo_uri"`
	DB  string `mapstructure:"mongo_db"`
}

type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
}

type JWTConfig struct {
	SecretKey       string        `mapstructure:"secret_key"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
}

type OAuthConfig struct {
	GoogleClientID     string `env:"OAUTH_GOOGLE_CLIENT_ID" validate:"required"`
	GoogleClientSecret string `env:"OAUTH_GOOGLE_CLIENT_SECRET" validate:"required"`
}

type AWSConfig struct {
	Region    string `env:"AWS_REGION" validate:"required"`
	S3Bucket  string `env:"AWS_S3_BUCKET" validate:"required"`
	AccessKey string `env:"AWS_ACCESS_KEY" validate:"required"`
	SecretKey string `env:"AWS_SECRET_KEY" validate:"required"`
}

func LoadConfig(path string) (*Config, error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	log.Println("Configuration loaded successfully")
	return &cfg, nil
}
