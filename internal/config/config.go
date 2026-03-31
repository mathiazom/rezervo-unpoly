package config

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	FusionAuthURL         string // used for browser redirects
	FusionAuthInternalURL string // used for server-to-server calls (defaults to FusionAuthURL)
	ClientID              string
	AppURL                string
	APIURL                string
	SecretKey             []byte
	Port                  string
	Secure                bool
}

func Load() (*Config, error) {
	loadEnvFile(".env")

	secretHex := mustGetenv("SECRET_KEY")
	key, err := hex.DecodeString(secretHex)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("SECRET_KEY must be exactly 64 hex characters (32 bytes); generate with: openssl rand -hex 32")
	}

	appURL := mustGetenv("APP_URL")

	faURL := mustGetenv("FUSIONAUTH_URL")
	faInternalURL := getenv("FUSIONAUTH_INTERNAL_URL", faURL)

	return &Config{
		FusionAuthURL:         strings.TrimRight(faURL, "/"),
		FusionAuthInternalURL: strings.TrimRight(faInternalURL, "/"),
		ClientID:              mustGetenv("FUSIONAUTH_CLIENT_ID"),
		AppURL:                strings.TrimRight(appURL, "/"),
		APIURL:                strings.TrimRight(mustGetenv("REZERVO_API_URL"), "/"),
		SecretKey:             key,
		Port:                  getenv("PORT", "3000"),
		Secure:                strings.HasPrefix(appURL, "https://"),
	}, nil
}

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func mustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "error: missing required environment variable: %s\n", key)
		os.Exit(1)
	}
	return v
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
