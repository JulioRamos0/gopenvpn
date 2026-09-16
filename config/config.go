package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port           int
	OpenVPNProfile string
	AWSCredentials string
}

func LoadConfig() (*Config, error) {
	portStr := os.Getenv("PORT")
	port := 8080
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT environment variable: %w", err)
		}
		port = p
	}

	if port < 1024 {
		return nil, fmt.Errorf("invalid PORT %d: cannot use privileged ports (< 1024) for security reasons", port)
	}

	profileB64 := os.Getenv("OPENVPN_PROFILE")
	if profileB64 == "" {
		return nil, fmt.Errorf("OPENVPN_PROFILE environment variable is not defined")
	}

	return &Config{
		Port:           port,
		OpenVPNProfile: profileB64,
		AWSCredentials: os.Getenv("AWS_CREDENTIALS"),
	}, nil
}
