package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultSourceURL = "https://www.latent.space/s/ainews"
	DefaultStateFile = "./data/state.json"
	DefaultMaxPosts  = 1
)

type Config struct {
	SlackBotToken string
	SlackChannel  string
	SourceURL     string
	StateFile     string
	MaxPosts      int
}

func Load() (Config, error) {
	cfg := Config{
		SlackBotToken: os.Getenv("SLACK_BOT_TOKEN"),
		SlackChannel:  os.Getenv("SLACK_CHANNEL_ID"),
		SourceURL:     envOrDefault("LATENT_SPACE_AINEWS_URL", DefaultSourceURL),
		StateFile:     envOrDefault("STATE_FILE", DefaultStateFile),
		MaxPosts:      DefaultMaxPosts,
	}

	if cfg.SlackBotToken == "" {
		return Config{}, errors.New("SLACK_BOT_TOKEN is required")
	}
	if cfg.SlackChannel == "" {
		return Config{}, errors.New("SLACK_CHANNEL_ID is required")
	}

	if raw := os.Getenv("MAX_POSTS_PER_RUN"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err == nil && value > 0 {
			cfg.MaxPosts = value
		}
	}

	return cfg, nil
}

func LoadDotEnv(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}

	return nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
