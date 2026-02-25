# BingPaperDesktop

[![Build Verification](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/verify.yml/badge.svg)](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/verify.yml)
[![Release](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/release.yml/badge.svg)](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/release.yml)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/hanxuanyu/BingPaperDesktop)](https://github.com/hanxuanyu/BingPaperDesktop/releases)

[English](./README_EN.md) | 简体中文  [开发文档 ](./DEV.md)

> 基于 Wails v2 + Go + React 开发的跨平台 Bing 每日壁纸桌面客户端。自动获取必应每日高清壁纸，让你的桌面每天焕然一新。

---

## 功能特性

###  双 API 模式

应用支持两种图片数据来源，可在设置中随时切换：

| 模式 | 接口地址 | 说明 |
|------|----------|------|
| **必应官方** | `https://www.bing.com/HPImageArchive.aspx?format=js&idx=0&n=1&uhd=1&mkt=zh-CN` | 直接从必应官方接口获取今日壁纸元数据，稳定可靠 |
| **BingPaper 接口** | `https://bing.coding.icu/api/v1/image/today/meta`（默认） | 由开源项目 [BingPaper](https://github.com/hanxuanyu/BingPaper) 提供的服务端 API，每天自动抓取必应每日壁纸并以标准化格式对外提供，支持多分辨率变体 |

> **关于 BingPaper 服务端**：[hanxuanyu/BingPaper](https://github.com/hanxuanyu/BingPaper) 是一个独立的开源后端项目，专注于每日自动抓取必应壁纸并提供 API 服务。你可以自行部署该项目，然后在本应用的设置中填写自己的 API 地址，实现完全自主可控的图片数据来源。

---

###  灵活的自动更新策略

在「设置  调度模式」中可选择以下四种更新方式：

| 模式 | 说明 |
|------|------|
| **关闭** | 不自动更新，仅手动刷新 |
| **每日定时** | 在指定时间（如 `08:30`）自动获取并应用今日壁纸 |
| **固定间隔** | 每隔指定分钟数自动刷新一次 |
| **唤醒触发** | 系统从睡眠或休眠状态唤醒后自动刷新壁纸 |

---

###  智能分辨率选择

- 自动检测当前屏幕分辨率和 DPR（设备像素比），选择最匹配的图片变体
- 支持**强制 UHD 模式**，优先获取超高清（38402160）原图
- 提供 16:9 和 4:3 两种宽高比的适配版本

---

###  壁纸历史管理

- 所有已下载的壁纸自动保存到本地历史记录
- 可在历史抽屉中浏览缩略图
- 一键将任意历史图片设为当前壁纸
- **随机历史模式**：每次自动更新时从历史记录中随机选取一张应用
- 支持按保留天数自动清理过期历史（设为 `0` 则永久保留）

---

###  水印与日历叠加

- **元数据水印**：可选在壁纸上叠加图片标题、版权信息、拍摄日期等
- **日历叠加**：可选在壁纸上显示当日农历日期及节假日信息（需联网下载节假日数据）
- 水印叠加图以 PNG 格式独立保存，不修改原始图片文件

---

###  灵活的数据存储

| 场景 | 存储位置 |
|------|----------|
| **便携模式**（默认） | 与可执行文件同级目录，适合 U 盘携带 |
| **标准模式** | 程序目录不可写时（如安装在 `C:\Program Files`）自动切换到用户配置目录 |
| **macOS** | 默认存储在 `~/Library/Application Support/BingPaperDesktop/` |
| **自定义目录** | 可在设置界面手动指定任意数据目录 |

数据目录结构：

```
<数据目录>/
 config.json           # 配置文件
 data/                 # 壁纸原图及元数据（按日期分类）
    YYYY-MM-DD/
        original.jpg
        watermark_16_9.png
        watermark_4_3.png
        meta.json
 thumbnails/           # 历史缩略图
 logs/                 # 应用日志
```

---

###  系统托盘集成

- 最小化到系统托盘，不占用任务栏
- 托盘菜单支持：**显示界面** / **立即刷新壁纸** / **退出程序**
- macOS 支持隐藏 Dock 图标，以纯托盘模式运行

---

###  其他实用功能

| 功能 | 说明 |
|------|------|
| **开机自启** | 可选随系统启动自动运行 |
| **日志管理** | 自动轮转日志文件，可配置最大大小和保留天数 |
| **节假日数据** | 自动从网络下载/更新当年节假日数据，用于日历叠加 |
| **快速操作** | 主界面提供一键刷新、立即应用等快捷按钮 |
| **壁纸信息展示** | 主界面展示当前壁纸的标题、版权等元数据 |

---

##  下载与安装

前往 [Releases 页面](https://github.com/hanxuanyu/BingPaperDesktop/releases) 下载对应平台的安装包：

| 平台 | 文件格式 | 安装方式 |
|------|----------|----------|
| **Windows 10/11** | `.exe` 绿色单文件 | 直接运行，无需安装 |
| **macOS** | `.dmg` 磁盘镜像 | 挂载后拖拽到 Applications 文件夹 |
| **Linux (GNOME)** | 二进制文件 | 赋予执行权限后直接运行 |

> **macOS 首次运行提示**：如果系统提示「无法验证开发者」，请前往**系统设置  隐私与安全性**允许运行，或在终端执行 `xattr -cr /Applications/BingPaperDesktop.app`。

---

## 高级用法

### 命令行参数

```bash
# 指定自定义数据目录
./BingPaperDesktop --data-path /path/to/data
```

### 环境变量

```bash
# 通过环境变量指定数据目录
export BING_PAPER_DATA_PATH=/path/to/data
./BingPaperDesktop
```

---

## 自行部署 BingPaper API

如果你希望使用自己搭建的图片 API，可以部署开源项目 [hanxuanyu/BingPaper](https://github.com/hanxuanyu/BingPaper)。

部署完成后，在本应用的设置中：

1. 将 **API 模式** 切换为 **BingPaper（自定义）**
2. 将 **自定义 API 地址** 修改为你部署的服务地址，例如：
   ```
   https://your-domain.com/api/v1/image/today/meta
   ```

---

## 参与开发

如果你对项目技术实现感兴趣，或希望参与贡献，请查阅 [开发文档 DEV.md](./DEV.md)，其中包含：

- 工程目录结构与模块说明
- 前后端构建流程
- CI/CD 发布配置

---

##  版权声明

- 本应用展示的所有壁纸图片**版权归微软必应（Microsoft Bing）所有**，图片元数据中包含版权归属信息（`copyright` 字段）。
- 本应用仅通过公开 API 获取图片地址供个人桌面使用，**不对图片进行再分发或商业使用**。
- 显示在壁纸上的版权文字（如 ` Getty Images`）均来自必应官方元数据，本应用如实呈现以尊重原始版权。
- 如需将壁纸用于商业或其他用途，请务必查看图片的版权信息并自行取得相应授权。

---

## 开源协议

本项目代码采用 [MIT License](./LICENSE) 开源。
