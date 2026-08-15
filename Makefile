.PHONY: all build run test test-race docker-up docker-down docker-logs k6-load k6-stress clean

all: test build

build:
	go build -o bin/gophermart ./cmd/server

run:
	go run ./cmd/server/main.go

test:
	go test -v ./...

test-race:
	go test -race -v ./tests/... ./internal/worker/...

docker-up:
	docker-compose up -d --build postgres redis api

docker-down:
	docker-compose down -v

docker-logs:
	docker-compose logs -f api

k6-load:
	docker-compose run --rm k6 run /scripts/load_test.js

k6-stress:
	docker-compose run --rm k6 run /scripts/overselling_stress_test.js

clean:
	rm -rf bin/ coverage.out

