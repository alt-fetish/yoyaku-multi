#!/bin/bash
# migrate.sh — DBマイグレーション実行スクリプト
# 必要: DATABASE_URL 環境変数（またはローカルの .env ファイル）
#        psql コマンド
set -e

# .env 読み込み（存在する場合）
if [ -f .env ]; then
  export $(grep -v '^#' .env | xargs)
fi

if [ -z "$DATABASE_URL" ]; then
  echo "ERROR: DATABASE_URL が設定されていません"
  exit 1
fi

echo "=== DBマイグレーション開始 ==="
echo "対象DB: ${DATABASE_URL%%@*}@..."

psql "$DATABASE_URL" -f migrations/001_init.sql
echo "001_init.sql 完了"

psql "$DATABASE_URL" -f migrations/002_chat.sql
echo "002_chat.sql 完了"

psql "$DATABASE_URL" -f migrations/003_sessions.sql
echo "003_sessions.sql 完了"

psql "$DATABASE_URL" -f migrations/004_event_capacity.sql
echo "004_event_capacity.sql 完了"

psql "$DATABASE_URL" -f migrations/005_option_sets.sql
echo "005_option_sets.sql 完了"

echo "=== マイグレーション完了 ==="
