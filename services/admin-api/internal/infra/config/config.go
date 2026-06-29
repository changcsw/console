package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppName            string
	Environment        string
	HTTPAddress        string
	PostgresDSN        string
	SandboxPostgresDSN string
	ProductionDSN      string

	// Auth / JWT（D5，compact「枚举与默认」）
	JWTSecret     string
	JWTIssuer     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration
	BcryptCost    int

	// 密文（00 §6）：AES-GCM 飞书令牌密钥（hex/base64 由 crypto 层解析）
	AESKey string

	// 飞书
	FeishuMock        bool
	FeishuAppID       string
	FeishuAppSecret   string
	FeishuRedirectURI string
}

func MustLoad() Config {
	env := getEnv("APP_ENV", "develop")
	return Config{
		AppName:            getEnv("APP_NAME", "admin-api"),
		Environment:        env,
		HTTPAddress:        getEnv("HTTP_ADDRESS", ":18080"),
		PostgresDSN:        getEnv("POSTGRES_DSN", ""),
		SandboxPostgresDSN: getEnv("SANDBOX_POSTGRES_DSN", ""),
		ProductionDSN:      getEnv("PRODUCTION_POSTGRES_DSN", ""),

		JWTSecret:     getEnv("ADMIN_JWT_SECRET", ""),
		JWTIssuer:     getEnv("ADMIN_JWT_ISSUER", "admin-api"),
		JWTAccessTTL:  getDuration("ADMIN_JWT_ACCESS_TTL", 30*time.Minute),
		JWTRefreshTTL: getDuration("ADMIN_JWT_REFRESH_TTL", 336*time.Hour),
		BcryptCost:    getInt("ADMIN_BCRYPT_COST", 10),

		AESKey: getEnv("ADMIN_AES_KEY", ""),

		// mock 仅 develop 生效（compact 红线）
		FeishuMock:        env == "develop" && getBool("ADMIN_FEISHU_MOCK", false),
		FeishuAppID:       getEnv("ADMIN_FEISHU_APP_ID", ""),
		FeishuAppSecret:   getEnv("ADMIN_FEISHU_APP_SECRET", ""),
		FeishuRedirectURI: getEnv("ADMIN_FEISHU_REDIRECT_URI", ""),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
