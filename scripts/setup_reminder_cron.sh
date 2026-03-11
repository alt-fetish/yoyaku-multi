#!/bin/bash
# setup_reminder_cron.sh — 前日リマインダーのスケジューラ設定
# Fly.io Machines API で毎日 20:00 JST（= 11:00 UTC）に実行する
set -e

APP_NAME="yoyaku-multi"
REGION="nrt"

echo "前日リマインダーのスケジュールを設定します..."

# Fly.io の Scheduled Machine を作成
# NOTE: これは fly.toml とは別に machine として登録する
fly machine run \
  --app "$APP_NAME" \
  --region "$REGION" \
  --schedule "daily" \
  --vm-memory 256 \
  --entrypoint "/app/reminder" \
  registry.fly.io/"$APP_NAME":latest

echo "スケジュール設定完了"
echo "毎日 11:00 UTC（20:00 JST）に前日のリマインダーが送信されます"
