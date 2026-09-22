#!/bin/sh
# 先跑 migration 再啟動 server；set -e 讓 migration 失敗時容器直接掛掉，
# 不會用舊 schema 起服務
set -e

migrate -path db/migration -database "$DB_SOURCE" -verbose up

exec /app/main
