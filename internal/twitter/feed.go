package twitter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// LogEntry represents a single deletion log entry
type LogEntry struct {
	Timestamp string
	TweetID   string
	Preview   string
	Status    string // DELETED, SKIPPED_DATE, ERROR
	Error     string
}

func (c *Client) CleanFeed(stats *Stats) error {
	return c.CleanFeedWithLogs(stats, nil)
}

// CleanFeedWithLogs runs cleaning and sends log entries to the provided channel
func (c *Client) CleanFeedWithLogs(stats *Stats, logChan chan<- LogEntry) error {
	var cursor string
	page := 0

	// Parse date filter if set
	var beforeDate time.Time
	if c.cfg.DeleteBeforeDate != "" {
		if d, err := time.Parse("2006-01-02", c.cfg.DeleteBeforeDate); err == nil {
			beforeDate = d
		}
	}

	for {
		if c.cfg.MaxPages > 0 && page >= c.cfg.MaxPages {
			break
		}
		page++

		entries, nextCursor, err := c.fetchUserTweets(cursor)
		if err != nil {
			return fmt.Errorf("page %d: %w", page, err)
		}

		if len(entries) == 0 {
			break
		}

		stats.Scanned += len(entries)

		for _, e := range entries {
			tweetID, text, tweetTime, isRetweet := c.extractTweetInfo(e)
			if tweetID == "" || !isRetweet {
				continue
			}
			stats.Retweets++

			// Check date filter
			if !beforeDate.IsZero() && !tweetTime.IsZero() && tweetTime.After(beforeDate) {
				continue
			}

			if !c.containsKeyword(text) {
				continue
			}
			stats.Matched++

			preview := truncate(text, 30)
			now := time.Now().Format("15:04:05")

			entry := LogEntry{
				Timestamp: now,
				TweetID:   tweetID,
				Preview:   preview,
			}

			if err := c.deleteRetweet(tweetID); err != nil {
				stats.Errors++
				entry.Status = "ERROR"
				entry.Error = truncate(err.Error(), 40)
			} else {
				stats.Deleted++
				entry.Status = "DELETED"
			}

			if logChan != nil {
				logChan <- entry
			}

			time.Sleep(time.Duration(c.cfg.DeleteDelaySec) * time.Second)
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return nil
}

func (c *Client) fetchUserTweets(cursor string) ([]gjson.Result, string, error) {
	variables := map[string]any{
		"userId":                                 c.cfg.UserID,
		"count":                                  20,
		"includePromotedContent":                 false,
		"withQuickPromoteEligibilityQueryFields": true,
		"withVoice":                              true,
		"withV2Timeline":                         true,
	}
	if cursor != "" {
		variables["cursor"] = cursor
	}

	features := map[string]any{
		"profile_label_improvements_pcf_label_in_post_enabled":                    true,
		"rweb_tipjar_consumption_enabled":                                         true,
		"responsive_web_graphql_exclude_directive_enabled":                        true,
		"verified_phone_label_enabled":                                            false,
		"creator_subscriptions_tweet_preview_api_enabled":                         true,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"subscriptions_feature_can_gift_premium":                                  false,
		"tweetypie_unmention_optimization_enabled":                                true,
		"responsive_web_edit_tweet_api_enabled":                                   true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
		"view_counts_everywhere_api_enabled":                                      true,
		"longform_notetweets_consumption_enabled":                                 true,
		"responsive_web_twitter_article_tweet_consumption_enabled":                false,
		"tweet_awards_web_tipping_enabled":                                        false,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"responsive_web_media_download_video_enabled":                             false,
		"responsive_web_enhance_cards_enabled":                                    false,
	}

	varsJSON, _ := json.Marshal(variables)
	featJSON, _ := json.Marshal(features)

	params := url.Values{}
	params.Set("variables", string(varsJSON))
	params.Set("features", string(featJSON))

	apiURL := fmt.Sprintf("https://x.com/i/api/graphql/%s/UserTweets?%s", c.cfg.QueryIDUserTweets, params.Encode())

	body, err := c.doGet(apiURL)
	if err != nil {
		return nil, "", err
	}

	data := gjson.Parse(body)

	// Try both possible paths
	instructions := data.Get("data.user.result.timeline_v2.timeline.instructions").Array()
	if len(instructions) == 0 {
		instructions = data.Get("data.user.result.timeline.timeline.instructions").Array()
	}

	var entries []gjson.Result
	var nextCursor string

	for _, inst := range instructions {
		typeName := inst.Get("type").String()
		if typeName == "TimelineAddEntries" {
			entries = inst.Get("entries").Array()
		} else if typeName == "TimelineReplaceEntry" {
			entryId := inst.Get("entryId").String()
			if entryId == "cursor-bottom" || strings.HasPrefix(entryId, "cursor-bottom-") {
				nextCursor = inst.Get("entry.content.value").String()
				if nextCursor == "" {
					nextCursor = inst.Get("entry.content.cursor.value").String()
				}
			}
		}
	}

	// fallback: look for cursor in entries directly
	if nextCursor == "" {
		for _, e := range entries {
			entryId := e.Get("entryId").String()
			if strings.HasPrefix(entryId, "cursor-bottom-") {
				nextCursor = e.Get("content.value").String()
				if nextCursor == "" {
					nextCursor = e.Get("content.cursor.value").String()
				}
				break
			}
		}
	}

	return entries, nextCursor, nil
}

func (c *Client) extractTweetInfo(entry gjson.Result) (tweetID, text string, tweetTime time.Time, isRetweet bool) {
	itemType := entry.Get("content.entryType").String()
	if itemType != "TimelineTimelineItem" && itemType != "TimelineTimelineModule" {
		return "", "", time.Time{}, false
	}

	// Handle TimelineTimelineModule (conversation)
	if itemType == "TimelineTimelineModule" {
		items := entry.Get("content.items").Array()
		if len(items) == 0 {
			return "", "", time.Time{}, false
		}
		// Check first item in conversation
		firstItem := items[0]
		item := firstItem.Get("item.itemContent")
		return c.parseTweetItem(item)
	}

	// Handle single TimelineTimelineItem
	item := entry.Get("content.itemContent")
	return c.parseTweetItem(item)
}

func (c *Client) parseTweetItem(item gjson.Result) (tweetID, text string, tweetTime time.Time, isRetweet bool) {
	if !item.Exists() {
		return "", "", time.Time{}, false
	}

	typeName := item.Get("itemType").String()
	if typeName != "TimelineTweet" {
		return "", "", time.Time{}, false
	}

	tweet := item.Get("tweet_results.result")
	if !tweet.Exists() {
		return "", "", time.Time{}, false
	}

	// Handle TweetWithVisibilityResults wrapper
	if tweet.Get("tweet").Exists() {
		tweet = tweet.Get("tweet")
	}

	legacy := tweet.Get("legacy")

	// Check if retweeted
	retweeted := legacy.Get("retweeted").Bool()
	if !retweeted {
		return "", "", time.Time{}, false
	}

	isRetweet = true

	// Get source tweet ID and text from retweeted status
	retweetedStatus := legacy.Get("retweeted_status_result.result")
	if retweetedStatus.Exists() {
		// Handle nested wrapper
		if retweetedStatus.Get("tweet").Exists() {
			retweetedStatus = retweetedStatus.Get("tweet")
		}
		tweetID = retweetedStatus.Get("rest_id").String()
		if tweetID == "" {
			tweetID = retweetedStatus.Get("legacy.id_str").String()
		}
		text = retweetedStatus.Get("legacy.full_text").String()

		// Parse tweet date
		createdAt := retweetedStatus.Get("legacy.created_at").String()
		if createdAt != "" {
			tweetTime, _ = time.Parse(time.RubyDate, createdAt)
		}
	}

	return tweetID, text, tweetTime, isRetweet
}

func (c *Client) containsKeyword(text string) bool {
	upper := strings.ToUpper(text)
	for _, kw := range c.cfg.Keywords {
		if strings.Contains(upper, strings.ToUpper(kw)) {
			return true
		}
	}
	return false
}

func (c *Client) deleteRetweet(sourceTweetID string) error {
	apiURL := fmt.Sprintf("https://x.com/i/api/graphql/%s/DeleteRetweet", c.cfg.QueryIDDeleteRetweet)

	bodyMap := map[string]any{
		"variables": map[string]any{
			"source_tweet_id": sourceTweetID,
			"dark_request":    false,
		},
		"queryId": c.cfg.QueryIDDeleteRetweet,
	}

	bodyJSON, err := json.Marshal(bodyMap)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}

	c.applyAuth(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", fmt.Sprintf("https://x.com/home"))

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		resp, err := c.http.Do(req)
		if err != nil {
			if i == maxRetries-1 {
				return err
			}
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		if resp.StatusCode == 429 {
			waitSec := c.calcRateLimitWait(resp)
			log.Printf("Delete rate limited, waiting %d seconds...", waitSec)
			time.Sleep(time.Duration(waitSec) * time.Second)
			continue
		}

		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(b), 200))
	}

	return fmt.Errorf("max retries exceeded")
}
