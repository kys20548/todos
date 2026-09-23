// Package migration 把 internal/db/migration 底下的 .sql 檔案編進執行檔本身
// （go:embed），讓 cmd/migrate 產出的 binary 不用依賴磁碟上的檔案路徑就能跑
// migration——binary 丟到哪裡（本機、container、其他主機）都能單獨執行。
package migration

import "embed"

//go:embed *.sql
var FS embed.FS
