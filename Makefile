DB_URL=postgresql://root:secret@localhost:5432/todoapp?sslmode=disable

postgres:
	docker compose up -d

createdb:
	docker exec -it todoapp_db createdb --username=root --owner=root todoapp

dropdb:
	docker exec -it todoapp_db dropdb todoapp

migrateup:
	migrate -path db/migration -database "$(DB_URL)" -verbose up

migratedown:
	migrate -path db/migration -database "$(DB_URL)" -verbose down

sqlc:
	sqlc generate

server:
	go run main.go

test:
	go test -v -cover ./...

.PHONY: postgres createdb dropdb migrateup migratedown sqlc server test
