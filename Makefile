.PHONY: up down logs test lint

up:
	docker-compose up --build

down:
	docker-compose down -v

logs:
	docker-compose logs -f app

test:
	go test ./...

lint:
	golangci-lint run ./...

locust:
	locust -f loadtest/locust.py --host=http://localhost:8080
