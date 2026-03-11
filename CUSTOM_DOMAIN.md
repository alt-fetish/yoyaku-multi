# カスタムドメインを Fly.io に設定する手順

対象: `yoyaku.alt-fetish.com` → Fly.io の `yoyaku-multi` アプリ

---

## 前提

- `alt-fetish.com` のDNS管理はサクラインターネットで行っている
- `yoyaku.alt-fetish.com` というサブドメインを新たに使いたい
- flyctl（Fly.io の CLI）がローカルPCにインストール済み

---

## STEP 1：Fly.io にカスタムドメインを登録する

**コマンドは自分のPC（ローカル）のターミナルで打ちます。サーバーではありません。**

```bash
export PATH=$PATH:/home/ryotaro/.fly/bin
flyctl certs add yoyaku.alt-fetish.com
```

成功するとIPアドレスが表示されます。

```
A    yoyaku.alt-fetish.com → 66.241.125.159
AAAA yoyaku.alt-fetish.com → 2a09:xxxx:xxxx::1  （IPv6）
```

このIPアドレスを次のステップで使います。

---

## STEP 2：サクラインターネットでサブドメインを作成する

### 2-1. サブドメインの作成

1. サクラインターネットの会員メニューにログイン
2. 「ドメイン」→ 対象ドメイン（`alt-fetish.com`）→ 「ゾーン編集」または「DNSレコード設定」を開く
3. **サブドメインを新規作成**する（`yoyaku` という名前で）
4. サブドメインの設定画面に入ると「レコード名 `@`」のグループが表示される

> ここでの `@` は `yoyaku.alt-fetish.com` 自体を指します。

### 2-2. A レコードの追加

| 項目 | 値 |
|------|-----|
| レコード名 | `@`（またはそのグループ） |
| タイプ | `A` |
| IPアドレス | STEP 1 で表示された IPv4（例: `66.241.125.159`） |

> もし同じグループに古いAレコード（別のIPアドレス）がある場合は**削除してください**。
> 2つのAレコードがあると、古いサーバーに繋がることがあります。

### 2-3. AAAA レコードの追加

| 項目 | 値 |
|------|-----|
| レコード名 | `@` |
| タイプ | `AAAA` |
| IPアドレス | STEP 1 で表示された IPv6 |

設定を保存します。

---

## STEP 3：DNS の伝播を待つ

DNS設定の変更が世界に広まるまで、**数分〜最大1時間**かかります。

確認コマンド（ローカルターミナルで）：

```bash
dig yoyaku.alt-fetish.com A
```

表示された IP が Fly.io のアドレスになっていればOKです。

---

## STEP 4：SSL証明書の確認

Fly.io は **Let's Encrypt** を使って SSL 証明書を**自動発行・自動更新**します。
自分で証明書を取得したり設定したりする必要はありません。

DNS が伝播したあと、以下のコマンドで証明書の状態を確認します：

```bash
export PATH=$PATH:/home/ryotaro/.fly/bin
flyctl certs show yoyaku.alt-fetish.com
```

`Status: Issued` と表示されれば証明書の発行完了です。

---

## STEP 5：動作確認

ブラウザで以下にアクセスして表示されれば完了です：

```
https://yoyaku.alt-fetish.com
```

---

## トラブルシューティング

| 症状 | 対処 |
|------|------|
| `dig` で古いIPが返ってくる | DNS伝播待ち（最大1時間） |
| 証明書が `Pending` のまま | DNS伝播を待ってから再確認 |
| `https` でアクセスできない | `flyctl certs show` で状態確認 |
| 古いサーバーに繋がる | 古いAレコードを削除したか確認 |
