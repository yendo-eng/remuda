package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yendo-eng/remuda/internal/env"
)

type Slack interface {
	GetThread(threadURL string) (string, error)
}

type httpSlack struct {
	client http.Client
	env    env.Provider
}

func NewHTTPSlack(client http.Client) Slack {
	return NewHTTPSlackWithEnv(client, env.Default())
}

func NewHTTPSlackWithEnv(client http.Client, provider env.Provider) Slack {
	return &httpSlack{client: client, env: env.OrDefault(provider)}
}

func (s *httpSlack) GetThread(threadURL string) (string, error) {
	return fetchSlackThread(threadURL, s.env)
}

type slackThreadInfo struct {
	ChannelID string
	ThreadTS  string
}

func fetchSlackThread(threadURL string, provider env.Provider) (string, error) {
	info, err := parseSlackThreadURL(threadURL)
	if err != nil {
		return "", err
	}

	provider = env.OrDefault(provider)
	token := strings.TrimSpace(provider.Getenv("SLACK_TOKEN"))
	if token == "" {
		return "", errors.New("SLACK_TOKEN environment variable is required when --slack-thread is used")
	}

	params := url.Values{}
	params.Set("channel", info.ChannelID)
	params.Set("ts", info.ThreadTS)
	params.Set("limit", "200")

	client := &http.Client{Timeout: 30 * time.Second}
	cursor := ""
	messages := make([]slackMessage, 0, 16)

	for {
		if cursor != "" {
			params.Set("cursor", cursor)
		} else {
			params.Del("cursor")
		}

		reqURL := &url.URL{Scheme: "https", Host: "slack.com", Path: "/api/conversations.replies", RawQuery: params.Encode()}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, reqURL.String(), nil)
		if err != nil {
			return "", fmt.Errorf("creating slack request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "remuda-vibe (https://github.com/yendo-eng/remuda)")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("calling Slack conversations.replies: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			status := resp.Status
			if resp.Body != nil {
				_ = resp.Body.Close()
			}
			return "", fmt.Errorf("slack API returned %s", status)
		}

		var payload slackRepliesResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			_ = resp.Body.Close()
			return "", fmt.Errorf("decoding Slack response: %w", err)
		}
		_ = resp.Body.Close()

		if !payload.OK {
			if payload.Error != "" {
				return "", fmt.Errorf("slack API error: %s", payload.Error)
			}
			return "", errors.New("slack API returned ok=false")
		}

		messages = append(messages, payload.Messages...)

		if payload.ResponseMetadata == nil || strings.TrimSpace(payload.ResponseMetadata.NextCursor) == "" {
			break
		}
		cursor = payload.ResponseMetadata.NextCursor
	}

	if len(messages) == 0 {
		return "(thread is empty)\n", nil
	}

	var sb strings.Builder
	for _, msg := range messages {
		line := formatSlackMessageLine(msg)
		sb.WriteString(line)
		if !strings.HasSuffix(line, "\n") {
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

type slackRepliesResponse struct {
	OK               bool           `json:"ok"`
	Error            string         `json:"error"`
	Messages         []slackMessage `json:"messages"`
	ResponseMetadata *slackMetadata `json:"response_metadata"`
}

type slackMetadata struct {
	NextCursor string `json:"next_cursor"`
}

type slackMessage struct {
	Text       string        `json:"text"`
	User       string        `json:"user"`
	Ts         string        `json:"ts"`
	Username   string        `json:"username"`
	BotProfile *slackBotInfo `json:"bot_profile"`
}

type slackBotInfo struct {
	Name string `json:"name"`
}

func formatSlackMessageLine(msg slackMessage) string {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		text = "(no text)"
	}

	name := strings.TrimSpace(msg.Username)
	if name == "" && msg.BotProfile != nil {
		name = strings.TrimSpace(msg.BotProfile.Name)
	}
	if name == "" {
		name = strings.TrimSpace(msg.User)
		if name == "" {
			name = "unknown"
		}
	}

	prettyTS := formatSlackTimestamp(msg.Ts)
	return fmt.Sprintf("[%s] %s: %s\n", prettyTS, name, text)
}

func formatSlackTimestamp(ts string) string {
	parts := strings.Split(ts, ".")
	if len(parts) == 0 || parts[0] == "" {
		return ts
	}
	sec, err := parseInt64(parts[0])
	if err != nil {
		return ts
	}
	var nsec int64
	if len(parts) > 1 {
		frac := parts[1]
		if len(frac) > 9 {
			frac = frac[:9]
		}
		for len(frac) < 9 {
			frac += "0"
		}
		if v, err := parseInt64(frac); err == nil {
			nsec = v
		}
	}
	return time.Unix(sec, nsec).UTC().Format(time.RFC3339)
}

func parseInt64(s string) (int64, error) {
	var n int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("non-digit in timestamp")
		}
		n = n*10 + int64(ch-'0')
	}
	return n, nil
}

// BuildSlackThreadContext fetches the provided Slack threads and concatenates
// them into a single text block suitable for saving to disk.
func BuildSlackThreadContext(
	slack Slack,
	urls []string,
) (string, error) {
	var sb strings.Builder
	for _, threadURL := range urls {
		content, err := slack.GetThread(threadURL)
		if err != nil {
			return "", err
		}
		sb.WriteString("---------- Slack Thread ")
		sb.WriteString(threadURL)
		sb.WriteString(" ----------\n")
		sb.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

func parseSlackThreadURL(raw string) (slackThreadInfo, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return slackThreadInfo{}, fmt.Errorf("invalid Slack thread URL: %w", err)
	}

	channel := ""
	threadTS := strings.TrimSpace(u.Query().Get("thread_ts"))

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) >= 2 && segments[0] == "archives" {
		channel = segments[1]
		if threadTS == "" && len(segments) >= 3 {
			threadTS = parsePermalinkTS(strings.TrimPrefix(segments[2], "p"))
		}
	}

	if channel == "" {
		if cid := strings.TrimSpace(u.Query().Get("cid")); cid != "" {
			channel = cid
		}
	}

	if (channel == "" || threadTS == "") && len(segments) >= 4 && segments[0] == "client" {
		if channel == "" {
			channel = segments[2]
		}
		for i := 3; i < len(segments); i++ {
			part := segments[i]
			if part == "thread" || part == "message" {
				if i+1 < len(segments) {
					combo := segments[i+1]
					if dash := strings.Index(combo, "-"); dash > 0 {
						channel = combo[:dash]
						threadTS = combo[dash+1:]
					}
				}
				break
			}
		}
	}

	if channel == "" || threadTS == "" {
		return slackThreadInfo{}, fmt.Errorf("unable to extract channel and thread timestamp from Slack URL: %s", raw)
	}

	return slackThreadInfo{ChannelID: channel, ThreadTS: threadTS}, nil
}

func parsePermalinkTS(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) <= 10 {
		return raw
	}
	return fmt.Sprintf("%s.%s", raw[:10], raw[10:])
}
