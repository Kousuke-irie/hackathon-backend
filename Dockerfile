FROM golang:1.24-alpine AS builder

# 依存パッケージのインストール (gitは不要)
RUN apk update && apk add --no-cache ca-certificates

WORKDIR /app

# Goのビルド設定
ENV CGO_ENABLED=0

# 依存モジュールをダウンロード (キャッシュを効かせるため先に実行)
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをコピーし、バイナリをビルド
COPY . .
# バイナリ名は 'main' とし、静的リンクを適用
RUN go build -ldflags '-s -w' -o /usr/local/bin/main .

FROM alpine:latest
# Cloud Runがデフォルトで使用するポート
ENV PORT 8080 
EXPOSE 8080

WORKDIR /app

# ステージ1でビルドしたバイナリをコピー
COPY --from=builder /usr/local/bin/main .

# 実行可能なエントリポイント
CMD ["./main"]