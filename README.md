# BingPaperDesktop

[![Build Verification](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/verify.yml/badge.svg)](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/verify.yml)
[![Release](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/release.yml/badge.svg)](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/release.yml)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/hanxuanyu/BingPaperDesktop)](https://github.com/hanxuanyu/BingPaperDesktop/releases)

[English](./README_EN.md) | 简体中文

基于 Wails v2 + Go + React 开发的跨平台 Bing 每日壁纸桌面客户端。它能够自动获取 Bing 每日高清壁纸，并支持壁纸预览、历史记录管理、自动更新以及水印叠加等功能。

### 🚀 功能特性

- **多 API 模式**：支持 Bing 官方接口及自定义 API。
- **自动壁纸更新**：支持定时任务（固定时间或固定间隔）自动更换桌面壁纸。
- **壁纸历史管理**：查看历史壁纸，一键设为当前或删除。
- **智能存储管理**：
    - **便携模式**：优先在应用同级目录存储数据。
    - **标准模式**：当应用目录不可写（如 macOS .app 内）时，自动切换到用户配置目录。
- **水印叠加**：可选在壁纸上叠加标题、版权、日期等元数据。
- **跨平台支持**：完美支持 Windows (10/11)、macOS 以及 Linux (GNOME)。

### 📦 安装与下载

前往 [Releases](https://github.com/hanxuanyu/BingPaperDesktop/releases) 页面下载对应平台的安装包：

- **Windows**: 绿色单文件版（`.exe`）。
- **macOS**: `.dmg` 磁盘镜像，拖拽即可安装。
- **Linux**: 直接运行的二进制文件。

### 🛠️ 本地开发

#### 前提条件
- [Go](https://go.dev/) (1.23+)
- [Node.js](https://nodejs.org/) (20+)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

#### 运行步骤
1. 克隆仓库：
   ```bash
   git clone https://github.com/hanxuanyu/BingPaperDesktop.git
   cd BingPaperDesktop
   ```

> **提示**：如果命令行推送时提示需要密码，建议[配置 SSH](https://docs.github.com/en/authentication/connecting-to-github-with-ssh) 或使用 [GitHub CLI](https://cli.github.com/) 授权。

2. 运行开发模式：
   ```bash
   wails dev
   ```

### 🏗️ 项目构建

你可以使用 `script` 目录下的脚本进行快速构建：

- **macOS / Linux**:
  ```bash
  ./script/build.sh
  ```
- **Windows (PowerShell)**:
  ```powershell
  .\script\build.ps1
  ```

构建产物将存放在 `build/bin` 目录下。

### 🔐 CI/CD 发布配置

本项目使用 GitHub Actions 自动进行构建、签名和发布。为了使 `release` 工作流正常运行，你需要在 GitHub 项目的 **Settings > Secrets and variables > Actions** 中配置以下 Secrets：

#### macOS 签名与公证 (必填)

| Secret 名称 | 说明 | 获取方式 |
| :--- | :--- | :--- |
| `APPLE_ID_USER` | **Apple ID 账号 (邮箱)** | 你的 Apple Developer 账号邮箱。 |
| `APPLE_PASSWORD` | **App 专用密码** | 登录 [appleid.apple.com](https://appleid.apple.com/)，在“登录和安全”中生成。 |
| `APPLE_PROVIDER` | **团队 ID (Team ID)** | 登录 [Apple Developer Portal](https://developer.apple.com/account/) 查看。 |
| `APPLE_IDENTITY` | **证书标识符** | 运行 `security find-identity -v -p codesigning` 获取，如 `Developer ID Application: Your Name (ABC123XYZ)`。 |
| `APPLE_DEVELOPER_CERTIFICATE_P12_BASE64` | **Base64 编码的 .p12 证书** | 导出证书并运行 `base64 -i cert.p12 \| pbcopy`。 |
| `APPLE_DEVELOPER_CERTIFICATE_PASSWORD` | **.p12 证书导出密码** | 导出证书时设置的密码。 |
| `AUTHOR_NAME` | **作者名称** | 用于替换 `wails.json` 中的作者名。 |
| `AUTHOR_EMAIL` | **作者邮箱** | 用于替换 `wails.json` 中的作者邮箱。 |

#### macOS 签名与公证 (可选)

| Secret 名称 | 说明 | 默认值 |
| :--- | :--- | :--- |
| `APPLE_BUNDLE_ID` | **应用包名 (Bundle ID)** | `com.wails.BingPaperDesktop` |

#### Windows 签名 (可选)

| Secret 名称 | 说明 | 获取方式 |
| :--- | :--- | :--- |
| `WIN_SIGNING_CERT` | **Base64 编码的 PFX 证书** | 运行 `certutil -encode .\cert.pfx cert.txt` 并复制内容。 |
| `WIN_SIGNING_CERT_PASSWORD` | **证书解密密码** | 证书的密码。 |

### 📄 开源协议

本项目采用 [MIT License](./LICENSE) 开源。
