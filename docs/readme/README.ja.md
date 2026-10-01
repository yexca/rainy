<p align="center">
  <img src="../assets/rainy-readme-icon.png" width="128" height="128" alt="Rainy のアイコン">
</p>

<h1 align="center">Rainy</h1>

<p align="center">
  NAS 向けのセルフホスト型音楽サーバーです。Subsonic 互換で、インストールできる Web プレーヤーとライブラリ管理機能を備えています。
</p>

<p align="center">
  <a href="../../README.md">English</a> ·
  <a href="README.zh-Hans.md">简体中文</a> ·
  <a href="README.zh-Hant.md">繁體中文</a> ·
  <a href="README.ja.md">日本語</a>
</p>

<p align="center">
  <a href="https://github.com/yexca/rainy/releases"><img alt="Latest release" src="https://img.shields.io/github/v/release/yexca/rainy"></a>
  <a href="https://hub.docker.com/r/yexca/rainy"><img alt="Docker image" src="https://img.shields.io/badge/docker-yexca%2Frainy-2496ed?logo=docker&amp;logoColor=white"></a>
  <a href="../../LICENSE"><img alt="License: AGPL-3.0" src="https://img.shields.io/github/license/yexca/rainy"></a>
</p>

<p align="center">
  <img src="../assets/rainy-showcase.png" width="1200" alt="アルバムの棚、プレーヤーバー、再生中カード、隅で音楽を聴くマスコットが並ぶ Rainy のホーム画面">
</p>

Rainy は [Navidrome](https://www.navidrome.org/) と同じく、NAS 上の音楽フォルダをどこからでも聴けるプライベートなストリーミングサービスにします。違いはライブラリ管理を中心機能として扱う点で、タグ編集、カバー画像、歌詞、パターンによるリネーム、アップロード、削除、問題のあるファイルの検出をすべてブラウザ上で行えます。

> [!IMPORTANT]
> Rainy は開発中です。アップグレード前にデータディレクトリ（`/data`）をバックアップしてください。Web 上でタグ編集やリネームを行う前には、音楽ファイルもバックアップしてください。家庭内ネットワークの外に公開する前に[デプロイのセキュリティ](../operations/security.md)をお読みください。

## 主な機能

- MP3、FLAC、AAC/M4A/ALAC、Ogg Vorbis、Opus、WAV、AIFF、APE、WavPack、WMA、DSD などに対応。元ファイルをそのまま配信するほか、ffmpeg によるリアルタイム変換も可能です。
- Subsonic API 1.16.1 と OpenSubsonic 拡張を実装しており、Symfonium、Amperfy、play:Sub、Feishin などのクライアントをそのまま使えます。
- PWA としてインストールできる Web プレーヤー。iOS 風の再生画面、同期歌詞、ドラッグで並べ替えられるキュー、ロック画面操作、AirPlay に対応します。
- ブラウザでのライブラリ管理。一括タグ編集、カバー、歌詞、リネームと整理、アップロード、yt-dlp による YouTube と bilibili のリンクからのダウンロード（任意）、lx music 互換の音源によるオンライン検索とダウンロード（任意）、復元できるゴミ箱、ライブラリ診断、編集履歴。
- GBK、Big5、Shift-JIS の文字化けを読み込み時に修復し、UTF-8 でファイルに書き戻すこともできます。
- 個人の再生統計と再生履歴（月間・年間のまとめ付き）。Last.fm と ListenBrainz への再生記録の送信（Scrobble、任意）。
- デイリーミックスと、似た曲を自動で追加し続ける無限モード。どちらも自分のライブラリと再生履歴からサーバー上で選びます。
- 複数ユーザー。管理者、ライブラリ管理者、一般ユーザーの役割と、個別のダウンロード権限。

Web インターフェースは現在、英語と簡体字中国語に対応しています。

## クイックスタート

1. 空のディレクトリに [`docker-compose.yml`](../../docker-compose.yml) を置きます。
2. 同じディレクトリに `.env` を作成します。

   ```dotenv
   RAINY_MUSIC_PATH=/path/to/music
   PUID=1000
   PGID=1000
   TZ=Asia/Tokyo
   ```

3. 起動します。

   ```sh
   docker compose up -d --pull always
   ```

4. `http://<NAS の IP>:7650` を開き、初回アクセス時に管理者アカウントを作成します。Rainy は `/music` を最初のライブラリとして追加し、スキャンを開始します。

`PUID`/`PGID` には音楽フォルダを読み取れるユーザーを指定してください。Web 上でファイルを編集する場合は書き込み権限も必要です。NAS ごとの設定は [Docker と NAS ガイド](../operations/docker.md)、すべての設定項目は [`.env.example`](../../.env.example) と[設定](../operations/configuration.md)を参照してください。

## ランタイムデータ

| ホストのパス | コンテナのパス | 内容 |
| --- | --- | --- |
| `RAINY_DATA_PATH`（既定 `./data`） | `/data` | データベース、`secret.key`、カバーキャッシュ、ゴミ箱 |
| `RAINY_MUSIC_PATH` | `/music` | 音楽ファイル |

`secret.key` は保存されたパスワードの暗号化に使われます。`rainy.db` と一緒にバックアップし、厳重に保管してください。これらのディレクトリはリポジトリにコミットしないでください。

## ユーザー文書

[ユーザーガイド](../user/index.md)では、はじめに、ライブラリとスキャン、再生と PWA、再生レポートと Scrobble、ライブラリ管理、Subsonic クライアントを扱います。[運用ドキュメント](../operations/configuration.md)では、Docker と NAS へのデプロイ、設定、リバースプロキシ、データベースとバックアップ、トラブルシューティングを扱います（英語）。

## ドキュメント

詳しいドキュメント（英語）は[ドキュメント索引](../README.md)にあります。[ユーザーガイド](../user/index.md)、[リバースプロキシ](../operations/reverse-proxy.md)、[トラブルシューティング](../operations/troubleshooting.md)もご覧ください。

## セキュリティとプライバシー

ライブラリのデータと音声の利用にはサインインが必要です。Rainy はテレメトリを収集しません。脆弱性は [SECURITY.md](../../SECURITY.md) の手順で非公開に報告し、ログを共有する前に [PRIVACY.md](../../PRIVACY.md) をお読みください。

## 開発と貢献

[CONTRIBUTING.md](../../CONTRIBUTING.md)、[AGENTS.md](../../AGENTS.md)、[ローカル開発](../development/local-dev.md)をお読みください。

## 謝辞

Rainy は TagLib（WebAssembly にコンパイルした go-taglib 経由）、FFmpeg、SQLite（modernc.org/sqlite 経由）、そして有効にした場合は yt-dlp の上に成り立っています。Subsonic と OpenSubsonic の API のおかげで、いつものアプリからそのまま使えます。また [Navidrome](https://www.navidrome.org/) は、セルフホストの音楽サーバーがどれほど快適になれるかを示してくれました。

## ライセンス

Rainy は [GNU Affero General Public License v3.0](../../LICENSE) の下で提供されます。
