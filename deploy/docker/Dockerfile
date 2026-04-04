# ========== Go build stage ==========
FROM golang:1.25-alpine AS backend

ARG BUILD_VERSION=dev
ARG BUILD_COMMIT=unknown

RUN apk add --no-cache git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.Version=${BUILD_VERSION} -X main.Commit=${BUILD_COMMIT}" \
    -o /out/sreagent \
    ./cmd/server

# ========== Node build stage ==========
FROM node:20-alpine AS frontend

WORKDIR /src/web

COPY web/package.json web/package-lock.json* ./
RUN npm ci 2>/dev/null || npm install

COPY web/ .
RUN npm run build

# ========== Final runtime stage ==========
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl bash mysql-client \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# Copy binaries
COPY --from=backend         /out/sreagent           ./sreagent
# Copy frontend
COPY --from=frontend        /src/web/dist           ./web/dist
# Copy config and entrypoint
COPY configs/config.example.yaml                    ./configs/config.yaml
COPY deploy/docker/entrypoint.sh                    ./entrypoint.sh

# Create data directories
RUN mkdir -p logs \
    && chmod +x ./sreagent ./entrypoint.sh

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=15s \
  CMD curl -f http://localhost:8080/healthz || exit 1

ENTRYPOINT ["./entrypoint.sh"]
