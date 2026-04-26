# SREAgent — CI/CD 与部署指南

> 最后更新：2026-04-04

---

## 目录

1. [CI 流水线（GitHub Actions）](#ci-流水线)
2. [Dockerfile（多阶段构建）](#dockerfile)
3. [入口脚本与启动流程](#入口脚本)
4. [配置变量](#配置变量)
5. [Kubernetes 部署](#kubernetes-部署)
6. [本地开发](#本地开发)
7. [构建与发布流程](#构建与发布流程)

---

## CI 流水线

**文件**：`.github/workflows/docker-build.yml`

### 触发规则

| 事件 | 条件 | 行为 |
|------|------|------|
| `push` | 分支 `main` | 构建 + 推送 `:latest` 标签 |
| `push` | 标签 `v*`（如 `v1.2.3`） | 构建 + 推送 `:v1.2.3`、`:1.2`、`:1`、`:latest` |
| `pull_request` | 目标 `main` | 仅构建（不推送）— CI 验证 |

### 作业

流水线包含 3 个作业：

#### 作业 1：`test` — Go 单元测试
```yaml
runs-on: ubuntu-latest
steps:
  - checkout
  - setup-go (version from go.mod, with cache)
  - go test ./... -timeout 120s
```
> 注意：目前为空通过（尚无 `*_test.go` 文件）。

#### 作业 2：`typecheck` — 前端 TypeScript 检查
```yaml
runs-on: ubuntu-latest
steps:
  - checkout
  - setup-node 20 (cache: npm, key: web/package-lock.json)
  - npm ci (in web/)
  - npm run typecheck (in web/)
```

#### 作业 3：`build-and-push` — 多架构 Docker 镜像
```yaml
runs-on: ubuntu-latest
needs: [test, typecheck]   # runs after both jobs pass
steps:
  - checkout
  - setup QEMU (for arm64 cross-compilation)
  - setup Docker Buildx
  - login to Docker Hub (skip on PR)
  - docker/metadata-action → generate tags
  - docker/build-push-action:
      context: .
      file: deploy/docker/Dockerfile
      platforms: linux/amd64, linux/arm64
      push: true (false on PR)
      cache: GitHub Actions cache (GHA)
      build-args:
        BUILD_VERSION=${{ github.ref_name }}
        BUILD_COMMIT=${{ github.sha }}
  - (optional) update Docker Hub README
```

### 所需 GitHub Secrets

| Secret | 用途 |
|--------|------|
| `DOCKERHUB_USERNAME` | Docker Hub 登录用户名 |
| `DOCKERHUB_TOKEN` | Docker Hub 访问令牌 |

### 镜像命名

镜像名称为 `${{ secrets.DOCKERHUB_USERNAME }}/sreagent`。

标签示例：
- 推送到 `main` → `user/sreagent:latest`
- 推送标签 `v1.2.3` → `user/sreagent:v1.2.3`、`user/sreagent:1.2`、`user/sreagent:1`、`user/sreagent:latest`
- PR #42 → `user/sreagent:pr-42`（仅构建不推送）

---

## Dockerfile

**文件**：`deploy/docker/Dockerfile`

### 构建阶段

| 阶段 | 基础镜像 | 用途 |
|------|----------|------|
| `backend` | `golang:1.24-alpine` | 使用 ldflags 编译 Go 二进制文件 |
| `frontend` | `node:20-alpine` | 构建 Vue 3 SPA |
| `final` | `alpine:3.20` | 最小化运行时镜像 |

### 构建参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `BUILD_VERSION` | `dev` | 通过 ldflags 注入为 `-X main.Version` |
| `BUILD_COMMIT` | `unknown` | 通过 ldflags 注入为 `-X main.Commit` |

### 最终镜像包含内容

```
/app/
├── sreagent              # Go 二进制文件（静态链接，CGO_ENABLED=0）
├── web/dist/             # 构建后的 Vue SPA 资源
├── configs/config.yaml   # config.example.yaml 的副本
├── entrypoint.sh         # 启动脚本
└── logs/                 # 空目录（应用默认输出日志到 stdout）
```

### 运行时依赖

通过 `apk add --no-cache` 安装：
- `ca-certificates` — TLS 证书信任库
- `tzdata` — 时区支持（默认 Asia/Shanghai）
- `curl` — 健康检查探测
- `bash` — 入口脚本
- `mysql-client` — 启动时创建数据库

### 健康检查

```dockerfile
HEALTHCHECK --interval=15s --timeout=3s --start-period=15s \
  CMD curl -f http://localhost:8080/healthz || exit 1
```

### 本地构建

```bash
# 基本构建
make docker-build

# 包含版本信息
docker build \
  --build-arg BUILD_VERSION=v1.0.0 \
  --build-arg BUILD_COMMIT=$(git rev-parse HEAD) \
  -t sreagent:v1.0.0 \
  -f deploy/docker/Dockerfile .
```

---

## 入口脚本

**文件**：`deploy/docker/entrypoint.sh`

### 启动顺序

1. **等待 MySQL 就绪** — 通过 TCP 探测 `${DB_HOST}:${DB_PORT}`，最多重试 60 次（间隔 2 秒 = 超时 2 分钟）
2. **创建数据库** — 通过 `mysql` CLI 执行 `CREATE DATABASE IF NOT EXISTS`：
   - 首先使用应用凭据（`SREAGENT_DATABASE_*`）尝试
   - 如果失败且设置了 `MYSQL_ROOT_PASSWORD`，则使用 root 重试
   - 如果都失败，则假定数据库已存在并继续
3. **启动服务器** — `exec ./sreagent --config configs/config.yaml`
   - golang-migrate 在启动时自动运行（嵌入式 SQL 迁移）
   - 如果没有用户存在，会创建管理员用户 `admin/admin123`

### 入口脚本环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SREAGENT_DATABASE_HOST` | `127.0.0.1` | MySQL 主机 |
| `SREAGENT_DATABASE_PORT` | `3306` | MySQL 端口 |
| `SREAGENT_DATABASE_USERNAME` | `sreagent` | MySQL 用户名 |
| `SREAGENT_DATABASE_PASSWORD` | `sreagent` | MySQL 密码 |
| `SREAGENT_DATABASE_DATABASE` | `sreagent` | 数据库名称 |
| `MYSQL_ROOT_PASSWORD` | （空） | 可选的 root 密码，用于创建数据库的备选方案 |

---

## 配置变量

### 配置文件（`configs/config.yaml`）

所有值都可以通过带有 `SREAGENT_` 前缀的环境变量使用 Viper 的 `AutomaticEnv()` 进行覆盖。点号变为下划线：`database.host` → `SREAGENT_DATABASE_HOST`。

```yaml
server:
  host: "0.0.0.0"           # SREAGENT_SERVER_HOST
  port: 8080                 # SREAGENT_SERVER_PORT
  mode: "debug"              # SREAGENT_SERVER_MODE ("debug" | "release")

database:
  driver: "mysql"            # SREAGENT_DATABASE_DRIVER
  host: "127.0.0.1"         # SREAGENT_DATABASE_HOST
  port: 3306                 # SREAGENT_DATABASE_PORT
  username: "sreagent"       # SREAGENT_DATABASE_USERNAME
  password: "change-me"     # SREAGENT_DATABASE_PASSWORD
  database: "sreagent"       # SREAGENT_DATABASE_DATABASE
  charset: "utf8mb4"        # SREAGENT_DATABASE_CHARSET
  max_idle_conns: 10         # SREAGENT_DATABASE_MAX_IDLE_CONNS
  max_open_conns: 100        # SREAGENT_DATABASE_MAX_OPEN_CONNS
  max_lifetime: 3600         # SREAGENT_DATABASE_MAX_LIFETIME

redis:
  host: "127.0.0.1"         # SREAGENT_REDIS_HOST
  port: 6379                 # SREAGENT_REDIS_PORT
  password: ""               # SREAGENT_REDIS_PASSWORD
  db: 0                      # SREAGENT_REDIS_DB
  pool_size: 100             # SREAGENT_REDIS_POOL_SIZE

jwt:
  secret: "change-me"       # SREAGENT_JWT_SECRET
  expire: 86400              # SREAGENT_JWT_EXPIRE (seconds)
  issuer: "sreagent"         # SREAGENT_JWT_ISSUER

log:
  level: "info"              # SREAGENT_LOG_LEVEL
  format: "json"             # SREAGENT_LOG_FORMAT
  output: "stdout"           # SREAGENT_LOG_OUTPUT
  file: "logs/sreagent.log"  # SREAGENT_LOG_FILE

engine:
  enabled: true              # SREAGENT_ENGINE_ENABLED
  sync_interval: 30          # SREAGENT_ENGINE_SYNC_INTERVAL (seconds)
```

### 手动读取的环境变量

这些通过 `os.Getenv()` 直接读取，**不**通过 Viper：

| 变量 | 说明 | 示例 |
|------|------|------|
| `SREAGENT_SECRET_KEY` | AES-256-GCM 主密钥，用于加密数据库敏感字段。64 位十六进制字符 = 32 字节。 | `a1b2c3...`（64 字符） |
| `SREAGENT_DB_DEBUG` | 启用 GORM SQL 调试日志（`"true"` 为启用） | `"false"` |
| `CORS_ALLOWED_ORIGINS` | 允许的 CORS 源列表，逗号分隔 | `"https://sreagent.example.com"` |

### AI 与飞书配置

AI 和飞书凭据**不**存储在配置文件或环境变量中。它们以加密形式（AES-256-GCM）存储在 `system_settings` 数据库表中，并通过 Web UI 的**设置 → AI 配置**和**设置 → 飞书机器人**进行管理。

---

## Kubernetes 部署

### 目录结构

```
deploy/kubernetes/
├── 00-namespace.yaml           # Namespace: sreagent
├── app/
│   ├── configmap.yaml          # 嵌入的 config.yaml
│   ├── secret.yaml             # 4 个密钥：db-password, redis-password, jwt-secret, secret-key
│   ├── deployment.yaml         # 1 副本，滚动更新，init 容器
│   ├── service.yaml            # ClusterIP 80→8080
│   ├── ingress.yaml            # 带 TLS 的 NGINX Ingress
│   └── hpa.yaml                # HPA min=1, max=3（CPU 80%）
├── mysql/                      # MySQL 8.0 StatefulSet + configmap + secret
├── redis/                      # Redis 7 StatefulSet + secret
├── helm/                       # （空 — 预留未来使用）
└── kustomize/                  # （空 — 预留未来使用）
```

### 部署顺序

```bash
# 1. 创建命名空间
kubectl apply -f deploy/kubernetes/00-namespace.yaml

# 2. 部署依赖服务
kubectl apply -f deploy/kubernetes/mysql/
kubectl apply -f deploy/kubernetes/redis/

# 3. 等待 MySQL 和 Redis 就绪
kubectl -n sreagent wait --for=condition=ready pod -l app=mysql --timeout=120s
kubectl -n sreagent wait --for=condition=ready pod -l app=redis --timeout=60s

# 4. 创建密钥（先编辑 base64 值！）
kubectl apply -f deploy/kubernetes/app/secret.yaml

# 5. 部署应用
kubectl apply -f deploy/kubernetes/app/configmap.yaml
kubectl apply -f deploy/kubernetes/app/deployment.yaml
kubectl apply -f deploy/kubernetes/app/service.yaml
kubectl apply -f deploy/kubernetes/app/ingress.yaml
kubectl apply -f deploy/kubernetes/app/hpa.yaml
```

### 密钥（Base64 编码）

应用前请先编辑 `deploy/kubernetes/app/secret.yaml`：

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sreagent-secret
  namespace: sreagent
type: Opaque
data:
  db-password: <base64>        # → SREAGENT_DATABASE_PASSWORD
  redis-password: <base64>     # → SREAGENT_REDIS_PASSWORD
  jwt-secret: <base64>         # → SREAGENT_JWT_SECRET
  secret-key: <base64>         # → SREAGENT_SECRET_KEY (64 hex chars)
```

生成 base64 值：
```bash
echo -n 'your-db-password' | base64
echo -n 'your-redis-password' | base64
echo -n 'your-jwt-secret' | base64
echo -n 'your-64-char-hex-key' | base64
```

### Init 容器

部署包含 2 个 init 容器：
1. `wait-for-mysql` — `busybox:1.36`，轮询 `nc -z mysql-svc 3306`
2. `wait-for-redis` — `busybox:1.36`，轮询 `nc -z redis 6379`

### 注入的环境变量

```yaml
env:
  - name: SREAGENT_DATABASE_PASSWORD  # from secret: db-password
  - name: SREAGENT_REDIS_PASSWORD     # from secret: redis-password
  - name: SREAGENT_JWT_SECRET         # from secret: jwt-secret
  - name: SREAGENT_SECRET_KEY         # from secret: secret-key
  - name: TZ                          # "Asia/Shanghai"
  - name: SREAGENT_DB_DEBUG           # "false"
  - name: CORS_ALLOWED_ORIGINS        # "https://sreagent.example.com"
```

### 资源限制

```yaml
resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 512Mi
```

### 探针

| 探针 | 路径 | 初始延迟 | 周期 |
|------|------|----------|------|
| 存活探针 | `GET /healthz` | 20s | 15s |
| 就绪探针 | `GET /healthz` | 10s | 5s |

### 扩缩容

- 默认：1 副本（单实例 — 告警引擎状态机为内存模式）
- HPA：min=1，max=3，目标 CPU=80%
- **注意**：多副本需要为告警引擎引入分布式锁（Phase 2）

### 滚动更新策略

```yaml
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxUnavailable: 0   # 零停机
    maxSurge: 1
```

### ConfigMap 变更时触发滚动重启

由于未使用 Helm 模板，需手动更新注解：
```bash
kubectl -n sreagent annotate deployment sreagent --overwrite \
  checksum/config=$(sha256sum deploy/kubernetes/app/configmap.yaml | cut -d' ' -f1)
```

### 更新镜像

```bash
# CI 推送新标签后
kubectl -n sreagent set image deployment/sreagent \
  sreagent=ghcr.io/sreagent/sreagent:v1.2.3
```

---

## 本地开发

### 前置条件

- Go 1.24+
- Node 20+
- MySQL 8.0（或使用 Docker）
- Redis 7（或使用 Docker）
- （可选）`air` 用于热重载：`go install github.com/air-verse/air@latest`
- （可选）`golangci-lint` 用于代码检查

### 快速开始

```bash
# 1. 启动依赖服务
make docker-up     # 在 Docker 容器中启动 MySQL + Redis

# 2. 复制配置文件
cp configs/config.example.yaml configs/config.yaml
# 编辑 configs/config.yaml 设置本地配置

# 3. 运行后端（带热重载）
make dev           # 使用 air 进行热重载
# 或
make run           # 构建并运行一次

# 4. 运行前端（另开终端）
make web-install   # 仅首次需要
make web-dev       # 启动带 HMR 的 Vite 开发服务器
```

### Makefile 目标

| 目标 | 说明 |
|------|------|
| `make help` | 显示所有目标及说明 |
| `make build` | 构建 Go 二进制文件到 `bin/sreagent` |
| `make run` | 构建并运行服务器 |
| `make dev` | 热重载运行（需要 `air`） |
| `make test` | 运行 Go 测试并生成覆盖率报告 |
| `make lint` | 运行 `golangci-lint` |
| `make fmt` | 格式化 Go 代码（`go fmt` + `goimports`） |
| `make tidy` | `go mod tidy` |
| `make web-install` | 安装前端 npm 依赖 |
| `make web-dev` | 启动前端开发服务器 |
| `make web-build` | 构建前端生产版本 |
| `make docker-up` | 启动 MySQL + Redis 容器 |
| `make docker-down` | 停止 MySQL + Redis 容器 |
| `make docker-build` | 本地构建 Docker 镜像 |
| `make db-migrate` | 运行数据库迁移（构建并运行二进制文件） |
| `make clean` | 删除 `bin/`、`web/dist/`、`web/node_modules/` |
| `make all` | `tidy` + `fmt` + `build` + `web-build` |

### 默认管理员凭据

首次启动空数据库时，服务器会创建：
- 用户名：`admin`
- 密码：`admin123`

**请在生产环境中立即修改。**

---

## 构建与发布流程

### 开发流程

1. 从 `main` 创建功能分支
2. 本地开发和测试（`make dev` + `make web-dev`）
3. 推送分支 → 创建 PR → CI 运行 `test` + `typecheck` + 构建（不推送）
4. 审核并合并到 `main` → CI 构建 + 推送 `:latest`

### 发布流程

1. 确保 `main` 分支稳定
2. 创建并推送语义化版本标签：
   ```bash
   git tag v1.2.3
   git push origin v1.2.3
   ```
3. CI 构建 + 推送：`:v1.2.3`、`:1.2`、`:1`、`:latest`
4. 更新 K8s 部署：
   ```bash
   kubectl -n sreagent set image deployment/sreagent \
     sreagent=ghcr.io/sreagent/sreagent:v1.2.3
   ```

### 回滚

```bash
# 查看部署历史
kubectl -n sreagent rollout history deployment/sreagent

# 回滚到上一个版本
kubectl -n sreagent rollout undo deployment/sreagent

# 或回滚到指定版本
kubectl -n sreagent rollout undo deployment/sreagent --to-revision=3
```
