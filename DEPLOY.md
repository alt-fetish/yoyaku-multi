# デプロイ手順

## 前提条件

- [Fly.io CLI](https://fly.io/docs/hands-on/install-flyctl/) がインストール済み
- [Neon](https://neon.tech) のPostgreSQLプロジェクト作成済み
- [Resend](https://resend.com) のAPIキー取得済み（送信元ドメイン検証済み）
- `psql` コマンドが使用可能

---

## 手順1：環境変数ファイルの準備

```bash
cp .env.example .env
```

`.env` を編集して各値を設定：

```
DATABASE_URL=postgres://user:password@host/dbname?sslmode=require
RESEND_API_KEY=re_xxxxxxxxxxxxxxxxxxxx
ADMIN_PASSWORD=強力なパスワードを設定
SESSION_SECRET=最低32文字のランダム文字列
BASE_URL=https://yoyaku-multi.fly.dev
FROM_EMAIL=noreply@yourdomain.com
```

SESSION_SECRET の生成例：
```bash
openssl rand -hex 32
```

---

## 手順2：DBマイグレーション実行

```bash
./scripts/migrate.sh
```

または psql を直接使用：

```bash
psql "$DATABASE_URL" -f migrations/001_init.sql
psql "$DATABASE_URL" -f migrations/002_chat.sql
psql "$DATABASE_URL" -f migrations/003_sessions.sql
```

---

## 手順3：Fly.io アプリ作成

```bash
# ログイン
fly auth login

# アプリ作成（fly.toml の app 名と一致させる）
fly launch --name yoyaku-multi --region nrt --no-deploy

# または既存の fly.toml を使用してデプロイ
fly deploy
```

---

## 手順4：Fly.io シークレット設定

```bash
./scripts/deploy.sh --set-secrets
```

または手動で：

```bash
fly secrets set \
  DATABASE_URL="postgres://..." \
  RESEND_API_KEY="re_..." \
  ADMIN_PASSWORD="..." \
  SESSION_SECRET="..." \
  BASE_URL="https://yoyaku-multi.fly.dev" \
  FROM_EMAIL="noreply@yourdomain.com"
```

---

## 手順5：デプロイ

```bash
fly deploy
```

---

## 手順6：前日リマインダーのスケジュール設定

デプロイ完了後、以下で毎日 20:00 JST にリマインダーバッチを実行する Machine を作成：

```bash
./scripts/setup_reminder_cron.sh
```

または手動で：

```bash
fly machine run \
  --app yoyaku-multi \
  --region nrt \
  --schedule daily \
  --vm-memory 256 \
  --entrypoint "/app/reminder" \
  registry.fly.io/yoyaku-multi:latest
```

---

## 手順7：動作確認

1. `https://yoyaku-multi.fly.dev/` にアクセス → `/events` にリダイレクト
2. `/register` から参加者登録 → 確認メール受信
3. `/admin/login` でログイン
4. `/admin/events` から開催日登録
5. 登録した参加者で `/events` から参加表明
6. 管理者が `/admin/events/:id/entries` で参加者確定

---

## ローカル開発

```bash
# .env を用意して
go run ./cmd/server/
```

アクセス: `http://localhost:8080`

リマインダーを手動実行：

```bash
go run ./cmd/reminder/
```

---

## アプリ URL

- 参加者向けトップ: `/events`
- 参加者登録: `/register`
- マイページ: `/my/:token`
- 管理者ログイン: `/admin/login`
- 管理者ダッシュボード: `/admin/events`
