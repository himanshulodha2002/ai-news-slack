package app

import (
	"context"
	"fmt"

	"github.com/himanshulodha/ai-news-slack/internal/config"
	"github.com/himanshulodha/ai-news-slack/internal/model"
	"github.com/himanshulodha/ai-news-slack/internal/slack"
	"github.com/himanshulodha/ai-news-slack/internal/source"
	"github.com/himanshulodha/ai-news-slack/internal/state"
)

func Run(ctx context.Context, cfg config.Config) error {
	store, err := state.Load(cfg.StateFile)
	if err != nil {
		return err
	}

	sourceService := source.NewService()
	items, err := sourceService.FetchAINewsItems(ctx, cfg.SourceURL)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("No AI News items found.")
		return nil
	}

	fetchedIDs := collectIDs(items)
	client := slack.NewClient(cfg.SlackBotToken, cfg.SlackChannel)

	if len(store.SeenIDs) == 0 {
		if err := client.PostItems(ctx, items[:1]); err != nil {
			return err
		}
		store.SeenIDs = state.RememberIDs(nil, fetchedIDs)
		if err := state.Save(cfg.StateFile, store); err != nil {
			return err
		}
		fmt.Println("Posted the latest AI News item and marked older items as seen.")
		return nil
	}

	candidates := selectNewItems(items, store.SeenIDs, cfg.MaxPosts)
	if len(candidates) == 0 {
		store.SeenIDs = state.RememberIDs(store.SeenIDs, fetchedIDs)
		if err := state.Save(cfg.StateFile, store); err != nil {
			return err
		}
		fmt.Println("No new AI News items to post.")
		return nil
	}

	if err := client.PostItems(ctx, candidates); err != nil {
		return err
	}

	store.SeenIDs = state.RememberIDs(store.SeenIDs, fetchedIDs)
	if err := state.Save(cfg.StateFile, store); err != nil {
		return err
	}

	fmt.Printf("Posted %d AI News item(s) to Slack.\n", len(candidates))
	return nil
}

func collectIDs(items []model.NewsItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func selectNewItems(items []model.NewsItem, seenIDs []string, maxPosts int) []model.NewsItem {
	newestSeenIndex := -1
	for index, item := range items {
		if contains(seenIDs, item.ID) {
			newestSeenIndex = index
			break
		}
	}

	var candidates []model.NewsItem
	if newestSeenIndex >= 0 {
		candidates = items[:newestSeenIndex]
	} else {
		for _, item := range items {
			if !contains(seenIDs, item.ID) {
				candidates = append(candidates, item)
			}
		}
	}

	if len(candidates) > maxPosts {
		candidates = candidates[:maxPosts]
	}

	reverseItems(candidates)
	return candidates
}

func reverseItems(items []model.NewsItem) {
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
