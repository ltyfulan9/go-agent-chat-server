.PHONY: run worker test tidy fmt docker-up docker-down docker-logs metrics pprof-goroutine bench-api bench-mock-chat bench-async-chat bench-suite

run:
	go run ./cmd/server

worker:
	go run ./cmd/worker

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

metrics:
	./scripts/read_metrics.sh

pprof-goroutine:
	curl -s http://127.0.0.1:6060/debug/pprof/goroutine?debug=1 | head -80

bench-api:
	./scripts/bench_api.sh

bench-mock-chat:
	./scripts/bench_mock_chat.sh

bench-async-chat:
	./scripts/bench_async_chat.sh

bench-suite:
	python3 scripts/benchmark_suite.py --n 300 --c 30 --chat-n 8 --chat-c 2
