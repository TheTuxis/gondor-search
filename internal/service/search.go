package service

import (
	"context"
	"errors"

	"go.uber.org/zap"

	es "github.com/TheTuxis/gondor-search/internal/elasticsearch"
	"github.com/TheTuxis/gondor-search/internal/model"
)

var (
	ErrEmptyQuery    = errors.New("search query cannot be empty")
	ErrInvalidEntity = errors.New("invalid entity type")
)

var validEntityTypes = map[string]bool{
	"project": true,
	"task":    true,
	"user":    true,
	"file":    true,
	"company": true,
}

type SearchService struct {
	esClient *es.Client
	logger   *zap.Logger
}

func NewSearchService(esClient *es.Client, logger *zap.Logger) *SearchService {
	return &SearchService{esClient: esClient, logger: logger}
}

func (s *SearchService) Search(ctx context.Context, req model.SearchRequest) (*model.SearchResult, error) {
	if req.Query == "" {
		return nil, ErrEmptyQuery
	}

	for _, et := range req.EntityTypes {
		if !validEntityTypes[et] {
			return nil, ErrInvalidEntity
		}
	}

	result, err := s.esClient.Search(ctx, req)
	if err != nil {
		s.logger.Error("search failed", zap.Error(err), zap.String("query", req.Query))
		return nil, err
	}

	return result, nil
}

func (s *SearchService) Suggest(ctx context.Context, req model.SuggestRequest) (*model.SuggestResult, error) {
	if req.Query == "" {
		return nil, ErrEmptyQuery
	}

	result, err := s.esClient.Suggest(ctx, req)
	if err != nil {
		s.logger.Error("suggest failed", zap.Error(err), zap.String("query", req.Query))
		return nil, err
	}

	return result, nil
}

func (s *SearchService) IndexDocument(ctx context.Context, doc model.IndexDocument) error {
	if !validEntityTypes[doc.EntityType] {
		return ErrInvalidEntity
	}

	if err := s.esClient.Index(ctx, doc); err != nil {
		s.logger.Error("index failed",
			zap.Error(err),
			zap.String("entity_type", doc.EntityType),
			zap.String("id", doc.ID),
		)
		return err
	}

	s.logger.Info("document indexed",
		zap.String("entity_type", doc.EntityType),
		zap.String("id", doc.ID),
	)
	return nil
}

func (s *SearchService) DeleteDocument(ctx context.Context, entityType, id string) error {
	if !validEntityTypes[entityType] {
		return ErrInvalidEntity
	}

	if err := s.esClient.Delete(ctx, entityType, id); err != nil {
		s.logger.Error("delete failed",
			zap.Error(err),
			zap.String("entity_type", entityType),
			zap.String("id", id),
		)
		return err
	}

	s.logger.Info("document deleted",
		zap.String("entity_type", entityType),
		zap.String("id", id),
	)
	return nil
}

func (s *SearchService) Reindex(ctx context.Context, entityType string) error {
	if !validEntityTypes[entityType] {
		return ErrInvalidEntity
	}

	// Create (or recreate) the index — actual reindexing from source services
	// would be triggered via NATS events in a future iteration.
	if err := s.esClient.CreateIndex(ctx, entityType); err != nil {
		s.logger.Error("reindex failed", zap.Error(err), zap.String("entity_type", entityType))
		return err
	}

	s.logger.Info("reindex triggered", zap.String("entity_type", entityType))
	return nil
}
