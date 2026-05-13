package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

const defaultJWTExpiry = 24 * time.Hour

type Config struct {
	RESTPort      string
	GRPCPort      string
	LogLevel      string
	JWTSecret     string
	JWTExpiry     time.Duration
	MySQL         MySQLConfig
	Redis         RedisConfig
	ExternalAPI   string
}

type MySQLConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DB       string
}

func (m MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=UTC&multiStatements=true",
		m.User, m.Password, m.Host, m.Port, m.DB)
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

func (r RedisConfig) Addr() string { return r.Host + ":" + r.Port }

func Load() (*Config, error) {
	_ = godotenv.Load() // best-effort

	jwtExpiry, err := time.ParseDuration(getenv("JWT_EXPIRY", defaultJWTExpiry.String()))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRY: %w", err)
	}
	redisDB, _ := strconv.Atoi(getenv("REDIS_DB", "0"))

	cfg := &Config{
		RESTPort:    getenv("REST_PORT", "8080"),
		GRPCPort:    getenv("GRPC_PORT", "9090"),
		LogLevel:    getenv("LOG_LEVEL", "info"),
		JWTSecret:   getenv("JWT_SECRET", "super-secret-change-me"),
		JWTExpiry:   jwtExpiry,
		ExternalAPI: getenv("EXTERNAL_USER_API", "https://jsonplaceholder.typicode.com"),
		MySQL: MySQLConfig{
			Host:     getenv("MYSQL_HOST", "127.0.0.1"),
			Port:     getenv("MYSQL_PORT", "3306"),
			User:     getenv("MYSQL_USER", "taskuser"),
			Password: getenv("MYSQL_PASSWORD", "taskpass"),
			DB:       getenv("MYSQL_DB", "taskmanager"),
		},
		Redis: RedisConfig{
			Host:     getenv("REDIS_HOST", "127.0.0.1"),
			Port:     getenv("REDIS_PORT", "6379"),
			Password: getenv("REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
