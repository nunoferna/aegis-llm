package config

import (
	"log"
	"os"
)

// Config holds all the environment-level configuration for our Gateway.
type Config struct {
	Port          string
	OpenAIKey     string
	OpenAIBaseURL string
}

// Load reads environment variables and returns a populated Config struct.
func Load() *Config {
	cfg := &Config{
		Port:          getEnvOrDefault("PORT", "8080"),
		OpenAIKey:     os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL: getEnvOrDefault("OPENAI_BASE_URL", "https://api.openai.com"),
	}

	// Fail fast if the core API key is missing. 
	// A senior engineer never lets an app boot if it's guaranteed to fail later.
	if cfg.OpenAIKey == "" {
		log.Fatal("FATAL: OPENAI_API_KEY environment variable is not set")
	}

	return cfg
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}