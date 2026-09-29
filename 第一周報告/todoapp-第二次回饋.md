# todoapp 修正紀錄（2026-09-23）

> Mentor Discussion · 討論紀錄

2026-09-23 ｜ 作者 **danny**

今天跟指導員討論後定下三個練習方向，重點都在「不靠 AI、自己把部署會用到的基本功練熟」。這篇記錄每項要做什麼、為什麼要這樣做。

---

## 01 SSH key：用本機已經打通的 key，不要在目標機器上自己 gen

不在跳板機或目標機器上另外產生新的 key，而是直接用本機那把已經被信任的 key，透過 agent forwarding（`ssh -A`）一路帶過去。這樣中間的機器上不會多出私鑰，要撤銷權限也只要管本機那一把。

### Windows 本機先啟用 ssh-agent

要用管理員身分執行：

```cmd
sc config ssh-agent start= auto
net start ssh-agent
```

> 注意 `start= auto` 的等號後面一定要有空格，這是 sc 的語法規定。

接著用一般 cmd 就可以：

```cmd
ssh-add %USERPROFILE%\.ssh\id_ed25519
ssh-add -l
```

### 透過 agent forwarding 連線

```bash
# 本機先確認 agent 裡有 key
ssh-add -l

# -A：把本機的 ssh-agent 轉送過去，在 jump 上再 ssh 到下一台時，用的還是本機那把 key
ssh -A jump
ssh 10.140.0.29          # 在 jump 上直接連，不用輸密碼、也不用在 jump 放私鑰
```

> 要注意：開了 `-A` 之後，那台遠端機器的 root 在連線期間也能借用你的 agent，所以只對信任的機器開。如果只是要穿過跳板機，`ssh -J jump devbox`（或 config 裡的 `ProxyJump`）不用把 agent 交出去，會更安全。

## 02 Docker：手動安裝，不透過 AI，也不用 apt-get 連外網

目的是搞清楚 Docker 到底由哪些元件組成、各自放在哪裡，而不是一行 `apt-get install` 帶過。做法是先在能上網的本機下載安裝檔，傳到目標機器後再離線安裝：

| 步驟 | 內容 |
|---|---|
| 1. 下載 | 在本機從 Docker 官方下載對應發行版的 `.deb`：`containerd.io`、`docker-ce-cli`、`docker-ce`、`docker-buildx-plugin`、`docker-compose-plugin` |
| 2. 傳檔 | 用 `scp` 傳到目標機器（見第 03 項） |
| 3. 安裝 | `sudo dpkg -i *.deb` |
| 4. 驗證 | `sudo systemctl status docker`、`docker version`、`docker compose version` |
| 5. 權限 | `sudo usermod -aG docker $USER`，重新登入後就不用每次都加 sudo |

### 安裝 docker by apt

```bash
sudo apt remove docker docker-engine docker.io containerd runc -y
sudo apt update && sudo apt upgrade -y
sudo apt install -y ca-certificates curl gnupg
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo tee /etc/apt/keyrings/docker.asc > /dev/null
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl start docker
sudo systemctl enable docker

sudo docker run hello-world
```

刪除 docker：

```bash
dpkg -l | grep -i docker
sudo apt-get purge -y docker-engine docker docker.io docker-ce docker-ce-cli docker-compose-plugin containerd.io docker-buildx-plugin
sudo apt-get autoremove -y --purge docker-engine docker docker.io docker-ce docker-compose-plugin containerd.io docker-buildx-plugin
```

### .deb 安裝 docker

先確認環境：

```bash
lsb_release -cs              # 例如 noble（24.04）、jammy（22.04）
dpkg --print-architecture    # 通常是 amd64
dpkg -l iptables | grep ^ii  # 確認有裝 iptables（docker-ce 需要它）
```

| 套件 | 檔名範例 |
|---|---|
| containerd.io | `containerd.io_<版本>~noble_amd64.deb` |
| docker-ce-cli | `docker-ce-cli_5%3a<版本>~noble_amd64.deb` |
| docker-ce | `docker-ce_5%3a<版本>~noble_amd64.deb` |
| docker-buildx-plugin | `docker-buildx-plugin_<版本>~noble_amd64.deb` |
| docker-compose-plugin | `docker-compose-plugin_<版本>~noble_amd64.deb` |
| docker-ce-rootless-extras（選用） | `docker-ce-rootless-extras_5%3a<版本>~noble_amd64.deb` |

> ⚠️ docker-ce 和 docker-ce-cli 的版本號必須一樣。

```bash
sudo dpkg -i ./*.deb                 # 一次全部安裝，dpkg 會自己處理安裝順序

sudo systemctl enable --now docker   # 開機自動啟動，並立刻啟動
sudo usermod -aG docker $USER        # 讓目前使用者不用 sudo 就能執行 docker
```

加入 docker 群組後要登出再登入才會生效。

步驟 5：確認安裝成功

```bash
systemctl status docker --no-pager
docker version
docker compose version
docker info | head -20
```

之後要移除：

```bash
sudo dpkg --purge docker-compose-plugin docker-buildx-plugin docker-ce-rootless-extras docker-ce docker-ce-cli containerd.io
```

## 03 常用指令熟悉：scp 等

部署會反覆用到的指令要能不查就打出來。先從檔案傳輸開始：

```bash
# 本機 → 遠端
scp todoapp.tar.gz jump:~/todoapp/

# 整個目錄要加 -r
scp -r config jump:~/todoapp/

# 遠端 → 本機
scp jump:~/todoapp/docker-compose.yaml .

# 經過跳板機傳到內網那台
scp -J jump config/app.dev.env danny@10.140.0.29:~/
```
