# デプロイ状況メモ

## 現在の公開URL

- 公開URL: https://yoyaku.alt-fetish.com/
- イベント一覧: https://yoyaku.alt-fetish.com/events
- 管理者ログイン: https://yoyaku.alt-fetish.com/admin/login

このURLは外部公開されている。

## DNS

- ドメイン: alt-fetish.com
- サブドメイン: yoyaku.alt-fetish.com
- DNS管理元: さくらインターネット
- ネームサーバ:
  - ns1.dns.ne.jp
  - ns2.dns.ne.jp
- DNSレコード:
  - yoyaku.alt-fetish.com A 140.83.39.110
- 宛先IP:
  - 140.83.39.110
  - AS31898 Oracle Corporation

DNSの流れ:

```text
yoyaku.alt-fetish.com
  -> さくらインターネットDNS
  -> Aレコード 140.83.39.110
  -> Oracle Cloud のComputeインスタンス
```

## Oracle Cloud

- Oracleアカウント名: east-inc
- ログインメールアドレス: ichihara@very-fp.com
- 認証: スマホの Oracle Mobile Authenticator

### Compute

- インスタンス名: yoyaku-multi
- 状態: 実行中
- シェイプ: VM.Standard.E2.1.Micro
- OCPU数: 1
- メモリー: 1 GB
- ネットワーク帯域幅: 0.48 Gbps
- パブリックIP: 140.83.39.110

VM.Standard.E2.1.Micro は Oracle Cloud Always Free 対象の代表的なシェイプ。現在の構成であれば、Compute本体は無料枠内の想定。

### Storage

- ブート・ボリューム名: yoyaku-multi (Boot Volume)
- サイズ: 50 GB
- 状態: アタッチ済
- イメージ: Canonical-Ubuntu-22.04-2026.02.28-0
- 追加ブロック・ボリューム: なし

ブート・ボリューム50GBのみで、追加ブロック・ボリュームはない。Oracle Cloud Always Free のブロック・ボリューム無料枠内の想定。

## デプロイ方法

現在は GitHub Actions から Oracle Cloud のサーバへSSHデプロイしている。

- Workflow: .github/workflows/deploy.yml
- Workflow名: Deploy to Oracle Cloud
- トリガー: main ブランチへの push
- GitHub Secrets:
  - SSH_PRIVATE_KEY
  - SSH_HOST
  - SSH_USER

デプロイ時の処理:

```text
1. Go/templ をセットアップ
2. templ generate
3. Linux amd64向けに server_bin / reminder_bin をビルド
4. SSH/SCPでOracle Cloud上のサーバへ転送
5. /opt/yoyaku/server と /opt/yoyaku/reminder を更新
6. systemctl restart yoyaku
```

サーバ側では systemd の yoyaku サービスとして動いている。

## 課金について

現時点で確認した構成は以下のため、無料枠内の想定。

- Compute: VM.Standard.E2.1.Micro
- OCPU: 1
- メモリ: 1 GB
- Boot Volume: 50 GB
- 追加Block Volume: なし

念のため、Oracle Cloud Console の「請求とコスト管理 > コスト分析」で、今月の Compute と Block Volume のコストが 0 であることを定期確認する。

## Fly.ioについて

リポジトリには Fly.io 用の設定や手順が残っている。

- fly.toml
- DEPLOY.md
- CUSTOM_DOMAIN.md
- scripts/deploy.sh
- scripts/setup_reminder_cron.sh

ただし現在の実運用は Oracle Cloud へのGitHub Actionsデプロイ。Fly.io は初期検討または過去運用の名残。
