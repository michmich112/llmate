package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AccessKey      string
	Port           string
	DBDriver       string
	DBPath         string
	HealthInterval time.Duration
	LogLevel       string
	MaxBodySize    int64
}

// Load reads configuration from environment variables with defaults.
func Load() (*Config, error) {
	accessKey := os.Getenv("ACCESS_KEY")
	if accessKey == "" {
		return nil, fmt.Errorf("ACCESS_KEY environment variable is required")
	}

	cfg := &Config{
		AccessKey:      accessKey,
		Port:           getEnvOrDefault("PORT", "8080"),
		DBDriver:       getEnvOrDefault("DB_DRIVER", "sqlite"),
		DBPath:         getEnvOrDefault("DB_PATH", "./llmate.db"),
		HealthInterval: parseDurationOrDefault("HEALTH_INTERVAL", 30*time.Second),
		LogLevel:       getEnvOrDefault("LOG_LEVEL", "info"),
		MaxBodySize:    parseIntOrDefault("MAX_BODY_SIZE", 10*1024*1024),
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func parseDurationOrDefault(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

func parseIntOrDefault(key string, defaultVal int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return defaultVal
}
