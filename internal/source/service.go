package source

import (
	"bytes"
	"context"
	"encoding/xml"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/himanshulodha/ai-news-slack/internal/httpx"
	"github.com/himanshulodha/ai-news-slack/internal/model"
)

type Service struct{}

type rssFeed struct {
	Channel struct {
		Items []struct {
			Title   string `xml:"title"`
			Link    string `xml:"link"`
			PubDate string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) FetchAINewsItems(ctx context.Context, sourceURL string) ([]model.NewsItem, error) {
	feedItems, _ := s.fetchFeedItems(ctx)
	sectionItems, err := s.fetchSectionItems(ctx, sourceURL)
	if err != nil {
		return nil, err
	}

	byURL := make(map[string]model.NewsItem)
	for _, item := range append(feedItems, sectionItems...) {
		byURL[item.URL] = item
	}

	items := make([]model.NewsItem, 0, len(byURL))
	for _, item := range byURL {
		enriched, err := s.enrichArticle(ctx, item)
		if err != nil {
			items = append(items, item)
			continue
		}
		items = append(items, enriched)
	}

	sortItemsByPublishedAt(items)
	return items, nil
}

func (s *Service) fetchFeedItems(ctx context.Context) ([]model.NewsItem, error) {
	candidates := []string{
		"https://www.latent.space/feed",
		"https://www.latent.space/s/ainews/feed",
	}

	for _, candidate := range candidates {
		body, err := httpx.Fetch(ctx, candidate)
		if err != nil {
			continue
		}

		var feed rssFeed
		if err := xml.Unmarshal(body, &feed); err != nil {
			continue
		}

		var items []model.NewsItem
		for _, entry := range feed.Channel.Items {
			if !looksLikeAINews(entry.Link, entry.Title) {
				continue
			}

			normalizedURL, err := httpx.NormalizeURL(entry.Link)
			if err != nil {
				continue
			}

			items = append(items, model.NewsItem{
				ID:          buildItemID(normalizedURL, entry.PubDate),
				Title:       normalizeWhitespace(entry.Title),
				URL:         normalizedURL,
				PublishedAt: normalizeWhitespace(entry.PubDate),
			})
		}

		if len(items) > 0 {
			return items, nil
		}
	}

	return nil, nil
}

func (s *Service) fetchSectionItems(ctx context.Context, sourceURL string) ([]model.NewsItem, error) {
	body, err := httpx.Fetch(ctx, sourceURL)
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	itemsByID := map[string]model.NewsItem{}
	walk(doc, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "a" {
			return
		}

		href := getAttr(node, "href")
		title := normalizeWhitespace(textContent(node))
		if href == "" || title == "" || !looksLikeAINews(href, title) {
			return
		}

		resolvedURL, err := httpx.ResolveURL(sourceURL, href)
		if err != nil || !strings.Contains(resolvedURL, "/p/") {
			return
		}

		item := model.NewsItem{
			ID:    buildItemID(resolvedURL, ""),
			Title: title,
			URL:   resolvedURL,
		}
		itemsByID[item.ID] = item
	})

	items := make([]model.NewsItem, 0, len(itemsByID))
	for _, item := range itemsByID {
		items = append(items, item)
	}

	return items, nil
}

func (s *Service) enrichArticle(ctx context.Context, item model.NewsItem) (model.NewsItem, error) {
	body, err := httpx.Fetch(ctx, item.URL)
	if err != nil {
		return item, err
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return item, err
	}

	titleNode := findFirstTag(doc, "h1")
	if titleNode == nil {
		return item, nil
	}

	article := findAncestorTag(titleNode, "article")
	if article == nil {
		article = doc
	}

	title := normalizeWhitespace(textContent(titleNode))
	summary := normalizeWhitespace(textContent(findFirstTag(article, "h3")))
	contentNodes := collectContentNodes(article)
	recapStart := findContentIndex(contentNodes, "AI Twitter Recap")
	redditStart := findContentIndex(contentNodes, "AI Reddit Recap")

	var threadBlocks []string
	if intro := buildIntroMessages(contentNodes, recapStart, item.URL); len(intro) > 0 {
		threadBlocks = append(threadBlocks, splitLongMessage(strings.Join(append([]string{"*Brief*"}, intro...), "\n"))...)
	}
	if recapStart >= 0 {
		end := len(contentNodes)
		if redditStart > recapStart {
			end = redditStart
		}
		threadBlocks = append(threadBlocks, buildRecapMessages(contentNodes[recapStart+1:end], item.URL)...)
	}

	if title != "" {
		item.Title = title
	}
	if summary != "" {
		item.Summary = summary
	}
	item.ThreadMessages = threadBlocks

	return item, nil
}

func reverseItems(items []model.NewsItem) {
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
}

func sortItemsByPublishedAt(items []model.NewsItem) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if parseTime(items[j].PublishedAt).After(parseTime(items[i].PublishedAt)) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func parseTime(raw string) time.Time {
	formats := []string{time.RFC1123Z, time.RFC1123, time.RFC3339, "Mon, 2 Jan 2006 15:04:05 MST"}
	for _, format := range formats {
		if parsed, err := time.Parse(format, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
