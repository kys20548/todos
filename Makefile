postgres:
	docker compose up -d

createdb:
	docker exec -it todoapp_db createdb --username=root --owner=root todoapp

dropdb:
	docker exec -it todoapp_db dropdb todoapp

migrateup:
	go run ./cmd/migrate up

migratedown:
	go run ./cmd/migrate down

sqlc:
	sqlc generate

server:
	go run ./cmd/todoapp

test:
	go test -v -cover ./...

.PHONY: postgres createdb dropdb migrateup migratedown sqlc server test
