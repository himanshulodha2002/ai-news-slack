package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/himanshulodha/ai-news-slack/internal/model"
)

const maxRememberedPosts = 1000

func Load(path string) (model.State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.State{}, nil
		}
		return model.State{}, err
	}

	var state model.State
	if err := json.Unmarshal(data, &state); err != nil {
		return model.State{}, err
	}

	return state, nil
}

func Save(path string, state model.State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func RememberIDs(existing, incoming []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	combined := make([]string, 0, len(existing)+len(incoming))

	for _, id := range append(existing, incoming...) {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		combined = append(combined, id)
	}

	if len(combined) > maxRememberedPosts {
		combined = combined[len(combined)-maxRememberedPosts:]
	}

	return combined
}
