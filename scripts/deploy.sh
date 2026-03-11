#!/bin/bash
# deploy.sh — Fly.io デプロイスクリプト
# 実行前に: chmod +x scripts/deploy.sh
set -e

APP_NAME="yoyaku-multi"

echo "=== Fly.io デプロイ開始 ==="

# 1. シークレット設定（初回のみ必要。値は .env から読み込む）
if [ "$1" = "--set-secrets" ]; then
  echo "シークレットを設定しています..."
  source .env
  fly secrets set \
    DATABASE_URL="$DATABASE_URL" \
    RESEND_API_KEY="$RESEND_API_KEY" \
    ADMIN_PASSWORD="$ADMIN_PASSWORD" \
    SESSION_SECRET="$SESSION_SECRET" \
    BASE_URL="$BASE_URL" \
    FROM_EMAIL="$FROM_EMAIL" \
    --app "$APP_NAME"
  echo "シークレット設定完了"
fi

# 2. templ ファイルをコンパイル（Dockerfile内でも実行されるが念のため）
echo "templ コンパイル中..."
templ generate

# 3. デプロイ
echo "デプロイ中..."
fly deploy --app "$APP_NAME"

echo "=== デプロイ完了 ==="
echo "URL: https://$APP_NAME.fly.dev"
