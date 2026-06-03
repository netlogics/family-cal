package api

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ServerPort int    `yaml:"server_port"`
	DBPath     string `yaml:"db_path"`
	SMTPHost   string `yaml:"smtp_host"`
	SMTPPort   int    `yaml:"smtp_port"`
	SMTPUser   string `yaml:"smtp_user"`
	SMTPPass   string `yaml:"smtp_pass"`
	SMTPFrom   string `yaml:"smtp_from"`
	JWTSecret  string `yaml:"jwt_secret"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		ServerPort: 8080,
		DBPath:     "./family-cal.db",
		SMTPPort:   587,
		JWTSecret:  "change-me",
	}

	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}

	// env var overrides
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.ServerPort = p
		}
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("SMTP_HOST"); v != "" {
		cfg.SMTPHost = v
	}
	if v := os.Getenv("SMTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.SMTPPort = p
		}
	}
	if v := os.Getenv("SMTP_USER"); v != "" {
		cfg.SMTPUser = v
	}
	if v := os.Getenv("SMTP_PASS"); v != "" {
		cfg.SMTPPass = v
	}
	if v := os.Getenv("SMTP_FROM"); v != "" {
		cfg.SMTPFrom = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}

	return cfg, nil
}
