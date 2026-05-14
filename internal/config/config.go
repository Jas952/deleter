package config

import (
	"deleter/internal/auth"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	UserID               string
	Cookies              map[string]any
	Headers              map[string]any
	QueryIDUserTweets    string
	QueryIDDeleteRetweet string
	Keywords             []string
	DeleteDelaySec       int
	MaxPages             int    // 0 = без лимита
	DeleteBeforeDate     string // Format: YYYY-MM-DD, empty = no date filter
}

// LoadFromEnv загружает конфигурацию только из переменных окружения
func LoadFromEnv() (*Config, error) {
	userID := os.Getenv("TWITTER_USER_ID")
	if userID == "" {
		return nil, fmt.Errorf("TWITTER_USER_ID is required")
	}

	cookies, err := parseJSON(os.Getenv("TWITTER_COOKIES"))
	if err != nil {
		return nil, fmt.Errorf("TWITTER_COOKIES: %w", err)
	}

	headers, err := parseJSON(os.Getenv("TWITTER_HEADERS"))
	if err != nil {
		return nil, fmt.Errorf("TWITTER_HEADERS: %w", err)
	}

	qidTweets := os.Getenv("QUERY_ID_USER_TWEETS")
	if qidTweets == "" {
		return nil, fmt.Errorf("QUERY_ID_USER_TWEETS is required")
	}

	qidDelete := os.Getenv("QUERY_ID_DELETE_RETWEET")
	if qidDelete == "" {
		return nil, fmt.Errorf("QUERY_ID_DELETE_RETWEET is required")
	}

	return buildConfig(userID, cookies, headers, qidTweets, qidDelete), nil
}

// LoadFromSession загружает конфигурацию из сохраненной сессии
func LoadFromSession(sm *auth.SessionManager) (*Config, error) {
	session, err := sm.Load()
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	if session == nil {
		return nil, nil
	}

	if !session.IsValid() {
		return nil, fmt.Errorf("session expired (%d days old)", 14-session.DaysUntilExpiry())
	}

	return buildConfig(
		session.UserID,
		session.Cookies,
		session.Headers,
		session.QueryIDUserTweets,
		session.QueryIDDeleteRetweet,
	), nil
}

// SaveToSession сохраняет конфигурацию в сессию
func (c *Config) SaveToSession(sm *auth.SessionManager) error {
	session := &auth.Session{
		UserID:               c.UserID,
		Cookies:              c.Cookies,
		Headers:              c.Headers,
		QueryIDUserTweets:    c.QueryIDUserTweets,
		QueryIDDeleteRetweet: c.QueryIDDeleteRetweet,
	}
	return sm.Save(session)
}

func buildConfig(userID string, cookies, headers map[string]any, qidTweets, qidDelete string) *Config {
	keywords := parseKeywords(os.Getenv("KEYWORDS"))

	// Default 10s delay to avoid Twitter rate limits (429 errors)
	delaySec := 10
	if v := os.Getenv("DELETE_DELAY_SEC"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			delaySec = d
		}
	}

	maxPages := 0
	if v := os.Getenv("MAX_PAGES"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			maxPages = p
		}
	}

	deleteBeforeDate := os.Getenv("DELETE_BEFORE_DATE")

	return &Config{
		UserID:               userID,
		Cookies:              cookies,
		Headers:              headers,
		QueryIDUserTweets:    qidTweets,
		QueryIDDeleteRetweet: qidDelete,
		Keywords:             keywords,
		DeleteDelaySec:       delaySec,
		MaxPages:             maxPages,
		DeleteBeforeDate:     deleteBeforeDate,
	}
}

// Load пытается загрузить из сессии, fallback на .env
func Load() (*Config, error) {
	sm := auth.NewSessionManager(".")

	// Пробуем сессию первой
	cfg, err := LoadFromSession(sm)
	if err == nil && cfg != nil {
		return cfg, nil
	}

	// Fallback на .env
	return LoadFromEnv()
}

func parseJSON(s string) (map[string]any, error) {
	if s == "" {
		return nil, fmt.Errorf("empty value")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func parseKeywords(s string) []string {
	if s == "" {
		return []string{"GIVEAWAY", "RUNE"}
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"GIVEAWAY", "RUNE"}
	}
	return out
}
