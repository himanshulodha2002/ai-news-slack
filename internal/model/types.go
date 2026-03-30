package model

type State struct {
	SeenIDs []string `json:"seen_ids"`
}

type NewsItem struct {
	ID             string
	Title          string
	URL            string
	PublishedAt    string
	Summary        string
	ThreadMessages []string
}
