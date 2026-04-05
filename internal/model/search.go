package model

import "time"

// SearchRequest represents a full-text search query.
type SearchRequest struct {
	Query       string            `json:"query" binding:"required"`
	EntityTypes []string          `json:"entity_types"` // project, task, user, file, company
	CompanyID   uint              `json:"company_id"`
	Page        int               `json:"page"`
	PageSize    int               `json:"page_size"`
	Filters     map[string]string `json:"filters"`
}

// SearchResult contains the response from a search operation.
type SearchResult struct {
	TotalHits int64             `json:"total_hits"`
	Hits      []SearchHit       `json:"hits"`
	Facets    map[string][]Facet `json:"facets,omitempty"`
	TookMs    int64             `json:"took_ms"`
}

// SearchHit represents a single search result.
type SearchHit struct {
	ID          string            `json:"id"`
	EntityType  string            `json:"entity_type"`
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Highlights  map[string]string `json:"highlights,omitempty"`
	Score       float64           `json:"score"`
}

// Facet represents an aggregation bucket.
type Facet struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// IndexDocument represents a document to be indexed in Elasticsearch.
type IndexDocument struct {
	ID         string            `json:"id"`
	EntityType string            `json:"entity_type" binding:"required"`
	CompanyID  uint              `json:"company_id" binding:"required"`
	Title      string            `json:"title" binding:"required"`
	Content    string            `json:"content"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// SuggestRequest represents an autocomplete query.
type SuggestRequest struct {
	Query       string   `json:"query" binding:"required"`
	EntityTypes []string `json:"entity_types"`
	CompanyID   uint     `json:"company_id"`
	Size        int      `json:"size"`
}

// SuggestResult contains autocomplete suggestions.
type SuggestResult struct {
	Suggestions []Suggestion `json:"suggestions"`
}

// Suggestion represents a single autocomplete suggestion.
type Suggestion struct {
	Text       string  `json:"text"`
	EntityType string  `json:"entity_type"`
	ID         string  `json:"id"`
	Score      float64 `json:"score"`
}
