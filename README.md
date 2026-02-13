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


### 📄 开源协议

本项目采用 [MIT License](./LICENSE) 开源。
