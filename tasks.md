# tasks.md — 実装タスク一覧

## フェーズ1：基盤構築

### 環境・プロジェクト初期化
- [ ] Go モジュール初期化（`go mod init`）
- [ ] templ インストール・設定
- [ ] Neon プロジェクト作成、接続文字列取得
- [ ] `.env` ファイル作成（`.env.example` をもとに）
- [ ] Fly.io アプリ作成（`fly launch`）
- [ ] Dockerfile 作成
- [ ] GitHub リポジトリ作成・初回push

### DBマイグレーション
- [ ] `migrations/001_init.sql` — applicants / ng_settings / events / event_entries
- [ ] `migrations/002_chat.sql` — chat_messages
- [ ] `migrations/003_sessions.sql` — sessions / session_participants
- [ ] マイグレーション実行・pgx/v5 接続確認

### CSS基盤
- [ ] `static/style.css` 作成
  - デザイントークン（CSS Variables）定義
  - Reset / Base スタイル
  - レイアウト（`.container`・`.admin-layout` グリッド）
  - 共通コンポーネント（Button / Form / Table / Badge / Chat吹き出し）
  - モバイルファースト、ブレークポイント768px一本

---

## フェーズ2：参加者登録（参加者側）

### バックエンド
- [ ] `GET /register` — 登録フォーム表示
- [ ] `POST /register` — 登録データ受信・バリデーション・DB保存
  - URLトークン生成（`crypto/rand`）
  - applicants + ng_settings をトランザクションで挿入
- [ ] `GET /register/done` — 登録完了画面

### テンプレート
- [ ] `views/register/form.templ`
  - ハンドルネーム・メールアドレス入力
  - NG行為チェックリスト（デフォルト全NG）
- [ ] `views/register/done.templ`

### メール
- [ ] 登録完了メール送信（Resend）
  - マイページURL（`/my/:token`）付き

---

## フェーズ3：開催日一覧・参加表明（参加者側）

### バックエンド
- [ ] `GET /events` — 公開中の開催日一覧（認証不要）
- [ ] `POST /events/:id/entry` — 参加表明（トークン認証）
  - event_entries にレコード挿入
  - 既に表明済みの場合は取消（toggle）
- [ ] `GET /my/:token` — マイページ（参加表明一覧・NG設定確認）

### テンプレート
- [ ] `views/events/index.templ`
  - 日時・種別・残枠・参加表明ボタン
  - ボタン押下後のhx-swapで即時「取消」表示に切替
- [ ] `views/my/index.templ`
  - 参加表明一覧（ステータスバッジ付き）
  - NG設定確認欄
  - チャット欄（フェーズ6で追加）

---

## フェーズ4：管理者画面 基礎

### 認証
- [ ] `GET /admin/login` — ログインフォーム
- [ ] `POST /admin/login` — セッション発行
- [ ] `GET /admin/logout` — セッション破棄
- [ ] 認証ミドルウェア（`/admin/*` 全体に適用）

### 開催日管理（`/admin/events`）
- [ ] `GET /admin/events` — 開催日一覧（ステータスフィルタ付き）
- [ ] `GET /admin/events/new` — 手動新規作成フォーム
- [ ] `POST /admin/events` — 開催日作成
  - 作成後、一斉通知メール送信の確認ダイアログ（htmx modal）
- [ ] `GET /admin/events/:id/edit` — 編集フォーム
- [ ] `PUT /admin/events/:id` — 更新（`open` のみ日時変更可）
- [ ] `DELETE /admin/events/:id` — 削除（`open` のみ、`hx-confirm`）
- [ ] `views/admin/events/index.templ`
  - ページ上部に「＋ セッション登録」ボタンを設置 → `GET /admin/events/new` に遷移
  - 隣に「📎 CSVアップロード」ボタンを設置 → `GET /admin/events/upload` に遷移
- [ ] `views/admin/events/new.templ`
- [ ] `views/admin/events/edit.templ`

### CSVアップロード（一括登録）
- [ ] `internal/model/event.go` に `BulkCreateEvents(rows []EventRow) BulkResult` を追加
  - `EventRow`：date / start_time / end_time / session_type
  - `BulkResult`：成功件数・スキップ行（重複）・エラー行（パース失敗）のリスト
  - `UNIQUE(event_date, event_time)` 制約違反をキャッチしてスキップ扱いに
  - 正常行はトランザクションで一括INSERT

- [ ] `GET /admin/events/upload` — CSVアップロード画面
- [ ] `POST /admin/events/upload/preview` — CSVパース・プレビュー表示（htmx）
  - ファイルを受信してメモリ上でパース（DBには未保存）
  - 正常行・重複行・エラー行を色分けテーブルで表示
  - 「この内容で登録する」確定ボタンを表示
- [ ] `POST /admin/events/upload/confirm` — プレビュー確認後にDB登録実行
  - セッションにパース済みデータを一時保持（またはhiddenフィールドで渡す）
  - 登録結果サマリーを表示（N件登録、M件スキップ）

- [ ] `views/admin/events/upload.templ`
  - ファイル選択（`<input type="file" accept=".csv">`）
  - プレビューエリア（`hx-post` + `hx-target`）
  - 結果サマリーエリア

- [ ] CSVフォーマットバリデーション
  - ヘッダー行の存在確認（`date,starttime,endtime,type`）
  - 日付形式：`YYYY-MM-DD`
  - 時刻形式：`HH:MM`
  - type：`solo` / `group` 以外はエラー行扱い
  - 空行・コメント行（`#`始まり）はスキップ

### 参加表明者一覧・確定操作
- [ ] `GET /admin/events/:id/entries` — 参加表明者一覧
  - 各参加者のNG設定を表示
- [ ] `GET /admin/events/:id/entries/compare` — NG比較ビュー（htmx）
  - 2名チェックボックス選択 → 比較表を動的表示
  - 両者OK / 片方NG / 両者NG を色分け
- [ ] `POST /admin/events/:id/confirm` — 参加者確定
  - 選定した参加者の event_entries.status を `confirmed` に更新
  - events.status を `confirmed` に更新
  - sessions + session_participants をトランザクションで自動生成
  - 確定メール送信（参加者それぞれに）
- [ ] `POST /admin/events/:id/decline` — 見送り通知（任意）
- [ ] `views/admin/events/entries.templ`

---

## フェーズ5：参加者管理・セッション履歴CRUD

### 参加者管理（`/admin/applicants`）
- [ ] `GET /admin/applicants` — 登録者一覧（参加回数・NG設定・チャットリンク）
- [ ] `GET /admin/applicants/:id` — 個別詳細（NG設定・参加履歴）
- [ ] `views/admin/applicants/index.templ`
- [ ] `views/admin/applicants/show.templ`

### セッション履歴 CRUD（`/admin/sessions`）
- [ ] `internal/model/session.go`
  - `ListSessions(statusFilter)`
  - `GetSession(id)`
  - `CreateSession(type, date, time, notes, eventID, applicantIDs[])`
  - `UpdateSession(id, ...)` — `done` 時に `completed_at = NOW()` 自動セット
  - `DeleteSession(id)` — CASCADE で session_participants も削除

**Read**
- [ ] `GET /admin/sessions` — 履歴一覧（日付降順・ステータスフィルタ）
- [ ] `views/admin/sessions/index.templ`
  - 種別バッジ（solo / group）・参加者ハンドル・日時・ステータス

**Create**
- [ ] `GET /admin/sessions/new` — 手動新規登録フォーム
- [ ] `POST /admin/sessions` — 登録（トランザクション）
- [ ] `views/admin/sessions/new.templ`
  - 種別選択で参加者セレクトボックスを動的切替（htmx）

**Update**
- [ ] `GET /admin/sessions/:id/edit` — 編集フォーム
- [ ] `PUT /admin/sessions/:id` — 更新
- [ ] `views/admin/sessions/edit.templ`

**Delete**
- [ ] `DELETE /admin/sessions/:id` — 物理削除（`hx-confirm`・行即時除去）

---

## フェーズ6：チャット機能

### バックエンド
- [ ] `internal/model/chat.go`
  - `GetMessages(applicantID)`
  - `PostMessage(applicantID, sender, body)`
  - `MarkAsRead(applicantID)`
  - `UnreadCount(applicantID)`

### 管理者側
- [ ] `GET /admin/chat/:applicant_id` — チャット画面
- [ ] `POST /admin/chat/:applicant_id` — メッセージ送信
- [ ] `GET /admin/chat/:applicant_id/messages` — ポーリング用（差分HTML）
- [ ] `views/admin/chat/show.templ`
  - 吹き出しUI（admin/applicantで左右分け）
  - `hx-trigger="every 10s"` で自動更新
- [ ] 参加者一覧に未読バッジ表示

### 参加者側
- [ ] `POST /my/:token/chat` — メッセージ送信
- [ ] `GET /my/:token/chat/messages` — ポーリング用
- [ ] `views/my/index.templ` にチャット欄を追加

---

## フェーズ7：メール仕上げ・前日リマインダー

- [ ] 全メールテンプレートの文面整備（Resend）
- [ ] 前日リマインダーバッチ（Fly.io スケジューラ or cron）
  - 翌日の `confirmed` セッションを取得
  - 参加者全員に同意再確認メール送信

---

## フェーズ8：デプロイ・本番稼働

- [ ] Fly.io シークレット設定（`fly secrets set`）
- [ ] 本番デプロイ（`fly deploy`）
- [ ] 動作確認（登録→開催日表明→確定→通知の一連フロー）
- [ ] 独自ドメイン設定（任意）

---

## 保留・将来検討

- 参加者が自分でNG設定を変更できる編集フォーム
- セッション履歴CSV出力
- 開催日の一斉通知メール配信履歴の管理
