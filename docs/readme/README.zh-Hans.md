<p align="center">
  <img src="../assets/rainy-readme-icon.png" width="128" height="128" alt="Rainy 图标">
</p>

<h1 align="center">Rainy</h1>

<p align="center">
  给 NAS 用的自托管音乐服务器：兼容 Subsonic，自带好看的 Web 播放器，还能直接在网页里整理曲库。
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
  <img src="../assets/rainy-showcase.png" width="1200" alt="Rainy 首页：专辑列表、播放栏、正在播放卡片，以及在角落听歌的看板娘">
</p>

Rainy 和 [Navidrome](https://www.navidrome.org/) 定位相似：把 NAS 上的音乐文件夹变成随时随地可听的私人流媒体服务。不同的是，Rainy 把**曲库管理**当成一等公民——改标签、换封面、加歌词、按规则重命名、上传、删除、找出有问题的文件，都能在浏览器里完成，不必再开电脑跑 Mp3tag 或 foobar2000。

Rainy 是一个内嵌 React 网页端的 Go 程序，以单个 Docker 镜像发布，支持 `linux/amd64` 和 `linux/arm64`（群晖 Plus 系列、ARM 版 NAS、树莓派 4/5 都能用）。

> [!IMPORTANT]
> Rainy 仍在积极开发中。升级前请备份数据目录（`/config`）；在网页里编辑标签、重命名之前，请先备份音乐文件。把实例暴露到家庭网络之外前，请阅读[部署安全](../operations/security.md)。

## 功能

- **常见格式都能播**：MP3、FLAC、AAC/M4A/ALAC、OGG Vorbis、Opus、WAV、AIFF、APE、WavPack、WMA、DSD（DSF/DFF）等。原始音质直连播放并支持拖动进度，也可以用 ffmpeg 实时转码为 MP3、Opus 或 AAC。
- **兼容现有客户端**：实现 Subsonic API 1.16.1 和 OpenSubsonic 扩展，Symfonium、Amperfy、play:Sub、Feishin、Tempo、DSub 等客户端可以直接使用；支持密码、令牌和 API 密钥三种认证方式。
- **值得安装的播放器**：网页端可安装为 PWA，手机上有 iOS 风格的「正在播放」页面、同步歌词、可拖动排序的队列、锁屏和耳机控制（Media Session）、iOS 上的 AirPlay、浅色/深色主题，界面支持中英文。
- **在浏览器里管理曲库**：单曲或批量编辑标签（先预览再应用）、封面、内嵌或 `.lrc` 歌词、按规则重命名/整理、上传、通过 yt-dlp 从 YouTube 和哔哩哔哩链接下载（可选）、通过兼容 lx music 的音源在线搜索并下载歌曲（可选）、可还原的回收站、曲库体检和完整的修改历史。
- **对中文曲库友好**：扫描时自动修复 GBK、Big5、Shift-JIS 编码造成的乱码，也可以一键以 UTF-8 永久写回；中文艺人名按拼音首字母归类。
- **你的收听统计**：像 Last.fm 一样的个人收听报表和播放记录，附带月度和年度回顾，还可以把网页端和 Subsonic 客户端的播放记录同步到 Last.fm 和 ListenBrainz（可选）。
- **为你推荐**：每日推荐，以及队列快播完时自动续上相似歌曲的无限模式，全部在你的服务器上根据你自己的曲库和播放记录计算，不向外发送任何数据。
- **多用户**：管理员、曲库管理员、普通用户三种角色，下载权限可单独控制。收藏、评分、播放次数、歌单和播放队列都按用户独立保存，队列还能在网页端和 Subsonic 客户端之间接着听。

## 快速开始

需要一台装了 Docker（含 Compose 插件）的 NAS 或 Linux 主机。

1. 新建一个目录，把 [`docker-compose.yml`](../../docker-compose.yml) 放进去。
2. 在同一目录创建 `.env`，指向你的音乐目录：

   ```dotenv
   RAINY_MUSIC_PATH=/path/to/music
   PUID=1000
   PGID=1000
   TZ=Asia/Shanghai
   ```

3. 启动：

   ```sh
   docker compose up -d --pull always
   docker compose logs -f rainy   # 看到 "Rainy is listening" 即可
   ```

4. 浏览器打开 `http://<NAS 的 IP>:7650`，第一次访问时创建管理员账号。Rainy 会自动把 `/data` 添加为第一个音乐库并开始扫描。

[`.env.example`](../../.env.example) 列出了所有可选项：镜像、数据目录、主机端口、扫描间隔、日志级别、反向代理信任和登录有效期。修改 `.env` 后执行 `docker compose up -d` 生效；`docker compose restart` 不会重新读取 `.env`。需要可复现的部署时，请在 `.env` 中把 `RAINY_IMAGE` 固定为某个发布版本或镜像 digest。完整说明见[配置](../operations/configuration.md)。

## 目录与权限（PUID / PGID）

| 主机路径 | 容器路径 | 内容 | 权限 |
| --- | --- | --- | --- |
| `RAINY_DATA_PATH`（默认 `./config`） | `/config` | 数据库 `rainy.db`、密钥 `secret.key`、封面缓存、回收站、上传暂存、yt-dlp 及加密保存的 Cookie | 可写；启动时自动改为 `PUID:PGID` 所有 |
| `RAINY_MUSIC_PATH`（默认 `./data`） | `/data` | 你的音乐 | 播放只需可读；**要在网页里改标签、换封面、重命名、上传、删除，就必须可写** |

旧安装将应用数据挂载到 `/data`、音乐挂载到 `/music` 时，请先按
[挂载路径升级说明](../operations/docker.md#upgrading-the-mount-layout)保留数据库和密钥，并修改已有音乐库的路径。

容器以 root 启动，只是为了把 `/config` 的所有者改为 `PUID:PGID`，随后立即降权运行。**`/data` 不会被 chown**，它的权限由你的 NAS 决定。

- SSH 登录 NAS 执行 `id 你的用户名`，例如得到 `uid=1026 gid=100`，就填 `PUID=1026`、`PGID=100`，并在共享文件夹权限里给该用户读写权限。
- 只想播放、不想让 Rainy 改动文件时，可以把音乐目录挂载为只读（给 `/data` 卷加上 `:ro`，写法见 [Docker 指南](../operations/docker.md#permissions-puid-and-pgid)）。
- `UMASK`（默认 `022`）设为 `002` 时，同组用户也能修改 Rainy 写入的文件。

常见 NAS 的参考值：

| NAS | 常用 PUID:PGID | 说明 |
| --- | --- | --- |
| 群晖 DSM 7.2+ | `1026:100` | Container Manager →「项目」→ 新增 |
| 威联通 QNAP | 从 `500:100` 起 | Container Station → 应用程序 → 创建；不要用 admin |
| Unraid | `99:100` | 共享默认属于 `nobody:users` |
| TrueNAS SCALE 24.10+ | `568:568` | Apps → Install via YAML；给 `apps` 用户 Modify 权限 |
| OpenMediaVault 6/7 | `1000:100` | openmediavault-compose 插件 |

每种 NAS 的逐步说明（可直接粘贴的 compose、路径示例）见 [Docker 与 NAS 指南](../operations/docker.md)。

## 反向代理与 HTTPS

内网直接用 `http://<NAS 的 IP>:7650` 播放没有问题。需要外网访问，或者想把网页**安装为 PWA**（Service Worker 只在 HTTPS 或 `localhost` 下工作）时，就需要 HTTPS：

- 设置 `RAINY_TRUST_PROXY=true`，让登录限流看到真实客户端 IP；
- 代理需要传递 `X-Forwarded-Proto`；
- `/api/events` 是 SSE 长连接，**必须关闭缓冲**，否则扫描进度不动；
- 记得调大上传的请求体大小限制。

nginx、Caddy、群晖 DSM 反向代理和 Tailscale 的配置示例见[反向代理](../operations/reverse-proxy.md)。目前不支持挂在子路径下，请使用独立的（子）域名。

## 使用 Subsonic 客户端

在客户端里填写服务器地址 `http://<NAS 的 IP>:7650` 或 `https://music.example.com`（**不要**加 `/rest`）、Rainy 用户名和密码即可。支持 OpenSubsonic API 密钥的客户端可以只填 API 密钥：在网页「设置 → Subsonic 客户端」中生成（只显示一次），随时可以撤销。推荐客户端和转码规则见[客户端](../user/clients.md)。

## 中文乱码修复

很多老的中文 MP3 用 GBK、Big5 或 Shift-JIS 写入标签，却声明为 Latin-1，于是在大多数播放器里显示成 `ÖÜ½ÜÂ×` 这样的乱码。Rainy 分两步处理：

1. **读取时自动修复（默认开启）**：扫描时还原成正确文字存入数据库，**不修改文件**。开关在「系统 → 服务器设置」。
2. **永久修复文件**：在「管理 → 体检」的「编码问题」里逐个字段预览原值和新值，确认后以 UTF-8 重新写入标签（需要音乐目录可写）。

识别规则很保守，正常的西文标签（如 `Exémplé`）不受影响。详见[曲库说明](../user/library.md#fixing-garbled-tags)。

## 备份与升级

所有状态都在数据目录里。**`rainy.db` 和 `secret.key` 必须一起备份**：`secret.key` 用于加密存储用户密码，丢了它所有用户都要重置密码；两者同时泄露则可还原出所有密码，请妥善保管备份。

```sh
docker compose stop rainy
tar czf rainy-backup-$(date +%F).tar.gz --exclude='./config/cache' --exclude='./config/tmp' ./config
docker compose start rainy
```

升级：

```sh
docker compose up -d --pull always
```

数据库结构会在启动时自动迁移。音乐文件本身请用 NAS 的快照或 Hyper Backup 等方式备份。详见[数据库与备份](../operations/database.md)。

## 命令行工具

`docker compose exec` 默认以 root 执行，请用 `-u` 指定你的 PUID:PGID：

```sh
# 忘记密码：重置并登出该用户的所有会话
docker compose exec -u 1000:1000 rainy rainy user reset-password admin '新密码'
# 列出用户
docker compose exec -u 1000:1000 rainy rainy user list
```

## 用户文档

[用户指南](../user/index.md)涵盖入门、曲库与扫描、播放与 PWA、收听报表与 Scrobble、曲库管理和 Subsonic 客户端。[运维文档](../operations/index.md)涵盖 Docker 与 NAS 部署、配置、反向代理、数据库与备份以及故障排查。

## 文档

| 目标 | 从这里开始 |
| --- | --- |
| 使用 Rainy | [用户指南](../user/index.md) |
| 在 NAS 上部署 | [Docker 与 NAS 指南](../operations/docker.md) |
| 配置与运维 | [配置](../operations/configuration.md) · [反向代理](../operations/reverse-proxy.md) · [数据库](../operations/database.md) |
| 排查问题 | [常见问题](../operations/troubleshooting.md) |
| 了解架构 | [架构](../architecture/index.md) |
| 全部文档 | [文档索引](../README.md) |

完整文档目前为英文。

## 安全与隐私

曲库数据和音频都需要登录才能访问。Rainy 不收集任何遥测数据，除非管理员开启可选的在线元数据搜索、YouTube 和哔哩哔哩链接下载、在线音乐或同步到 Last.fm 和 ListenBrainz，服务器本身不会向外发起网络请求。漏洞请通过 [SECURITY.md](../../SECURITY.md) 私下报告；分享日志前请阅读 [PRIVACY.md](../../PRIVACY.md)。

## 开发与贡献

开发环境、测试、迁移和发布流程见[本地开发](../development/local-dev.md)和[测试](../development/testing.md)。贡献前请阅读 [CONTRIBUTING.md](../../CONTRIBUTING.md) 和 [AGENTS.md](../../AGENTS.md)。

## 致谢

Rainy 基于 TagLib（通过编译为 WebAssembly 的 go-taglib）、FFmpeg、SQLite（通过 modernc.org/sqlite），以及开启后使用的 yt-dlp。Subsonic 与 OpenSubsonic API 让它可以直接配合你已在用的客户端；[Navidrome](https://www.navidrome.org/) 则让人看到自托管音乐服务器可以多么好用。

## 许可证

Rainy 采用 [GNU Affero General Public License v3.0](../../LICENSE)。
