<p align="center">
  <img src="../assets/rainy-readme-icon.png" width="128" height="128" alt="Rainy 圖示">
</p>

<h1 align="center">Rainy</h1>

<p align="center">
  為 NAS 打造的自架音樂伺服器：相容 Subsonic，內建美觀的網頁播放器，還能直接在網頁上整理音樂庫。
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
  <img src="../assets/rainy-showcase.png" width="1200" alt="Rainy 首頁：專輯列表、播放列、正在播放卡片，以及在角落聽歌的看板娘">
</p>

Rainy 與 [Navidrome](https://www.navidrome.org/) 定位相近，把 NAS 上的音樂資料夾變成隨時隨地可聽的私人串流服務。不同之處在於 Rainy 把**音樂庫管理**視為核心功能：編輯標籤、更換封面、加入歌詞、依規則重新命名、上傳、刪除、找出有問題的檔案，都能在瀏覽器中完成。

> [!IMPORTANT]
> Rainy 仍在積極開發中。升級前請備份資料目錄（`/data`）；在網頁上編輯標籤或重新命名之前，請先備份音樂檔案。將實例公開到家用網路之外前，請閱讀[部署安全](../operations/security.md)。

## 功能

- 支援 MP3、FLAC、AAC/M4A/ALAC、Ogg Vorbis、Opus、WAV、AIFF、APE、WavPack、WMA、DSD 等格式；原始檔直接串流，也可透過 ffmpeg 即時轉碼。
- 實作 Subsonic API 1.16.1 與 OpenSubsonic 擴充，可直接使用 Symfonium、Amperfy、play:Sub、Feishin 等用戶端。
- 可安裝為 PWA 的網頁播放器：iOS 風格的「正在播放」畫面、同步歌詞、可拖曳排序的佇列、鎖定畫面控制與 AirPlay。
- 在瀏覽器中管理音樂庫：批次編輯標籤、封面、歌詞、重新命名與整理、上傳、透過 yt-dlp 從 YouTube 與嗶哩嗶哩連結下載（選用）、透過相容 lx music 的音源線上搜尋並下載歌曲（選用）、可還原的資源回收筒、音樂庫健檢與修改紀錄。
- 自動修復 GBK、Big5、Shift-JIS 編碼造成的亂碼，並可用 UTF-8 永久寫回檔案。
- 個人收聽統計與播放紀錄（含月度與年度回顧），並可將播放紀錄同步到 Last.fm 與 ListenBrainz（選用）。
- 每日推薦，以及自動接續相似歌曲的無限模式，全部在伺服器上依你自己的音樂庫與播放紀錄計算。
- 多使用者：管理員、音樂庫管理者與一般使用者，下載權限可個別設定。

網頁介面目前提供英文與簡體中文。

## 快速開始

1. 在空目錄中放入 [`docker-compose.yml`](../../docker-compose.yml)。
2. 在同一目錄建立 `.env`：

   ```dotenv
   RAINY_MUSIC_PATH=/path/to/music
   PUID=1000
   PGID=1000
   TZ=Asia/Taipei
   ```

3. 啟動：

   ```sh
   docker compose up -d --pull always
   ```

4. 開啟 `http://<NAS 的 IP>:7650`，首次造訪時建立管理員帳號。Rainy 會自動將 `/music` 加入為第一個音樂庫並開始掃描。

`PUID`/`PGID` 須能讀取音樂資料夾；若要在網頁上編輯檔案，還需要寫入權限。各家 NAS 的設定方式請參考 [Docker 與 NAS 指南](../operations/docker.md)，所有選項請參考 [`.env.example`](../../.env.example) 與[設定](../operations/configuration.md)。

## 執行期資料

| 主機路徑 | 容器路徑 | 內容 |
| --- | --- | --- |
| `RAINY_DATA_PATH`（預設 `./data`） | `/data` | 資料庫、`secret.key`、封面快取、資源回收筒 |
| `RAINY_MUSIC_PATH` | `/music` | 你的音樂 |

`secret.key` 用於加密儲存的密碼，請與 `rainy.db` 一併備份並妥善保管。請勿將這些目錄提交到版本庫。

## 使用者文件

[使用者指南](../user/index.md)涵蓋入門、音樂庫與掃描、播放與 PWA、收聽報表與 Scrobble、音樂庫管理以及 Subsonic 用戶端。[維運文件](../operations/configuration.md)涵蓋 Docker 與 NAS 部署、設定、反向代理、資料庫與備份以及疑難排解（目前為英文）。

## 文件

完整文件（英文）請見[文件索引](../README.md)，包括[使用者指南](../user/index.md)、[反向代理](../operations/reverse-proxy.md)與[疑難排解](../operations/troubleshooting.md)。

## 安全與隱私

音樂庫資料與音訊都需要登入才能存取，Rainy 不收集任何遙測資料。請透過 [SECURITY.md](../../SECURITY.md) 私下回報漏洞，分享記錄檔前請閱讀 [PRIVACY.md](../../PRIVACY.md)。

## 開發與貢獻

請閱讀 [CONTRIBUTING.md](../../CONTRIBUTING.md)、[AGENTS.md](../../AGENTS.md) 與[本地開發](../development/local-dev.md)。

## 致謝

Rainy 建立在 TagLib（透過編譯為 WebAssembly 的 go-taglib）、FFmpeg、SQLite（透過 modernc.org/sqlite）之上，啟用時也會使用 yt-dlp。Subsonic 與 OpenSubsonic API 讓它能直接搭配你已在使用的用戶端；[Navidrome](https://www.navidrome.org/) 則讓人看見自架音樂伺服器可以多麼好用。

## 授權

Rainy 採用 [GNU Affero General Public License v3.0](../../LICENSE)。
