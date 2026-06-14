.PHONY: run test tidy fmt docker-up docker-down docker-logs

run:
	go run ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy

fmt:
	gofmt -w ./cmd ./internal

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f app
