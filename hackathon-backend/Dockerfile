FROM golang:1.24-alpine

# システムパッケージの更新とgitのインストール
RUN apk update && apk add --no-cache git

WORKDIR /app

# Goのビルド設定
ENV CGO_ENABLED=0
ENV GOPROXY=https://proxy.golang.org,direct

# 【修正箇所】Airのバージョンを安定版(v1.61.1)に固定します
RUN go install github.com/air-verse/air@v1.61.1

# 定義ファイルをコピー
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをコピー
COPY . .

CMD ["air"]