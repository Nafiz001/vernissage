.PHONY: setup db db-stop migrate ingest seed dev server web test test-server test-web build up

DATABASE_URL ?= postgres://vernissage:vernissage@127.0.0.1:5491/vernissage?sslmode=disable
TEST_DATABASE_URL ?= postgres://vernissage:vernissage@127.0.0.1:5491/vernissage_test?sslmode=disable
export DATABASE_URL

setup:
	cd server && go mod download
	cd web && npm install

# PostgreSQL 18 in Docker on port 5491, with a second database for tests.
db:
	docker compose up -d db
	@until docker compose exec -T db pg_isready -U vernissage >/dev/null 2>&1; do sleep 1; done
	-docker compose exec -T db createdb -U vernissage vernissage_test 2>/dev/null

db-stop:
	docker compose stop db

migrate:
	cd server && go run ./cmd/vernissage migrate

# Read both museums' collections. The server looks at every picture in the
# background once it's running; `make seed` then hangs six exhibitions.
ingest:
	cd server && go run ./cmd/vernissage ingest

seed:
	cd server && go run ./cmd/vernissage seed

server:
	cd server && go run ./cmd/vernissage serve

web:
	cd web && npm run dev

# Both, stopped together with Ctrl+C.
dev:
	@echo "server  http://localhost:8790"
	@echo "web     http://localhost:3790"
	@$(MAKE) -j2 server web

test: test-server test-web

test-server:
	cd server && gofmt -l . | (! grep .) && go vet ./...
	cd server && VERNISSAGE_TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race ./...

test-web:
	cd web && npm run lint && npm run typecheck

build:
	cd server && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/vernissage ./cmd/vernissage
	cd web && npm run build

# The whole thing in containers behind Caddy on http://localhost:8080.
up:
	docker compose --profile full up --build
