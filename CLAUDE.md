# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview
Search microservice for Gondor platform. Go/Gin service providing full-text search, autocomplete, and document indexing via Elasticsearch.

## Commands
- `make build` -- compile to bin/server
- `make run` -- run locally (needs Elasticsearch + Redis)
- `make test` -- run all tests with race detector
- `make lint` -- golangci-lint
- `make docker` -- build Docker image

## Architecture
- `cmd/server/main.go` -- entry point, dependency injection, route registration
- `internal/config/` -- env-based configuration
- `internal/model/` -- plain structs (SearchRequest, SearchResult, SearchHit, IndexDocument)
- `internal/elasticsearch/` -- Elasticsearch client wrapper (go-elasticsearch/v8)
- `internal/service/` -- business logic
- `internal/handler/` -- HTTP handlers (Gin)
- `internal/middleware/` -- JWT auth (validate-only), logging
- `internal/pkg/jwt/` -- JWT validation (tokens issued by gondor-users-security)

## Key Decisions
- JWT tokens are validated only (issued by gondor-users-security service)
- Port 8007
- No PostgreSQL -- uses Elasticsearch as primary data store
- Elasticsearch index naming: gondor_{entity_type} (e.g., gondor_project, gondor_task)
- Multi-tenancy via company_id filter on all queries
- Valid entity types: project, task, user, file, company
- Documents indexed via REST (POST /v1/search/index) or NATS events (future)
- Autocomplete via Elasticsearch completion suggester on title.suggest field
- /health and /metrics skip JWT auth

## Routes
- POST /v1/search -- full-text search
- POST /v1/search/suggest -- autocomplete suggestions
- POST /v1/search/index -- index a document
- DELETE /v1/search/index/:entity_type/:id -- remove from index
- POST /v1/search/reindex/:entity_type -- trigger reindex
- GET /health -- health check (Elasticsearch + Redis)
- GET /metrics -- Prometheus metrics

## Environment Variables
- `PORT` (default: 8007)
- `ELASTICSEARCH_URL` (default: http://localhost:9200)
- `JWT_SECRET` (default: change-me-in-production)
- `REDIS_URL` (default: localhost:6379)
- `NATS_URL` (default: nats://localhost:4222)
- `LOG_LEVEL` (default: info)
- `ENVIRONMENT` (default: development)
