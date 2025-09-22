package config

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig              `mapstructure:"app"`
	HTTP     HTTPConfig             `mapstructure:"http"`
	GRPC     GRPCConfig             `mapstructure:"grpc"`
	Mongo    MongoConfig            `mapstructure:"mongo"`
	Redis    RedisConfig            `mapstructure:"redis"`
	JWT      JWTConfig              `mapstructure:"jwt"`
	OAuth    OAuthConfig            `mapstructure:"oauth"`
	AWS      AWSConfig              `mapstructure:"aws"`
	Kafka    KafkaConfig            `mapstructure:"kafka"`
	Log      LogConfig              `mapstructure:"log"`
	Services map[string]ServiceAddr `mapstructure:"services"`
}

type AppConfig struct {
	Name        string `mapstructure:"name" validate:"required"`
	Environment string `mapstructure:"environment" validate:"required,oneof=development staging production"`
	Version     string `mapstructure:"version"`
}

type HTTPConfig struct {
	Port            string        `mapstructure:"port" validate:"required"`
	Host            string        `mapstructure:"host"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type GRPCConfig struct {
	Port              string        `mapstructure:"port" validate:"required"`
	Host              string        `mapstructure:"host"`
	MaxReceiveSize    int           `mapstructure:"max_receive_size"`
	MaxSendSize       int           `mapstructure:"max_send_size"`
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`
	EnableReflection  bool          `mapstructure:"enable_reflection"`
	EnableHealthCheck bool          `mapstructure:"enable_health_check"`
}

type MongoConfig struct {
	URI             string        `mapstructure:"uri" validate:"required"`
	Database        string        `mapstructure:"database" validate:"required"`
	MaxPoolSize     uint64        `mapstructure:"max_pool_size"`
	MinPoolSize     uint64        `mapstructure:"min_pool_size"`
	ConnectTimeout  time.Duration `mapstructure:"connect_timeout"`
	ServerSelection time.Duration `mapstructure:"server_selection_timeout"`
}

type RedisConfig struct {
	Host         string        `mapstructure:"host" validate:"required"`
	Port         string        `mapstructure:"port" validate:"required"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type JWTConfig struct {
	SecretKey       string        `mapstructure:"secret_key" validate:"required,min=32"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
	Issuer          string        `mapstructure:"issuer"`
	Audience        string        `mapstructure:"audience"`
}

type OAuthConfig struct {
	GoogleClientID     string `mapstructure:"google_client_id" validate:"required"`
	GoogleClientSecret string `mapstructure:"google_client_secret" validate:"required"`
	GoogleRedirectURL  string `mapstructure:"google_redirect_url" validate:"required,url"`
}

type AWSConfig struct {
	Region      string `mapstructure:"region" validate:"required"`
	AccessKeyID string `mapstructure:"access_key_id" validate:"required"`
	SecretKey   string `mapstructure:"secret_access_key" validate:"required"`
	S3Bucket    string `mapstructure:"s3_bucket" validate:"required"`
	S3Region    string `mapstructure:"s3_region"`
}

type KafkaConfig struct {
	Brokers        []string      `mapstructure:"brokers" validate:"required,min=1"`
	GroupID        string        `mapstructure:"group_id" validate:"required"`
	AutoOffset     string        `mapstructure:"auto_offset"`
	SessionTimeout time.Duration `mapstructure:"session_timeout"`
	RetryMax       int           `mapstructure:"retry_max"`
}

type LogConfig struct {
	Level  string `mapstructure:"level" validate:"required,oneof=debug info warn error"`
	Format string `mapstructure:"format" validate:"oneof=pretty json"`
	Output string `mapstructure:"output" validate:"oneof=stdout stderr file"`
	File   string `mapstructure:"file"`
}

type ServiceAddr struct {
	Name     string `mapstructure:"name"`
	Host     string `mapstructure:"host" validate:"required"`
	GRPCPort string `mapstructure:"grpc_port" validate:"required"`
	HTTPPort string `mapstructure:"http_port"`
}

// Load config data from file
func LoadConfig(configPath ...string) (*Config, error) {
	// Viper configuration
	viper.AddConfigPath(".")

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_", "grpc_port", "GRPCPort"))

	viper.AutomaticEnv()
	viper.SetEnvPrefix("MASTERS")

	setDefaults()
	bindEnvVars()

	// try to read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		log.Println("Config file not found. Using environment variables and defaults.")
	} else {
		log.Printf("Using config file: %s", viper.ConfigFileUsed())
	}

	bindEnvVars()

	// parse configuration data
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// validate conf data
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	log.Println("Configuration loaded and validated successfully")
	return &cfg, nil
}

// set defaults data for config
func setDefaults() {
	// App defaults
	viper.SetDefault("app.name", "remaster")
	viper.SetDefault("app.environment", "development")
	viper.SetDefault("app.version", "1.0.0")

	// HTTP defaults
	viper.SetDefault("http.port", "8080")
	viper.SetDefault("http.host", "0.0.0.0")
	viper.SetDefault("http.read_timeout", "10s")
	viper.SetDefault("http.idle_timeout", "5s")
	viper.SetDefault("http.write_timeout", "10s")
	viper.SetDefault("http.shutdown_timeout", "5s")

	// gRPC defaults
	viper.SetDefault("grpc.port", "9090")
	viper.SetDefault("grpc.host", "0.0.0.0")
	viper.SetDefault("grpc.max_receive_size", 4*1024*1024) // 4MB
	viper.SetDefault("grpc.max_send_size", 4*1024*1024)    // 4MB
	viper.SetDefault("grpc.connection_timeout", "10s")
	viper.SetDefault("grpc.enable_reflection", true)
	viper.SetDefault("grpc.enable_health_check", true)

	// MongoDB defaults
	viper.SetDefault("mongo.uri", "mongodb://localhost:27017")
	viper.SetDefault("mongo.database", "masters_platform")
	viper.SetDefault("mongo.max_pool_size", 100)
	viper.SetDefault("mongo.min_pool_size", 5)
	viper.SetDefault("mongo.connect_timeout", "10s")
	viper.SetDefault("mongo.server_selection_timeout", "5s")

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 20)
	viper.SetDefault("redis.min_idle_conns", 5)
	viper.SetDefault("redis.dial_timeout", "5s")
	viper.SetDefault("redis.read_timeout", "3s")
	viper.SetDefault("redis.write_timeout", "3s")

	// JWT defaults
	viper.SetDefault("jwt.secret_key", "change-this-in-production-min-32-characters")
	viper.SetDefault("jwt.access_token_ttl", "15m")
	viper.SetDefault("jwt.refresh_token_ttl", "24h")
	viper.SetDefault("jwt.issuer", "remaster")
	viper.SetDefault("jwt.audience", "remaster-users")

	// OAuth defaults
	viper.SetDefault("oauth.google_redirect_url", "http://localhost:8080/auth/google/callback")

	// AWS defaults
	viper.SetDefault("aws.region", "us-east-1")
	viper.SetDefault("aws.s3_region", "us-east-1")

	// Kafka defaults
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.group_id", "remaster")
	viper.SetDefault("kafka.auto_offset", "latest")
	viper.SetDefault("kafka.session_timeout", "10s")
	viper.SetDefault("kafka.retry_max", 3)

	// Log defaults
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "pretty")
	viper.SetDefault("log.output", "stdout")
}

// bind Env variables to config fields
func bindEnvVars() {
	envBindings := map[string]string{
		// App
		"app.name":        "APP_NAME",
		"app.environment": "APP_ENV",
		"app.version":     "APP_VERSION",

		// HTTP
		"http.port": "HTTP_PORT",
		"http.host": "HTTP_HOST",

		// gRPC
		"grpc.port": "GRPC_PORT",
		"grpc.host": "GRPC_HOST",

		// MongoDB
		"mongo.uri":      "MONGO_URI",
		"mongo.database": "MONGO_DATABASE",

		// Redis
		"redis.host":     "REDIS_HOST",
		"redis.port":     "REDIS_PORT",
		"redis.password": "REDIS_PASSWORD",
		"redis.db":       "REDIS_DB",

		// JWT
		"jwt.secret_key":        "JWT_SECRET_KEY",
		"jwt.access_token_ttl":  "JWT_ACCESS_TOKEN_TTL",
		"jwt.refresh_token_ttl": "JWT_REFRESH_TOKEN_TTL",

		// OAuth
		"oauth.google_client_id":     "GOOGLE_CLIENT_ID",
		"oauth.google_client_secret": "GOOGLE_CLIENT_SECRET",
		"oauth.google_redirect_url":  "GOOGLE_REDIRECT_URL",

		// AWS
		"aws.region":            "AWS_REGION",
		"aws.access_key_id":     "AWS_ACCESS_KEY_ID",
		"aws.secret_access_key": "AWS_SECRET_ACCESS_KEY",
		"aws.s3_bucket":         "AWS_S3_BUCKET",

		// Kafka
		"kafka.brokers":  "KAFKA_BROKERS",
		"kafka.group_id": "KAFKA_GROUP_ID",

		// Log
		"log.level":  "LOG_LEVEL",
		"log.format": "LOG_FORMAT",
		"log.output": "LOG_OUTPUT",
		"log.file":   "LOG_FILE",
	}

	for key, env := range envBindings {
		viper.BindEnv(key, env)
	}
}

// validate data for config
func validateConfig(cfg *Config) error {
	// check for prod JWT settings
	if cfg.App.Environment == "production" {
		if cfg.JWT.SecretKey == "change-this-in-production-min-32-characters" {
			return fmt.Errorf("JWT secret key must be changed in production")
		}
		if len(cfg.JWT.SecretKey) < 32 {
			return fmt.Errorf("JWT secret key must be at least 32 characters")
		}
	}

	// required MONGO fields
	if cfg.Mongo.URI == "" {
		return fmt.Errorf("MongoDB URI is required")
	}
	if cfg.Mongo.Database == "" {
		return fmt.Errorf("MongoDB database name is required")
	}

	// validate HTTP and gRPC ports
	if cfg.HTTP.Port == cfg.GRPC.Port {
		return fmt.Errorf("HTTP and gRPC ports must be different")
	}

	return nil
}

func (c *Config) GetServiceGRPCAddr(name string) (string, error) {
	svc, ok := c.Services[name]
	if !ok {
		return "", fmt.Errorf("service %s not found in config", name)
	}
	return fmt.Sprintf("%s:%s", svc.Host, svc.GRPCPort), nil
}
