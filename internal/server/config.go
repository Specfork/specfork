package server

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"
)

// Config is everything `specfork serve` reads from its environment (API-2).
type Config struct {
	Port              string
	DatabaseURL       string
	ClientID          string
	ClientSecret      string
	SessionSecret     []byte
	AppID             int64
	AppKey            *rsa.PrivateKey
	InstallationID    int64
	MinAccountAgeDays int

	// Where things live; the defaults are the real ones, tests point them at fakes.
	SiteURL      string // origin of the static site, the only CORS origin with credentials
	GitHubWeb    string
	GitHubAPI    string
	GitHubRaw    string
	RegistryRepo string // owner/repo that holds registry/names.json
	RegistryRef  string
}

// LoadConfig reads the environment. Errors name the variable, never its value.
func LoadConfig(getenv func(string) string) (*Config, error) {
	c := &Config{
		Port: getenv("PORT"), MinAccountAgeDays: 30,
		SiteURL: "https://specfork.dev", GitHubWeb: "https://github.com", GitHubAPI: "https://api.github.com",
		GitHubRaw: "https://raw.githubusercontent.com", RegistryRepo: "specfork/specfork", RegistryRef: "main",
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	need := func(name string) (string, error) {
		if v := getenv(name); v != "" {
			return v, nil
		}
		return "", fmt.Errorf("%s is not set", name)
	}
	num := func(name string) (int64, error) {
		v, err := need(name)
		if err != nil {
			return 0, err
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("%s must be a positive number", name)
		}
		return n, nil
	}
	var err error
	if c.DatabaseURL, err = need("DATABASE_URL"); err != nil {
		return nil, err
	}
	if c.ClientID, err = need("GITHUB_CLIENT_ID"); err != nil {
		return nil, err
	}
	if c.ClientSecret, err = need("GITHUB_CLIENT_SECRET"); err != nil {
		return nil, err
	}
	secret, err := need("SESSION_SECRET")
	if err != nil {
		return nil, err
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET must be at least 32 bytes")
	}
	c.SessionSecret = []byte(secret)
	if c.AppID, err = num("GITHUB_APP_ID"); err != nil {
		return nil, err
	}
	if c.InstallationID, err = num("GITHUB_APP_INSTALLATION_ID"); err != nil {
		return nil, err
	}
	pemKey, err := need("GITHUB_APP_PRIVATE_KEY")
	if err != nil {
		return nil, err
	}
	if c.AppKey, err = parseKey(pemKey); err != nil {
		return nil, fmt.Errorf("GITHUB_APP_PRIVATE_KEY is not a PEM RSA key")
	}
	if v := getenv("SCORE_MIN_ACCOUNT_AGE_DAYS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("SCORE_MIN_ACCOUNT_AGE_DAYS must be a number of days")
		}
		c.MinAccountAgeDays = n
	}
	return c, nil
}

// parseKey accepts PKCS#1 or PKCS#8, with real or escaped (\n) line breaks as env files have them.
func parseKey(s string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.ReplaceAll(s, `\n`, "\n")))
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not RSA")
	}
	return rk, nil
}
