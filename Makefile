postgres:
	docker compose -f docker-compose.local.yaml up -d db

createdb:
	docker exec -it todoapp_db createdb --username=root --owner=root todoapp

dropdb:
	docker exec -it todoapp_db dropdb todoapp

ENV ?= dev

migrateup:
	go run ./cmd/todoapp --env $(ENV) migrate up

migratedown:
	go run ./cmd/todoapp --env $(ENV) migrate down

server:
	go run ./cmd/todoapp --env $(ENV) serve

test:
	go test -v -cover ./...

.PHONY: postgres createdb dropdb migrateup migratedown server test
