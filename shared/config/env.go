package config

import (
	"fmt"
	"os"
	"strconv"
)

// GetEnv returns the value of an environment variable or a fallback value
func GetEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// GetEnvRequired returns the value of a required environment variable
// Panics if the variable is not set
func GetEnvRequired(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("Required environment variable %s is not set", key))
	}
	return value
}

// GetEnvInt returns an integer environment variable or a fallback value
func GetEnvInt(key string, fallback int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return fallback
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return fallback
	}
	return value
}

// GetEnvBool returns a boolean environment variable or a fallback value
func GetEnvBool(key string, fallback bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return fallback
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return fallback
	}
	return value
}

// ValidateRequiredEnvVars validates that all required environment variables are set
func ValidateRequiredEnvVars(requiredVars []string) error {
	missing := []string{}
	for _, varName := range requiredVars {
		if os.Getenv(varName) == "" {
			missing = append(missing, varName)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}
	return nil
}
