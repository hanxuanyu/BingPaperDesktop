# BingPaperDesktop

基于 Wails v2 + Go + React 开发的跨平台 Bing 每日壁纸桌面客户端。

## 功能特性

- **自动获取**：每日自动从 Bing 获取高清壁纸。
- **便携存储**：配置文件、图片数据、日志均存放在应用同级目录，绿色便携。
- **壁纸历史**：支持浏览历史壁纸、设为当前、删除及清空。
- **水印叠加**：可选在壁纸上叠加图片元数据（标题、日期、版权）。
- **定时任务**：支持按每日固定时间或固定间隔自动获取。
- **跨平台支持**：支持 Windows、macOS 及 Linux (GNOME)。

## 存储结构 (Portable Mode)

- `config.json`：应用配置。
- `data/`：壁纸存储目录，按日期分文件夹。
- `data/index.json`：历史记录索引。
- `logs/app.log`：运行日志。

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

在 macOS 上，如果将 `.app` 放入只读目录（如某些 DMG 挂载点），应用将因无法写入配置和日志而报错。请确保应用运行在可写目录下。

## 问题排查

如果壁纸未能成功设置，请查看 `logs/app.log` 获取详细错误堆栈。常见原因包括 API 地址失效、网络超时或系统权限受限。
