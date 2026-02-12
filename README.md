# BingPaperDesktop

基于 Wails v2 + Go + React 开发的跨平台 Bing 每日壁纸桌面客户端。

## 功能特性

- **多 API 支持**：支持必应官方 API 和 BingPaper 自定义 API，可根据需求灵活选择。
- **自动获取**：每日自动从 Bing 获取高清壁纸。
- **智能存储**：默认使用便携模式（数据存放在应用同级目录）；若当前目录不可写（如 macOS 的 `.app` 包内或只读目录），则自动切换至系统用户配置目录。
- **壁纸历史**：支持浏览历史壁纸、设为当前、删除及清空。
- **水印叠加**：可选在壁纸上叠加图片元数据（标题、日期、版权）。
- **定时任务**：支持按每日固定时间或固定间隔自动获取。
- **跨平台支持**：支持 Windows、macOS 及 Linux (GNOME)。

## 存储结构

应用支持两种存储模式：

1. **便携模式 (Portable)**：当应用所在目录可写时，配置文件 (`config.json`)、数据目录 (`data/`) 和日志 (`logs/`) 均存放在应用同级目录。
2. **标准模式**：当应用在只读目录运行（如 macOS 的 `.app` 包内或 Linux 的 `/usr/bin`）时，数据将存放在系统用户配置目录：
   - **macOS**: `~/Library/Application Support/BingPaperDesktop/`
   - **Windows**: `%AppData%\BingPaperDesktop\`
   - **Linux**: `~/.config/BingPaperDesktop/`

## 开发与构建

### 前提条件

- Go 1.22+
- Node.js & npm
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

### 运行开发模式

```bash
wails dev
```

### 构建正式版本

```bash
wails build
```

## Linux 支持说明

目前 Linux 端仅针对 GNOME 桌面环境（使用 `gsettings`）进行了测试与支持。其他桌面环境将提示“不支持”并提供打开数据目录的功能。

## 注意事项 (macOS)

在 macOS 上，直接从 DMG 运行或将 `.app` 放入 `/Applications` 时，应用会自动切换到“标准模式”，将数据保存在用户支持目录。这解决了因 `.app` 包内部只读而导致的闪退问题。

## 问题排查

如果壁纸未能成功设置，请查看 `logs/app.log` 获取详细错误堆栈。常见原因包括 API 地址失效、网络超时或系统权限受限。
