postgres:
	docker compose up -d

createdb:
	docker exec -it todoapp_db createdb --username=root --owner=root todoapp

dropdb:
	docker exec -it todoapp_db dropdb todoapp

ENV ?= dev

migrateup:
	go run ./cmd/migrate --env $(ENV) up

migratedown:
	go run ./cmd/migrate --env $(ENV) down

server:
	go run ./cmd/todoapp --env $(ENV)

test:
	go test -v -cover ./...

.PHONY: postgres createdb dropdb migrateup migratedown server test
