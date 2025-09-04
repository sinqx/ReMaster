package config

type Config struct {
	HTTPPort   string `env:"HTTP_PORT" envDefault:"8080"`
	JWTSecret  string `env:"JWT_SECRET" envRequired:"true"`
	MongoDBURI string `env:"MONGODB_URI" envDefault:"mongodb://localhost:27017"`
	OAuth      struct {
		GoogleClientID     string `env:"GOOGLE_CLIENT_ID"`
		GoogleClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
	}
}
