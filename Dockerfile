FROM golang:1.24-alpine AS builder

WORKDIR /app

# 依存関係をコピー・ダウンロード
COPY go.mod go.sum ./
RUN go mod download

# templ のインストール
RUN go install github.com/a-h/templ/cmd/templ@latest

# ソースコードをコピー
COPY . .

# templ ファイルをコンパイル
RUN templ generate

# Go バイナリをビルド
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server/
RUN CGO_ENABLED=0 GOOS=linux go build -o /reminder ./cmd/reminder/

# --- 実行ステージ ---
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Tokyo

WORKDIR /app

COPY --from=builder /server /app/server
COPY --from=builder /reminder /app/reminder
COPY --from=builder /app/static /app/static

EXPOSE 8080

CMD ["/app/server"]
