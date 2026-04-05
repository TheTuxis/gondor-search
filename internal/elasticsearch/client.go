package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"

	"github.com/TheTuxis/gondor-search/internal/model"
)

const (
	indexPrefix = "gondor"
)

// Client wraps the official Elasticsearch client.
type Client struct {
	es     *elasticsearch.Client
	logger *zap.Logger
}

// NewClient creates a new Elasticsearch client wrapper.
func NewClient(url string, logger *zap.Logger) (*Client, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{url},
	}
	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}
	return &Client{es: es, logger: logger}, nil
}

// Ping checks if the Elasticsearch cluster is reachable.
func (c *Client) Ping(ctx context.Context) error {
	res, err := c.es.Ping(c.es.Ping.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("elasticsearch ping failed: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("elasticsearch ping returned status: %s", res.Status())
	}
	return nil
}

// indexName returns the full index name for an entity type.
func indexName(entityType string) string {
	return fmt.Sprintf("%s_%s", indexPrefix, entityType)
}

// CreateIndex creates an Elasticsearch index with default mappings.
func (c *Client) CreateIndex(ctx context.Context, entityType string) error {
	mapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":          map[string]string{"type": "keyword"},
				"entity_type": map[string]string{"type": "keyword"},
				"company_id":  map[string]string{"type": "integer"},
				"title": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard",
					"fields": map[string]interface{}{
						"keyword": map[string]string{"type": "keyword"},
						"suggest": map[string]string{"type": "completion"},
					},
				},
				"content": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard",
				},
				"metadata":   map[string]string{"type": "object"},
				"created_at": map[string]string{"type": "date"},
				"updated_at": map[string]string{"type": "date"},
			},
		},
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
	}

	body, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("failed to marshal index mapping: %w", err)
	}

	res, err := c.es.Indices.Create(
		indexName(entityType),
		c.es.Indices.Create.WithContext(ctx),
		c.es.Indices.Create.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		respBody, _ := io.ReadAll(res.Body)
		return fmt.Errorf("create index error [%s]: %s", res.Status(), string(respBody))
	}

	c.logger.Info("created index", zap.String("index", indexName(entityType)))
	return nil
}

// Index indexes a document in Elasticsearch.
func (c *Client) Index(ctx context.Context, doc model.IndexDocument) error {
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now().UTC()
	}
	doc.UpdatedAt = time.Now().UTC()

	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	res, err := c.es.Index(
		indexName(doc.EntityType),
		bytes.NewReader(body),
		c.es.Index.WithContext(ctx),
		c.es.Index.WithDocumentID(doc.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		respBody, _ := io.ReadAll(res.Body)
		return fmt.Errorf("index error [%s]: %s", res.Status(), string(respBody))
	}

	return nil
}

// Delete removes a document from Elasticsearch.
func (c *Client) Delete(ctx context.Context, entityType, id string) error {
	res, err := c.es.Delete(
		indexName(entityType),
		id,
		c.es.Delete.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		respBody, _ := io.ReadAll(res.Body)
		return fmt.Errorf("delete error [%s]: %s", res.Status(), string(respBody))
	}

	return nil
}

// Search performs a full-text search across one or more entity types.
func (c *Client) Search(ctx context.Context, req model.SearchRequest) (*model.SearchResult, error) {
	// Build query
	must := []map[string]interface{}{
		{
			"multi_match": map[string]interface{}{
				"query":  req.Query,
				"fields": []string{"title^3", "content"},
				"type":   "best_fields",
			},
		},
	}

	filter := []map[string]interface{}{}
	if req.CompanyID > 0 {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{"company_id": req.CompanyID},
		})
	}
	for k, v := range req.Filters {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{k: v},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must":   must,
				"filter": filter,
			},
		},
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"title":   map[string]interface{}{},
				"content": map[string]interface{}{},
			},
		},
		"aggs": map[string]interface{}{
			"entity_types": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "entity_type",
				},
			},
		},
	}

	// Pagination
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query["from"] = (page - 1) * pageSize
	query["size"] = pageSize

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search query: %w", err)
	}

	// Determine target indices
	indices := make([]string, 0)
	if len(req.EntityTypes) > 0 {
		for _, et := range req.EntityTypes {
			indices = append(indices, indexName(et))
		}
	} else {
		indices = append(indices, indexName("*"))
	}

	res, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(indices...),
		c.es.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		respBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("search error [%s]: %s", res.Status(), string(respBody))
	}

	// Parse response
	var esResp esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	result := &model.SearchResult{
		TotalHits: esResp.Hits.Total.Value,
		TookMs:    int64(esResp.Took),
		Hits:      make([]model.SearchHit, 0, len(esResp.Hits.Hits)),
	}

	for _, hit := range esResp.Hits.Hits {
		var doc model.IndexDocument
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			c.logger.Warn("failed to unmarshal hit", zap.Error(err))
			continue
		}

		highlights := make(map[string]string)
		for field, fragments := range hit.Highlight {
			if len(fragments) > 0 {
				highlights[field] = fragments[0]
			}
		}

		result.Hits = append(result.Hits, model.SearchHit{
			ID:          doc.ID,
			EntityType:  doc.EntityType,
			Title:       doc.Title,
			Description: doc.Content,
			Highlights:  highlights,
			Score:       hit.Score,
		})
	}

	// Parse facets
	if agg, ok := esResp.Aggregations["entity_types"]; ok {
		facets := make([]model.Facet, 0, len(agg.Buckets))
		for _, bucket := range agg.Buckets {
			facets = append(facets, model.Facet{
				Key:   bucket.Key,
				Count: bucket.DocCount,
			})
		}
		result.Facets = map[string][]model.Facet{
			"entity_types": facets,
		}
	}

	return result, nil
}

// Suggest returns autocomplete suggestions.
func (c *Client) Suggest(ctx context.Context, req model.SuggestRequest) (*model.SuggestResult, error) {
	size := req.Size
	if size < 1 || size > 20 {
		size = 5
	}

	query := map[string]interface{}{
		"suggest": map[string]interface{}{
			"title-suggest": map[string]interface{}{
				"prefix": req.Query,
				"completion": map[string]interface{}{
					"field": "title.suggest",
					"size":  size,
				},
			},
		},
		"size": 0,
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal suggest query: %w", err)
	}

	indices := make([]string, 0)
	if len(req.EntityTypes) > 0 {
		for _, et := range req.EntityTypes {
			indices = append(indices, indexName(et))
		}
	} else {
		indices = append(indices, indexName("*"))
	}

	res, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(indices...),
		c.es.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, fmt.Errorf("suggest request failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		respBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("suggest error [%s]: %s", res.Status(), string(respBody))
	}

	var esResp esSuggestResponse
	if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("failed to parse suggest response: %w", err)
	}

	result := &model.SuggestResult{
		Suggestions: make([]model.Suggestion, 0),
	}

	if suggestions, ok := esResp.Suggest["title-suggest"]; ok {
		for _, suggest := range suggestions {
			for _, option := range suggest.Options {
				var doc model.IndexDocument
				if err := json.Unmarshal(option.Source, &doc); err != nil {
					continue
				}
				result.Suggestions = append(result.Suggestions, model.Suggestion{
					Text:       doc.Title,
					EntityType: doc.EntityType,
					ID:         doc.ID,
					Score:      option.Score,
				})
			}
		}
	}

	return result, nil
}

// Elasticsearch response types

type esSearchResponse struct {
	Took         int                       `json:"took"`
	Hits         esHits                    `json:"hits"`
	Aggregations map[string]esAggregation  `json:"aggregations"`
}

type esHits struct {
	Total esTotal  `json:"total"`
	Hits  []esHit  `json:"hits"`
}

type esTotal struct {
	Value int64 `json:"value"`
}

type esHit struct {
	Source    json.RawMessage      `json:"_source"`
	Score     float64              `json:"_score"`
	Highlight map[string][]string  `json:"highlight"`
}

type esAggregation struct {
	Buckets []esBucket `json:"buckets"`
}

type esBucket struct {
	Key      string `json:"key"`
	DocCount int64  `json:"doc_count"`
}

type esSuggestResponse struct {
	Suggest map[string][]esSuggest `json:"suggest"`
}

type esSuggest struct {
	Options []esSuggestOption `json:"options"`
}

type esSuggestOption struct {
	Source json.RawMessage `json:"_source"`
	Score  float64         `json:"_score"`
}
