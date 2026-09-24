# todoapp 修正紀錄

> Review Fixes · 修正紀錄

2026-09-22 ｜ 更新 2026-09-23 ｜ 作者 **danny**

指導員看過第一版程式碼後開出五項修正，全部已完成，之後再補上第六項設定檔依環境切換；這篇記錄每項改了什麼、為什麼改，給後面接手的人看。

---

## 01 錯誤代碼：改成物件包裝

原本 `errcode.Code` 是 int 常數，訊息另外放在一個獨立的 `map[Code]string` 裡維護，兩邊要一起改、容易漏改一邊。改成 `Code{ID, Msg}` 這個物件，一個變數宣告同時帶碼跟訊息，格式也從數字改成字串：`E000` 成功、`E0xx` 通用錯誤、`E1xx` todo 專用。

## 02 資料夾結構與指令化

整個專案改成標準 Go 專案的 `cmd/`、`internal/`、`pkg/`、`third_party/` 慣例：

| 變化 | 內容 |
|---|---|
| 進入點 | `main.go` 搬進 `cmd/todoapp/main.go` |
| 內部套件 | `api/`、`db/`、`errcode/`、`util/` 全部搬進 `internal/`（本來就不打算給外部 import） |
| `pkg/`、`third_party/` | 建空資料夾占位，目前沒有東西適合放進去 |

用 urfave/cli 包成一支正常的 CLI

## 03 日誌：zerolog 換成 log/slog

`zerolog` 全部換成標準庫 `log/slog`

`go.mod` 也把 `rs/zerolog` 這顆依賴拿掉。額外發現 gorm 預設的 logger 會把每次「查不到」印成一行帶顏色的純文字到 stdout，跟服務自己的 slog 結構化 log 是兩套不相干的格式，而且把正常的 404 印成刺眼的錯誤，順便把它關掉（`LogMode(Silent)`），交給 access log 自己記。

## 04 容器：docker-compose 加 pgAdmin

`docker-compose.yaml` 新增 `pgadmin` service（`dpage/pgadmin4`）

現在 `docker compose up -d --build` 會同時啟動 3 個 service：`postgres`、`app`、`pgadmin`。

## 05 資料庫 ORM：sqlc 換成 gorm

選擇跟大家一樣的就好

## 06 設定檔：依環境用指令切換

原本只有一份根目錄的 `app.env`，要換環境只能改檔案內容。改成每個環境一份，放在 `config/` 底下，啟動時用 `--env` 指定要讀哪一份：

| 變化 | 內容 |
|---|---|
| 設定檔 | `app.env` 拆成 `config/app.dev.env`、`config/app.qa.env`、`config/app.prod.env` |
| 選環境 | `todoapp` 跟 `migrate` 都吃 `--env dev\|qa\|prod`，也可以用環境變數 `APP_ENV`；沒給預設 `dev` |
| 防呆 | 不在清單內的值（例如打錯成 `prd`）直接啟動失敗，不會默默讀不到設定 |
| `ENVIRONMENT` | 從設定檔拿掉，改由 `--env` 決定，避免「用 qa 啟動、檔案卻寫 development」兩邊對不上；`dev` 用文字 log + gin debug mode，`qa`／`prod` 用 JSON log + release mode |
| Makefile／compose | `make server ENV=qa`；`APP_ENV=qa docker compose up -d`，compose 改成掛整個 `./config` 目錄 |

注意：預設是 `dev`，image 不透過 compose 直接 `docker run` 又沒帶 `APP_ENV` 的話會吃到 dev 設定，部署 qa／prod 時要明確帶上。
