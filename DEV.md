# BingPaperDesktop 开发文档

[English](./DEV_EN.md) | 简体中文  [返回 README](./README.md)

本文档面向开发者，介绍项目工程结构、模块划分、前后端构建流程以及 CI/CD 配置，帮助你快速参与项目开发。

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 桌面框架 | [Wails v2](https://wails.io/)（Go + WebView） |
| 后端语言 | Go 1.23 |
| 前端框架 | React 18 + TypeScript |
| 前端构建 | Vite |
| UI 样式 | Tailwind CSS + shadcn/ui |
| 日志 | slog + lumberjack（日志轮转） |
| 系统托盘 | energye/systray |

---

## 工程目录结构

```
BingPaperDesktop/
 main.go                  # 程序入口：初始化 store、logger、Wails 应用及托盘
 go.mod / go.sum          # Go 模块依赖
 wails.json               # Wails 项目配置（应用名、Bundle ID 等）

 internal/                # 所有后端业务逻辑
    app/
       app.go           # 核心应用逻辑：API 暴露给前端的方法（FetchToday、ApplyWallpaper 等）
       tray.go          # 系统托盘菜单初始化
    bing/
       client.go        # 必应 API 客户端：获取元数据、选择变体、下载图片
    overlay/
       watermark.go     # 纯 Go 水印叠加实现（后备方案，主要叠加由前端 Canvas 渲染）
    scheduler/
       scheduler.go     # 调度器：支持 off/daily/interval/wakeup 四种模式
    store/
       config.go        # 配置文件读写、默认值、路径管理（便携/标准/自定义模式）
       holiday.go       # 节假日数据下载与缓存
       index.go         # 壁纸历史索引（CRUD）
    util/
       util.go          # 通用工具函数（重试、日期规范化等）
       util_windows.go  # Windows 平台特定实现（开机自启、Dock 等）
       util_darwin.go   # macOS 平台特定实现
       util_others.go   # 其他平台实现
       image.go         # 图片处理辅助（缩放等）
       wake_listener.go # 系统唤醒事件监听
    wallpaper/
        wallpaper.go         # 壁纸设置接口定义
        wallpaper_windows.go # Windows 实现（调用 SystemParametersInfo）
        wallpaper_darwin.go  # macOS 实现
        wallpaper_linux.go   # Linux/GNOME 实现（gsettings）

 frontend/                # 前端源码（React + TypeScript）
    index.html
    vite.config.ts       # Vite 构建配置
    tailwind.config.js   # Tailwind CSS 配置
    package.json
    src/
        main.tsx         # React 应用入口
        App.tsx          # 根组件：整体布局、全局状态管理
        components/
           WallpaperInfo.tsx    # 壁纸信息展示面板
           HistoryDrawer.tsx    # 壁纸历史抽屉
           HistoryItemCard.tsx  # 历史记录卡片
           QuickToggles.tsx     # 快速操作按钮组
           SettingsDialog.tsx   # 设置对话框
           ResetDialog.tsx      # 重置确认对话框
           ui/                  # shadcn/ui 基础组件（button、dialog 等）
        lib/
           utils.ts      # 前端工具函数（cn 等）
           watermark.ts  # 前端 Canvas 水印渲染逻辑
        types/
            lunar-javascript.d.ts  # 农历库类型声明

 wailsjs/                 # Wails 自动生成的 JS/TS 绑定（勿手动修改）
    go/
       models.ts        # Go 结构体对应的 TypeScript 类型
       app/
           App.d.ts     # 前端可调用的 Go 方法类型声明
           App.js       # 实际调用实现（通过 Wails IPC）
    runtime/             # Wails 运行时 JS 库

 build/                   # 平台构建资源
    bin/                 # 构建产物输出目录
    windows/             # Windows 图标、清单文件
    darwin/              # macOS 签名、公证配置文件

 script/                  # 快速构建脚本
     build.sh             # Linux/macOS 构建脚本
     build.ps1            # Windows PowerShell 构建脚本
     publish_tag.sh       # 发布 Tag 脚本（Linux/macOS）
     publish_tag.ps1      # 发布 Tag 脚本（Windows）
```

---

## 模块详细说明

### `internal/app/app.go`  核心应用层

这是暴露给前端的核心 API 层。所有在 `wailsjs/go/app/App.d.ts` 中列出的方法均在此实现。主要包括：

- `FetchToday(screenW, screenH int, dpr float64)`  获取今日壁纸的完整流程（拉元数据  选变体  下载  生成水印  存历史  可选应用）
- `ApplyHistoryToMonitor(key string, monitorIdx, screenW, screenH int)`  将指定历史图片设为壁纸
- `GetConfig() / SaveConfig()`  配置读写
- `ListHistory() / DeleteHistory()`  历史记录管理
- `GetVersionInfo()`  获取版本信息

### `internal/bing/client.go`  API 客户端

负责所有 HTTP 请求逻辑：

- 支持两种 API 格式：**必应官方** JSON 格式和 **BingPaper** 自定义格式
- `FetchMeta(apiType, url)`  获取元数据，统一转换为内部 `Meta` 结构体
- `SelectVariant(meta, screenW, screenH, forceUHD)`  根据屏幕分辨率和宽高比智能选择最佳图片变体
- `DownloadImage(url, destPath)`  带重试机制的图片下载

### `internal/scheduler/scheduler.go`  调度器

实现四种调度模式：

| 模式 | 实现方式 |
|------|----------|
| `off` | 不启动 goroutine |
| `daily` | 计算到指定时间的间隔，用 `time.Sleep` 等待 |
| `interval` | 每隔 N 分钟用 `time.Ticker` 触发 |
| `wakeup` | 通过 `wake_listener.go` 注册系统唤醒事件回调 |

### `internal/store/`  数据持久化

- **`config.go`**：配置 JSON 的读写，包含自动迁移旧配置字段的逻辑；路径策略：便携标准自定义
- **`index.go`**：`index.json` 维护壁纸历史列表，支持去重、按日期排序、按天数清理
- **`holiday.go`**：从 GitHub 下载当年节假日 JSON 数据，缓存到本地

### `internal/wallpaper/`  壁纸设置

跨平台壁纸设置，通过 Go 的构建标签（build tags）在编译时选择平台实现：

- **Windows**：通过 `win32.SystemParametersInfo(SPI_SETDESKWALLPAPER)` 设置
- **macOS**：通过 AppleScript 调用 `System Events` 设置
- **Linux**：通过 `gsettings set org.gnome.desktop.background picture-uri` 设置

### `frontend/src/lib/watermark.ts`  前端水印渲染

水印叠加的主要渲染逻辑运行在前端 Canvas：

1. 将原图加载到 `<canvas>`
2. 在 Canvas 上绘制文字（标题、版权、日期）和可选的农历/节假日日历信息
3. 将 Canvas 导出为 Base64 PNG，通过 Wails IPC 传给 Go 保存为文件

---

## 开发环境搭建

### 前提条件

| 工具 | 最低版本 | 用途 |
|------|----------|------|
| [Go](https://go.dev/) | 1.23 | 后端语言运行时 |
| [Node.js](https://nodejs.org/) | 20 | 前端构建 |
| [Wails CLI](https://wails.io/docs/gettingstarted/installation) | v2 | 项目构建与开发服务器 |

安装 Wails CLI：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 克隆与运行

```bash
git clone https://github.com/hanxuanyu/BingPaperDesktop.git
cd BingPaperDesktop

# 启动开发模式（热重载前端，后端改动需重启）
wails dev
```

开发模式下，Wails 会自动：
- 在后端监听文件变化并重新编译 Go
- 启动 Vite 开发服务器用于前端热更新
- 在 WebView 窗口中展示前端

---

## 生产构建

### 使用快速脚本

```bash
# Linux / macOS
./script/build.sh

# Windows (PowerShell)
.\script\build.ps1
```

### 手动构建

```bash
# 基础构建（无额外 flags）
wails build

# 指定版本信息（ldflags 注入）
wails build -ldflags "-X BingPaperDesktop/internal/app.Version=1.2.3 \
  -X BingPaperDesktop/internal/app.CommitHash=$(git rev-parse --short HEAD) \
  -X BingPaperDesktop/internal/app.BuildTime=$(date -u +%FT%TZ)"
```

构建产物默认输出到 `build/bin/` 目录。

### 前端单独构建

如需单独调试前端：

```bash
cd frontend
npm install
npm run build   # 生产构建
npm run dev     # 独立 Vite 开发服务器（无 Go 后端）
```

---

## Wails 前后端通信

Wails 通过代码生成实现前后端 IPC 通信：

1. 在 `internal/app/app.go` 中定义公开方法（首字母大写）
2. 运行 `wails dev` 或 `wails generate module` 时，Wails 会自动在 `wailsjs/go/app/` 目录生成对应的 TypeScript 声明文件和 JS 调用封装
3. **不要手动修改** `wailsjs/` 目录下的文件

事件通信（Go  前端）使用 `runtime.EventsEmit(ctx, "event-name", payload)` 推送，前端通过 `EventsOn("event-name", callback)` 监听。

---

## CI/CD 发布配置

本项目使用 GitHub Actions 自动构建、签名与发布。工作流文件位于 `.github/workflows/`。

### 触发条件

- **verify.yml**：每次 Push 和 PR 触发，执行编译验证（不生成产物）
- **release.yml**：推送形如 `v*.*.*` 的 Tag 时触发，生成并发布各平台安装包

### 发布前准备（Secrets 配置）

在 GitHub 项目的 **Settings  Secrets and variables  Actions** 中配置：

#### 必填  作者信息

| Secret | 说明 |
|--------|------|
| `AUTHOR_NAME` | 注入到 `wails.json` 的作者名 |
| `AUTHOR_EMAIL` | 注入到 `wails.json` 的作者邮箱 |

#### 必填  macOS 签名与公证

| Secret | 说明 |
|--------|------|
| `APPLE_ID_USER` | Apple Developer 账号邮箱 |
| `APPLE_PASSWORD` | App 专用密码（appleid.apple.com 生成） |
| `APPLE_PROVIDER` | Team ID（Apple Developer Portal 查看） |
| `APPLE_IDENTITY` | 证书标识符，如 `Developer ID Application: Your Name (TEAMID)` |
| `APPLE_DEVELOPER_CERTIFICATE_P12_BASE64` | Base64 编码的 `.p12` 证书 |
| `APPLE_DEVELOPER_CERTIFICATE_PASSWORD` | `.p12` 证书密码 |

#### 可选  macOS 自定义 Bundle ID

| Secret | 默认值 |
|--------|--------|
| `APPLE_BUNDLE_ID` | `com.hxuanyu.BingPaperDesktop` |

#### 可选  Windows 代码签名

| Secret | 说明 |
|--------|------|
| `WIN_SIGNING_CERT` | Base64 编码的 PFX 证书 |
| `WIN_SIGNING_CERT_PASSWORD` | PFX 证书密码 |

### 发布新版本

```bash
# 创建并推送 Tag 即可触发发布工作流
git tag v1.2.3
git push origin v1.2.3

# 或使用项目脚本（会自动更新 wails.json 中的版本号）
.\script\publish_tag.ps1   # Windows
./script/publish_tag.sh    # Linux/macOS
```

---

## 贡献指南

1. Fork 本仓库，在自己的分支上开发
2. 遵循现有代码风格（Go 使用标准格式，前端使用 Prettier）
3. 新功能请附带简要说明，提交 Pull Request 时描述改动内容
4. 如发现 Bug，请在 [Issues](https://github.com/hanxuanyu/BingPaperDesktop/issues) 中提交，附上复现步骤和日志（日志位于数据目录的 `logs/app.log`）

---

## 开源协议

本项目代码采用 [MIT License](./LICENSE) 开源。
