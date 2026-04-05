.PHONY: build run test lint docker

build:
	go build -o bin/server cmd/server/main.go

run:
	go run cmd/server/main.go

test:
	go test ./... -v -race

lint:
	golangci-lint run ./...

docker:
	docker build -t gondor-search .
