package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/himanshulodha/ai-news-slack/internal/model"
)

type Client struct {
	token   string
	channel string
}

type postMessageRequest struct {
	Channel     string `json:"channel"`
	Text        string `json:"text"`
	ThreadTS    string `json:"thread_ts,omitempty"`
	UnfurlLinks bool   `json:"unfurl_links"`
}

type postMessageResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	TS    string `json:"ts"`
}

func NewClient(token, channel string) *Client {
	return &Client{token: token, channel: channel}
}

func (c *Client) PostItems(ctx context.Context, items []model.NewsItem) error {
	for _, item := range items {
		rootResp, err := c.postMessage(ctx, postMessageRequest{
			Channel:     c.channel,
			Text:        buildRootMessage(item),
			UnfurlLinks: false,
		})
		if err != nil {
			return err
		}

		for _, threadMessage := range item.ThreadMessages {
			if _, err := c.postMessage(ctx, postMessageRequest{
				Channel:     c.channel,
				Text:        threadMessage,
				ThreadTS:    rootResp.TS,
				UnfurlLinks: false,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Client) postMessage(ctx context.Context, payload postMessageRequest) (postMessageResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return postMessageResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return postMessageResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return postMessageResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return postMessageResponse{}, err
	}

	var parsed postMessageResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return postMessageResponse{}, err
	}
	if !parsed.OK {
		return postMessageResponse{}, fmt.Errorf("slack API error: %s", parsed.Error)
	}

	return parsed, nil
}

func buildRootMessage(item model.NewsItem) string {
	lines := []string{"*" + item.Title + "*"}
	if item.Summary != "" {
		lines = append(lines, "_"+item.Summary+"_")
	}
	lines = append(lines, "<"+item.URL+"|Open full recap>")
	return strings.Join(lines, "\n")
}
