package twitter

import (
	"deleter/internal/config"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	cfg  *config.Config
	http *http.Client
}

type Stats struct {
	Scanned  int
	Retweets int
	Matched  int
	Deleted  int
	Errors   int
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) applyAuth(req *http.Request) {
	for k, v := range c.cfg.Headers {
		req.Header.Set(k, fmt.Sprint(v))
	}

	var parts []string
	for k, v := range c.cfg.Cookies {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	req.Header.Set("Cookie", strings.Join(parts, "; "))
}

func (c *Client) doGet(reqURL string) (string, error) {
	maxRetries := 5

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return "", err
		}
		c.applyAuth(req)

		resp, err := c.http.Do(req)
		if err != nil {
			return "", err
		}

		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", err
		}

		if resp.StatusCode == 200 {
			return string(b), nil
		}

		if resp.StatusCode == 429 {
			waitSec := c.calcRateLimitWait(resp)
			log.Printf("Rate limited (429), waiting %d seconds...", waitSec)
			time.Sleep(time.Duration(waitSec) * time.Second)
			continue
		}

		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(b), 300))
	}

	return "", fmt.Errorf("max retries (%d) exceeded", maxRetries)
}

func (c *Client) calcRateLimitWait(resp *http.Response) int {
	resetStr := resp.Header.Get("x-rate-limit-reset")
	if resetStr == "" {
		return 60
	}
	resetTime, err := strconv.ParseInt(resetStr, 10, 64)
	if err != nil {
		return 60
	}
	wait := int(resetTime - time.Now().Unix())
	if wait < 5 {
		return 5
	}
	if wait > 120 {
		return 120
	}
	return wait
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
