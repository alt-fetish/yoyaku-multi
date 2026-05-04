# tech.md — 技術構成

## スタック概要

| レイヤー | 技術 | 備考 |
|----------|------|------|
| バックエンド | Go 1.22+ | net/http標準ライブラリ中心 |
| HTMLテンプレート | templ | コンパイル型、型安全 |
| インタラクション | htmx | 部分レンダリング、JS最小化 |
| スタイリング | Plain CSS + CSS Variables | 外部依存ゼロ、ビルドステップなし |
| データベース | PostgreSQL (Neon) | 無料枠0.5GB、接続はpgx/v5 |
| メール送信 | Resend | 無料枠3,000通/月、既存利用実績あり |
| ホスティング | Fly.io | Go+Docker、無料枠あり |

---

## ディレクトリ構成

```
/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── handler/
│   │   ├── admin_events.go      # 開催日管理
│   │   ├── admin_applicants.go  # 参加者管理
│   │   ├── admin_sessions.go    # セッション履歴CRUD
│   │   ├── admin_chat.go        # チャット（管理者側）
│   │   ├── events.go            # 開催日一覧（参加者側）
│   │   ├── register.go          # 登録フォーム
│   │   └── mypage.go            # マイページ
│   ├── model/
│   │   ├── applicant.go
│   │   ├── event.go             # 開催日 + 参加表明
│   │   ├── session.go           # セッション履歴
│   │   ├── ng.go
│   │   └── chat.go
│   ├── mail/
│   │   └── resend.go
│   └── middleware/
│       └── auth.go
├── views/
│   ├── layout/
│   ├── admin/
│   │   ├── events/
│   │   ├── applicants/
│   │   ├── sessions/
│   │   └── chat/
│   ├── events/                  # 参加者向け開催日一覧
│   ├── register/
│   └── my/
├── static/
├── migrations/
│   ├── 001_init.sql
│   ├── 002_chat.sql
│   └── 003_sessions.sql
├── Dockerfile
├── fly.toml
└── .env.example
```

---

## DBスキーマ

```sql
-- 参加希望者（登録者台帳）
CREATE TABLE applicants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    handle      TEXT NOT NULL,
    email       TEXT NOT NULL UNIQUE,
    token       TEXT NOT NULL UNIQUE,  -- URLトークン（アカウント不要）
    status      TEXT NOT NULL DEFAULT 'active',  -- active / banned
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- NG設定（1登録者に複数行）
CREATE TABLE ng_settings (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    applicant_id  UUID NOT NULL REFERENCES applicants(id) ON DELETE CASCADE,
    action_key    TEXT NOT NULL,
    -- 'finger_mouth' | 'anal' | 'sheath' | 'kiss' | 'fellatio'
    is_ok         BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (applicant_id, action_key)
);

-- 開催日（管理者が先に設定する）
CREATE TABLE events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_type  TEXT NOT NULL CHECK (session_type IN ('solo', 'group')), -- 互換用。定員判定には使わない
    event_date    DATE NOT NULL,
    event_time    TIME NOT NULL,
    end_time      TIME,                     -- 終了時刻（CSVのendtimeから登録）
    capacity      INTEGER NOT NULL DEFAULT 2 CHECK (capacity > 0),
    status        TEXT NOT NULL DEFAULT 'open',
    -- open / confirmed / done / cancelled
    notes         TEXT,                     -- 管理者内部メモ
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (event_date, event_time)         -- 重複チェック用
);

CREATE INDEX idx_events_date ON events(event_date DESC);
CREATE INDEX idx_events_status ON events(status);

-- 参加表明（applicants ↔ events の中間テーブル）
CREATE TABLE event_entries (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id      UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    applicant_id  UUID NOT NULL REFERENCES applicants(id),
    status        TEXT NOT NULL DEFAULT 'pending',
    -- pending（表明中）/ confirmed（確定）/ declined（見送り）/ cancelled（本人取消）
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (event_id, applicant_id)
);

CREATE INDEX idx_event_entries_event_id ON event_entries(event_id);
CREATE INDEX idx_event_entries_applicant_id ON event_entries(applicant_id);

-- セッション履歴
CREATE TABLE sessions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_type  TEXT NOT NULL CHECK (session_type IN ('solo', 'group')),
    session_date  DATE NOT NULL,
    session_time  TIME NOT NULL,
    completed_at  TIMESTAMPTZ,
    status        TEXT NOT NULL DEFAULT 'confirmed',
    -- confirmed / done / cancelled
    notes         TEXT,
    event_id      UUID REFERENCES events(id),  -- 開催日から自動生成の場合はリンク
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_date ON sessions(session_date DESC);

-- セッション参加者（sessions ↔ applicants の多対多）
CREATE TABLE session_participants (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id    UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    applicant_id  UUID NOT NULL REFERENCES applicants(id),
    UNIQUE (session_id, applicant_id)
);

CREATE INDEX idx_session_participants_session_id ON session_participants(session_id);
CREATE INDEX idx_session_participants_applicant_id ON session_participants(applicant_id);

-- チャットメッセージ（管理者 ↔ 参加者 1対多）
CREATE TABLE chat_messages (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    applicant_id  UUID NOT NULL REFERENCES applicants(id) ON DELETE CASCADE,
    sender        TEXT NOT NULL CHECK (sender IN ('admin', 'applicant')),
    body          TEXT NOT NULL,
    is_read       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_messages_applicant_id ON chat_messages(applicant_id);
CREATE INDEX idx_chat_messages_created_at ON chat_messages(created_at);
```

---

## エンティティ関係図

```
applicants
  ├── ng_settings          (1対多)
  ├── event_entries        (多対多中間) ── events
  ├── session_participants (多対多中間) ── sessions
  └── chat_messages        (1対多)

events ──(event_id)──► sessions   (done時に自動生成)
```

---

## 認証設計

| 対象 | 方式 |
|------|------|
| 管理者画面 `/admin/*` | セッション認証（Cookie + 環境変数パスワード） |
| 参加者マイページ `/my/:token` | URLトークン認証 |
| 開催日一覧 `/events` | 認証なし（閲覧は公開、表明はトークン必要） |
| 登録フォーム `/register` | 認証なし（公開） |

---

## メール設計

| タイミング | 送信先 | 内容 |
|------------|--------|------|
| 登録完了時 | 登録者 | マイページURL（トークン付き） |
| 開催日作成時（任意） | 全登録者 | 新しい開催日のお知らせ |
| 参加確定時 | 確定参加者 | 日時・場所・同意確認リンク |
| 見送り通知時（任意） | 未選定者 | 今回は見送りの旨 |
| セッション前日 | 確定参加者 | リマインダー・同意再確認 |
| キャンセル時 | 確定参加者 | キャンセル通知 |

送信元署名：市川（ALT-FETISH）

---

## 環境変数

```
DATABASE_URL=       # NeonのPostgreSQL接続文字列
RESEND_API_KEY=     # Resend APIキー
ADMIN_PASSWORD=     # 管理者ログインパスワード
SESSION_SECRET=     # セッションCookieの署名キー
BASE_URL=           # アプリのベースURL
FROM_EMAIL=         # 送信元メールアドレス
```

---

## SOLID原則の適用方針

### S — 単一責任原則
各パッケージの責務を明確に分離する。

| パッケージ | 責務 |
|------------|------|
| `handler/` | リクエスト受付・バリデーション・レスポンス返却のみ |
| `model/` | DB操作のみ（ビジネスロジックを含む） |
| `mail/` | メール送信のみ |
| `middleware/` | 認証・ロギングのみ |

### O — 開放閉鎖原則
メール送信・DB操作をinterfaceで抽象化し、実装を差し替え可能にする。
新しいメールプロバイダーや別DBに変更しても、handlerを修正しない。

### L — リスコフ置換原則
interfaceを実装する型は、すべてその契約を完全に満たす。
GoにはクラスはないためInterface実装で担保する。

### I — インターフェース分離原則
大きなRepositoryインターフェースを作らず、用途別に小さく分割する。

```go
// model/interfaces.go
type EventReader    interface { Get(id UUID) (*Event, error) }
type EventWriter    interface { Create(e *Event) error; Update(e *Event) error }
type EventLister    interface { List(status string) ([]*Event, error) }
type ApplicantStore interface { Get(id UUID) (*Applicant, error); Create(a *Applicant) error }
type Mailer         interface { Send(to, subject, body string) error }
```

### D — 依存性逆転原則
handlerは具体型ではなくinterfaceに依存する。
`cmd/server/main.go` でDIを組み立てる。

```go
// handler は interface に依存
type EventHandler struct {
    events EventLister
    mailer Mailer
}

// main.go で具体型を注入
h := &EventHandler{
    events: &model.PostgresEventRepo{DB: db},
    mailer: &mail.ResendMailer{APIKey: cfg.ResendKey},
}
```

---

## ホスティング：Fly.io 選定理由

- Go + Dockerのデプロイが最もシンプル
- 無料枠：256MB RAM × 3台まで、月2,340時間
- Neon（PostgreSQL）と組み合わせて完全無料運用が可能
- `fly deploy` 一コマンドでデプロイ完結

---

## htmx利用方針

- 参加表明ボタンの即時反映（`hx-post` + `hx-swap`）
- 管理者のNG比較ビュー（2名選択時に動的表示）
- チャットのポーリング（`hx-trigger="every 10s"`）
- セッション削除の行即時除去（`hx-swap="outerHTML"`）

JSは原則書かない。htmxの属性のみで完結させる。

---

## CSS設計方針

### ファイル構成

```
static/
└── style.css   # 単一ファイルで管理
```

### 基本方針

- Tailwind等の外部CSSフレームワークは使用しない
- CSS Variables でデザイントークンを一元管理
- 全ページ（参加者向け・管理者向け問わず）レスポンシブ対応

### デザイントークン（例）

```css
:root {
    /* カラー */
    --color-bg:       #0f0f0f;
    --color-surface:  #1a1a1a;
    --color-border:   #2e2e2e;
    --color-accent:   #c0392b;
    --color-text:     #e0e0e0;
    --color-muted:    #888888;

    /* タイポグラフィ */
    --font-base: 'Helvetica Neue', Arial, sans-serif;
    --font-size-base: 16px;

    /* スペーシング */
    --space-sm:  8px;
    --space-md:  16px;
    --space-lg:  32px;

    /* その他 */
    --radius:    6px;
    --shadow:    0 2px 8px rgba(0,0,0,0.4);
}
```

### レスポンシブ設計

モバイルファーストで記述する。ブレークポイントは1種類のみ（シンプル維持）。

```css
/* モバイルベース（〜767px）*/
.container { padding: var(--space-md); }

/* PC（768px〜）*/
@media (min-width: 768px) {
    .container { max-width: 960px; margin: 0 auto; }
}
```

### 管理者画面のレイアウト

```css
/* モバイル：ハンバーガーメニュー非表示→縦積み */
/* PC：サイドバー + メインコンテンツの2カラム */
@media (min-width: 768px) {
    .admin-layout {
        display: grid;
        grid-template-columns: 220px 1fr;
    }
}
```

### SOLID原則との整合

CSSもSingleResponsibility を意識し、`style.css` 内をセクションコメントで明確に分割する。

```css
/* =========================
   1. Reset / Base
   2. Design Tokens
   3. Layout
   4. Components（Button, Form, Table, Badge, Chat）
   5. Pages（Admin, Events, MyPage）
   6. Utilities
   ========================= */
```
