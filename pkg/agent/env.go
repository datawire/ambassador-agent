package agent

import "os"

func getEnvWithDefault(envVarKey string, defaultValue string) string {
	value := os.Getenv(envVarKey)
	if value == "" {
		value = defaultValue
	}
	return value
}
