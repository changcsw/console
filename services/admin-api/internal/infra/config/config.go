package config

import "os"

type Config struct {
	AppName            string
	Environment        string
	HTTPAddress        string
	PostgresDSN        string
	SandboxPostgresDSN string
	ProductionDSN      string
}

func MustLoad() Config {
	return Config{
		AppName:            getEnv("APP_NAME", "admin-api"),
		Environment:        getEnv("APP_ENV", "develop"),
		HTTPAddress:        getEnv("HTTP_ADDRESS", ":18080"),
		PostgresDSN:        getEnv("POSTGRES_DSN", ""),
		SandboxPostgresDSN: getEnv("SANDBOX_POSTGRES_DSN", ""),
		ProductionDSN:      getEnv("PRODUCTION_POSTGRES_DSN", ""),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
