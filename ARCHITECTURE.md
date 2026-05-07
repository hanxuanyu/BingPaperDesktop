# BingPaperDesktop — 架构与实现原理

> 本文档由代码 review 整理而成，面向希望快速理解项目实现原理的开发者。  
> 技术栈：**Wails v2 + Go 1.26.1 + React 18 + TypeScript + Tailwind CSS**

---

## 目录

1. [整体架构](#整体架构)
2. [进程模型与通信机制](#进程模型与通信机制)
3. [后端模块详解](#后端模块详解)
   - [main.go — 程序入口](#maingo--程序入口)
   - [internal/app — 核心应用层](#internalapp--核心应用层)
   - [internal/bing — API 客户端](#internalbing--api-客户端)
   - [internal/scheduler — 调度器](#internalscheduler--调度器)
   - [internal/store — 数据持久化](#internalstore--数据持久化)
   - [internal/overlay — 图像合成](#internaloverlay--图像合成)
   - [internal/wallpaper — 跨平台设置](#internalwallpaper--跨平台设置)
   - [internal/util — 工具函数](#internalutil--工具函数)
4. [前端模块详解](#前端模块详解)
   - [App.tsx — 全局状态与事件中心](#apptsx--全局状态与事件中心)
   - [watermark.ts — Canvas 水印渲染](#watermarkts--canvas-水印渲染)
   - [组件层级](#组件层级)
5. [核心业务流程](#核心业务流程)
   - [获取今日壁纸（FetchToday）](#获取今日壁纸fetchtoday)
   - [水印叠加渲染流程](#水印叠加渲染流程)
   - [壁纸应用流程（ApplyHistoryToMonitor）](#壁纸应用流程applyhistorytomonitor)
   - [配置变更联动](#配置变更联动)
6. [数据存储结构](#数据存储结构)
7. [调度器状态机](#调度器状态机)
8. [分辨率选择算法](#分辨率选择算法)
9. [跨平台差异矩阵](#跨平台差异矩阵)

---

## 整体架构

```
┌──────────────────────────────────────────────────────────────┐
│                       BingPaperDesktop                        │
│                                                              │
│  ┌───────────────────┐        ┌──────────────────────────┐   │
│  │   前端 (WebView)   │ ◄────► │      后端 (Go Process)    │   │
│  │                   │  IPC   │                          │   │
│  │  React 18 + TS    │        │  Wails v2 Runtime        │   │
│  │  Tailwind CSS     │        │  ┌─────────────────────┐ │   │
│  │  Canvas API       │        │  │  app.App  (核心层)   │ │   │
│  │  watermark.ts     │        │  │  bing.Client         │ │   │
│  │                   │        │  │  scheduler           │ │   │
│  └───────────────────┘        │  │  store (config/idx)  │ │   │
│                               │  │  overlay (合成)      │ │   │
│  ┌───────────────────┐        │  │  wallpaper (设置)    │ │   │
│  │   系统托盘         │        │  └─────────────────────┘ │   │
│  │ energyde/systray  │◄──────►│                          │   │
│  └───────────────────┘        └──────────────────────────┘   │
│                                           │                   │
│                               ┌───────────▼─────────────┐    │
│                               │   文件系统 (本地存储)     │    │
│                               │  config.json / index.json│    │
│                               │  data/YYYY-MM-DD/        │    │
│                               │  thumbnails/             │    │
│                               │  logs/                   │    │
│                               └──────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
         │                                    │
         ▼                                    ▼
  Bing 官方 API                    BingPaper 自定义 API
  bing.com/HPImageArchive       bing.coding.icu/api/v1/...
```

---

## 进程模型与通信机制

Wails v2 将 Go 后端和 WebView 前端运行在**同一个进程**中，通过内部 IPC（进程内通信）实现双向调用。

```
┌────────────────────────────────────────────────────┐
│                    同一个 OS 进程                    │
│                                                    │
│  Go Runtime                WebView2 / WKWebView    │
│  ┌──────────────┐          ┌─────────────────────┐ │
│  │  app.App     │          │  React App           │ │
│  │  (struct)    │◄─ RPC ──►│  wailsjs/go/app/App │ │
│  └──────────────┘          └─────────────────────┘ │
│                                                    │
│  Go → 前端:  runtime.EventsEmit(ctx, event, data)  │
│  前端 → Go:  直接调用 wailsjs 的 JS 绑定函数         │
└────────────────────────────────────────────────────┘
```

**事件列表（Go → 前端）：**

| 事件名 | 触发时机 | 数据结构 |
|--------|---------|---------|
| `current-image-changed` | 壁纸应用成功后 | `store.HistoryItem` |
| `render-watermark` | 需要前端渲染叠加层时 | `app.OverlayRequest` |
| `prepare-show-window` | 窗口即将显示/关闭时 | 无 |

---

## 后端模块详解

### main.go — 程序入口

**职责**：初始化顺序管理、Wails 应用配置、日志系统。

```
main() 启动流程：
 1. flag.Parse()          —— 解析 -data-path 命令行参数
 2. store.Init()          —— 确定并创建数据目录（便携/标准/自定义三种策略）
 3. initLogger()          —— 配置 slog + lumberjack 日志轮转
 4. app.NewApp()          —— 创建 App 实例（含调度器）
 5. app.SetupTray()       —— 注册系统托盘（返回平台相关的 start/end 函数）
 6. wails.Run()           —— 启动事件循环
    ├── OnStartup         → appInstance.Startup(ctx)
    ├── OnDomReady        → 隐藏 macOS Dock 图标（如配置）
    ├── OnBeforeClose     → 触发清除 toast 事件、隐藏 Dock 图标
    └── OnShutdown        → 停止托盘
```

**日志架构**：

```go
// 同时写入文件和 stdout
mw := io.MultiWriter(lumberjackWriter, os.Stdout)
slog.SetDefault(slog.New(slog.NewTextHandler(mw, &slog.HandlerOptions{Level: slog.LevelInfo})))
```

- 日志文件：`<baseDir>/logs/app.log`
- 轮转策略：最大 `LogMaxSize` MB，保留 3 个旧文件，按 `LogRetainDays` 天清理

---

### internal/app — 核心应用层

这是所有暴露给前端方法的实现层。文件分为两个：

#### app.go — 业务逻辑

**App 结构体**：

```
App
├── ctx       context.Context    —— Wails 上下文（用于事件发送和窗口控制）
├── sched     *scheduler.Scheduler
├── fetchMu   sync.Mutex         —— 防止并发 FetchToday 执行
├── mu        sync.RWMutex       —— 保护 lastFetch（读写锁）
├── lastFetch *CurrentResult     —— 当前已应用壁纸的缓存状态
├── wmChan    chan string (buf=1) —— 前端 Canvas 渲染结果传递通道
└── wmMu      sync.Mutex         —— 串行化叠加层渲染请求
```

> **wmChan 设计要点**：容量为 1 的缓冲 channel。Go 通过 `EventsEmit` 触发前端渲染后立即进入 `select` 等待，JS 回调异步完成后调用 `SubmitWatermark`。缓冲确保即便 JS 回调比 Go select 稍早执行，数据也不会丢失。`wmMu` 互斥锁确保同一时刻只有一个渲染请求在等待，避免 channel 竞争。

**暴露给前端的方法列表**：

| 方法 | 说明 |
|------|------|
| `FetchToday(w,h,dpr)` | 获取今日壁纸完整流程 |
| `ApplyHistoryToMonitor(key,monID,w,h)` | 将历史图应用到指定显示器 |
| `ApplyHistory(key,w,h)` | 应用到所有显示器（monID=-1） |
| `GetCurrentItem()` | 获取内存中的最新壁纸状态 |
| `ListHistory()` | 读取历史索引 |
| `DeleteHistory(key)` | 删除单条历史 |
| `ClearHistory()` | 清空所有历史 |
| `CleanupByRetainDays(days)` | 按保留天数清理历史 |
| `GetConfig()` | 读取配置 |
| `SaveConfig(cfg)` | 保存配置并联动调度器 |
| `GetMonitors()` | 获取显示器列表 |
| `GetThumbnailURL(rel)` | 获取（按需生成）缩略图 URL |
| `GetImageURL(rel)` | 获取图片 URL |
| `SubmitWatermark(b64)` | 接收前端渲染结果 |
| `GetVersionInfo()` | 版本信息 |
| `SetBaseDir(path)` | 切换数据目录 |
| `SelectDirectory()` | 打开目录选择对话框 |
| `OpenDataDir/BaseDir/LogsDir()` | 在资源管理器中打开目录 |
| `ResetSettings()` | 仅重置配置为默认 |
| `ResetApplication()` | 清空所有数据+配置 |
| `Quit()` | 退出程序 |

**AssetsHandler** — 本地图片 HTTP 服务：

```
请求 /images/<relPath>
  ↓
将 relPath 拼接到 baseDir
  ↓
安全检查（防路径穿越）
  ↓
响应文件（Cache-Control: public, max-age=31536000, immutable）
```

#### tray.go — 系统托盘

```
SetupTray() 逻辑：
  ├── macOS: systray.RunWithExternalLoop()（配合 Wails 事件循环）
  └── 其他: systray.Register()

托盘菜单项：
  ├── 显示界面   → EventsEmit("prepare-show-window") + WindowShow()
  ├── 立即刷新壁纸 → goroutine: FetchToday(0,0,1.0)
  ├── ─────────
  └── 退出程序    → runtime.Quit() / os.Exit(0)

单击行为（平台差异）：
  Windows: 左键 → 无操作；右键/双击 → 显示菜单/窗口
  macOS:   左键 → ShowMenu()；双击 → ShowWindow()
```

---

### internal/bing — API 客户端

```
bing/
└── client.go
    ├── Meta        (统一内部元数据结构)
    ├── Variant     (单张图片变体)
    ├── FetchMeta   (HTTP GET + 响应解析 + 格式统一化)
    ├── SelectVariant (分辨率选择算法)
    └── DownloadImage (带原子性的下载：.part 临时文件)
```

**两种 API 格式对比**：

| | 必应官方 (`bing`) | BingPaper 自定义 (`custom`) |
|--|--|--|
| 响应格式 | `{"images":[...]}` | 与内部 `Meta` 结构一致 |
| 变体信息 | 仅 UHD URL | 多分辨率 Variants 列表 |
| 使用场景 | 稳定，无需中间服务 | 更多变体选择 |

---

### internal/scheduler — 调度器

```
Scheduler
├── mode: "off" | "daily" | "interval" | "wakeup"
├── dailyTime: "HH:MM"
├── interval: time.Duration
├── stopChan: chan struct{}（用于停止 goroutine）
├── isRunning: bool
└── isExecuting: bool（防并发执行）
```

**状态转换图**：

```
          Update(mode)/Stop()
  ┌──────────────────────────────────┐
  ▼                                  │
STOPPED ──── Start() ──► RUNNING     │
  │             mode≠off             │
  │                                  │
  │         stop/Update              │
  └──────────────────────────────────┘

RUNNING goroutine 行为：
  "daily"    → calculateNextDaily() → time.Sleep → execute()
  "interval" → time.Ticker          → execute()
  "wakeup"   → 由 util.OnWake 回调触发 execute()（不进入 goroutine run()）
```

**防并发重入**：`isExecuting` 标志确保调度任务不会并发执行（例如定时触发和唤醒触发同时来到时）。

**每日时间计算**：

```go
next = today HH:MM
if next < now: next += 24h
sleep(next - now)
```

---

### internal/store — 数据持久化

#### config.go

```
baseDir 确定策略（优先级递减）：
  1. CLI 参数 -data-path
  2. 环境变量 BING_PAPER_DATA_PATH
  3. macOS → ~/Library/Application Support/BingPaperDesktop/
  4. Windows（可写） → 可执行文件所在目录
  5. Windows（不可写）→ %APPDATA%/BingPaperDesktop/
```

**配置迁移**（LoadConfig 内含）：读取配置后检测是否缺少新字段，若缺少则补全默认值并异步回写 config.json，实现平滑升级。

**config.json 完整字段**：

```json
{
  "api_type": "bing",               // "bing" | "custom"
  "bing_api_url": "...",            // 官方 API URL
  "custom_api_url": "...",          // 自定义 API URL
  "api_meta_url": "...",            // 已废弃，兼容旧版
  "auto_apply": true,               // 获取后自动设置壁纸
  "overlay_metadata": false,        // 启用元数据水印叠加
  "force_uhd": true,                // 强制选 UHD 分辨率
  "schedule_mode": "daily",         // 调度模式
  "daily_time": "08:30",            // 每日定时时间
  "interval_minutes": 60,           // 间隔分钟数
  "retain_days": 0,                 // 历史保留天数（0=永久）
  "random_history": false,          // 自动更新时随机历史
  "log_retain_days": 30,
  "log_max_size": 10,               // MB
  "auto_start": false,
  "hide_dock_icon": false,          // macOS 专用
  "enable_calendar": false,         // 日历叠加开关
  "enable_holiday": true,           // 节假日数据开关
  "holiday_api_url": "..."          // 节假日数据源 URL（含 yyyy 占位符）
}
```

#### index.go

**历史索引结构**：

```
data/index.json
{
  "items": [
    {
      "key": "2026-01-21_<hsh>",   // 唯一标识
      "date": "2026-01-21",
      "title": "...",
      "copyright": "...",
      "chosen_variant": "UHD",
      "image_path": "data/2026-01-21/original.jpg",
      "watermark_path": "",         // 已废弃字段（叠加层路径动态计算）
      "created_at": "2026-01-21T08:30:00Z"
    },
    ...
  ]
}
```

**主要操作**：
- `AddToHistory`：去重逻辑（按 key），新增后按 `CreatedAt` 降序排列
- `DeleteFromHistory`：删除条目 + 整个日期目录（`data/YYYY-MM-DD/`）+ 缩略图目录
- `CleanupByRetainDays`：按 `item.Date` 对比阈值日期删除，计数并返回已删除数

#### holiday.go

从 GitHub（NateScarlet/holiday-cn）下载当年节假日 JSON，缓存到 `data/holiday/<year>.json`。使用 30s 超时的 HTTP 客户端防止网络异常卡死。

---

### internal/overlay — 图像合成

```
overlay/watermark.go 提供三个函数：
  ├── SaveBase64Image(b64, dest)  —— 解码 data-URL 并写入文件（供 SubmitWatermark 调用）
  ├── Composite(bg, overlays, dest) —— 将多个 PNG 叠加层合成到背景图并输出 JPEG
  └── AddWatermark(...)  —— ⚠️ 纯 Go 水印实现（当前未被调用）
                             保留作为无前端环境下的备用方案
```

**合成流程**（`Composite`）：

```
读取背景 JPEG → 解码为 image.RGBA
  ↓
遍历 overlays（PNG 文件）
  ↓
draw.Draw(rgba, ..., ovImg, ..., draw.Over)  // Alpha 混合叠加
  ↓
编码为 JPEG（Quality=95）输出
```

---

### internal/wallpaper — 跨平台设置

通过 Go build tags 在编译期选择平台实现：

```
wallpaper/
├── wallpaper.go          —— 接口定义（GetMonitors / Set / SetOnMonitor / Supported）
├── wallpaper_windows.go  —— CoInitialize + IDesktopWallpaper COM 接口
│                             回退：SystemParametersInfo(SPI_SETDESKWALLPAPER)
├── wallpaper_darwin.go   —— AppleScript: tell System Events to set picture
└── wallpaper_linux.go    —— gsettings set org.gnome.desktop.background picture-uri
```

**多显示器支持（Windows）**：

```
CoCreateInstance(IDesktopWallpaper)
  ↓
IDesktopWallpaper.GetMonitorDevicePathAt(monitorIndex)
  ↓
IDesktopWallpaper.SetWallpaper(monitorPath, wallpaperPath)
  ↓
若 COM 方式不可用 → 回退到全局 SystemParametersInfo
```

---

### internal/util — 工具函数

| 文件 | 内容 |
|------|------|
| `util.go` | `Retry`（指数退避）、`NormalizeDate`（YYYYMMDD → YYYY-MM-DD）、`ResizeIcon`（图标缩放） |
| `image.go` | `GenerateThumbnail`（Lanczos 缩放，宽度限制为 400px） |
| `wake_listener.go` | 回调注册（`OnWake`）、触发（`TriggerWake`）的跨平台入口 |
| `util_windows.go` | 注册开机自启（注册表）、唤醒监听（Win32 消息循环 + WM_POWERBROADCAST）、`SetAutoStart`、`OpenFolder`、`OpenURL` |
| `util_darwin.go` | `HideDockIcon`/`ShowDockIcon`（ActivationPolicy）、macOS 唤醒监听 |
| `util_others.go` | Linux/其他平台的空实现 |

---

## 前端模块详解

### App.tsx — 全局状态与事件中心

**状态树**：

```typescript
App 组件状态
├── config: store.Config | null     —— 应用配置
├── currentImage: HistoryItem | null —— 当前显示/应用的壁纸
├── currentImageDataURL: string      —— 当前壁纸的 img src（来自 /images/ 路由）
├── prevImageDataURL: string         —— 上一张图片 URL（实现过渡动画）
├── history: HistoryItem[]           —— 历史记录列表
├── monitors: Monitor[]              —— 显示器列表
├── loading: boolean                 —— 全局加载状态
├── wallpaperSupport: any            —— 壁纸设置支持情况
├── platform: string                 —— 当前平台（"windows"/"darwin"/"linux"）
├── isSettingsOpen: boolean
└── isResetDialogOpen: boolean

Refs（不触发重渲染）：
├── isManualFetching: boolean  —— 区分手动/自动触发（避免重复 toast）
└── configRef: Config | null   —— 事件监听器中访问最新 config（闭包同步）
```

**初始化流程**（useEffect，仅执行一次）：

```
并行加载：loadConfig() + loadHistory() + loadMonitors() + fetchToday(silent) + GetWallpaperSupport()
  ↓
注册 Wails 事件监听：
  "current-image-changed" → setCurrentImage + 条件 loadHistory
  "prepare-show-window"   → toast.dismiss() + GetCurrentItem() 同步状态
  "render-watermark"      → renderWatermark() → SubmitWatermark()
  ↓
注册 window.focus 监听 → toast.dismiss()
```

**背景图切换动画**：

```
currentImage 变化
  ↓
GetImageURL(imageRelPath) → "/images/data/YYYY-MM-DD/original.jpg"
  ↓
new Image().decode()  —— 预加载确保无闪烁
  ↓
setPrevImageDataURL(旧 URL)  —— 保持旧背景可见（CSS transition）
setCurrentImageDataURL(新 URL)
  ↓
新图层 key={url} 触发重渲染，CSS `animate-in fade-in` 淡入 700ms
```

---

### watermark.ts — Canvas 水印渲染

水印渲染完全在浏览器端执行，利用 Canvas 2D API：

```
renderWatermark(data: OverlayRequest): Promise<string>
  ↓
创建 <canvas>，设置尺寸
  ├── 有 image_path → 加载图片 → img.decode() → 可选 drawImage
  └── 无 image_path → 纯参数驱动（仅日历叠加层）
  ↓
[enable_watermark=true] drawWatermark()
  ↓
[enable_calendar=true]  drawCalendar()
  ↓
canvas.toDataURL("image/png")  —— only_overlay=true 时导出透明 PNG
canvas.toDataURL("image/jpeg", 0.95) —— 完整合成时
```

**`calculateSafeArea` — 安全区域计算**：

壁纸可能因宽高比不匹配而被裁剪（`background-size: cover`）。此函数计算在目标宽高比下实际可见的区域，确保水印文字不落在被裁剪的区域外。

```
图片宽高比 > 目标宽高比 → 左右被裁 → visibleWidth = height * targetRatio
图片宽高比 < 目标宽高比 → 上下被裁 → visibleHeight = width / targetRatio

安全区 = 可见区域内缩 4%（边距）
```

**`drawWatermark` — 元数据水印**：

```
在安全区底部靠右绘制：
  ├── 渐变遮罩（黑色，底部30%高度，提升文字可读性）
  ├── 标题（bold，字号≥24px，自适应可见高度4.5%）
  ├── 版权信息（正常字重，字号≥14px）
  └── 标签组（日期/分辨率，带圆角半透明背景框）
```

**`drawCalendar` — 月历叠加层**：

```
位置：右上角，宽度320×scale，高度动态（按行数）
内容：
  ├── 半透明磨砂背景（圆角矩形）
  ├── 公历年月日（bold）
  ├── 农历年份（干支/生肖）+ 月日（lunar-javascript 库）
  ├── 星期标题行（日/一-六，周末红色）
  └── 日期网格（每格含公历数字 + 农历/节气/法定节日副文字 + 休/班角标）

节假日优先级：
  后端传入 holidayData → 默认周六日逻辑
  副文字优先级：法定节日名 > 节气 > 农历普通日 > 节假日名
```

---

### 组件层级

```
App
├── TooltipProvider
├── <背景图层> (prev + current，CSS fade-in 动画)
├── <渐变遮罩层>
├── QuickToggles          —— 左上角快捷开关（AutoApply / OverlayMetadata / EnableCalendar）
├── <右上角动作区>
│   ├── HistoryDrawer     —— 历史抽屉（Radix Sheet）
│   │   └── HistoryItemCard × N
│   └── SettingsDialog    —— 设置对话框（Radix Dialog）
├── WallpaperInfo         —— 中央信息面板（标题/版权/日期/操作按钮）
└── ResetDialog           —— 重置确认对话框
```

---

## 核心业务流程

### 获取今日壁纸（FetchToday）

```
FetchToday(screenW, screenH, dpr)
  │
  ├─ 1. fetchMu.Lock()  —— 防并发重入
  │
  ├─ 2. 计算物理像素：realW = screenW × dpr, realH = screenH × dpr
  │      ⚠️ 前端必须传入 CSS 逻辑像素（window.screen.width），
  │         不能传入已乘 DPR 的物理像素（否则 DPR 被应用两次）
  │
  ├─ 3. bing.FetchMeta(apiType, url)
  │      └─ HTTP GET（带3次重试）→ 解析 JSON → 返回 *Meta
  │
  ├─ 4. bing.SelectVariant(meta, realW, realH, forceUHD)
  │      └─ 选择最匹配的图片变体（见分辨率选择算法）
  │
  ├─ 5. 准备存储路径：data/YYYY-MM-DD/
  │      └─ migrateOldDataDir（旧格式 YYYYMMDD → YYYY-MM-DD 迁移）
  │
  ├─ 6. saveMetaJson(meta, dir)  —— 保存 meta.json
  │
  ├─ 7. bing.DownloadImage（若本地无 original.jpg）
  │
  ├─ 8. [cfg.OverlayMetadata] ensureWatermarkOverlay（16:9 和 4:3 各一张）
  │
  ├─ 9. store.AddToHistory(item)
  │
  └─ 10. 根据配置决定如何应用：
       ├─ [AutoApply && RandomHistory] → ApplyRandomHistory → ApplyHistoryToMonitor
       ├─ [AutoApply]                  → ApplyHistoryToMonitor(item.Key, -1, ...)
       └─ [非AutoApply]               → 更新 lastFetch + EventsEmit("current-image-changed")
```

---

### 水印叠加渲染流程

这是一个跨进程边界的异步协作流程，通过 Wails 事件和 `wmChan` 实现同步：

```
Go (ensureWatermarkOverlay)          前端 JS
       │                                 │
       │ 1. wmMu.Lock()                  │
       │ 2. EventsEmit("render-watermark", req) ──────────►│
       │ 3. select { case <-wmChan: ... }│                 │
       │    time.After(10s) 超时保护      │                 │
       │                                 │ 4. renderWatermark(req)
       │                                 │    - 加载图片到 canvas
       │                                 │    - drawWatermark()
       │                                 │    - drawCalendar()
       │                                 │    - canvas.toDataURL()
       │                                 │ 5. SubmitWatermark(base64)
       │◄── wmChan ← base64Data ─────────│
       │ 6. overlay.SaveBase64Image(dst) │
       │ 7. wmMu.Unlock()               │
```

> **关键设计**：`wmMu` 互斥锁确保同时只有一个渲染请求在飞行，`wmChan` 缓冲=1 确保前端回调即便在 Go select 就绪之前也能完成投递。

---

### 壁纸应用流程（ApplyHistoryToMonitor）

```
ApplyHistoryToMonitor(key, monitorID, screenW, screenH)
  │
  ├─ key=="" → FetchToday(screenW, screenH, 1.0) 获取 key
  │
  ├─ store.LoadIndex() → 查找 target HistoryItem
  │
  ├─ wallpaper.GetMonitors() 获取显示器列表
  │   └─ 失败时回退：[{ID:0, W:screenW, H:screenH}]
  │
  ├─ 过滤目标显示器（monitorID>=0 则取特定 ID，否则取全部）
  │
  └─ 遍历目标显示器：
       prepareWallpaperForMonitor(target, monitor, cfg)
         ├─ [无叠加] → 直接用 original 路径
         ├─ [有水印] → ensureWatermarkOverlay（缓存命中或触发渲染）
         ├─ [有日历] → getCalendarOverlay（按日期+尺寸缓存）
         └─ overlay.Composite(bg, overlays, current_wallpaper_<ID>_<ts>.jpg)
         │   清理该显示器旧的临时壁纸文件（防同路径不触发系统更新）
       wallpaper.SetOnMonitor(monitorID, path)
         └─ 失败 → 回退到 wallpaper.Set(path)（全局设置）
       │
       更新 lastFetch + EventsEmit("current-image-changed")
```

---

### 配置变更联动

```
前端调用 SaveConfig(newCfg)
  ↓
Go SaveConfig():
  ├─ AutoStart 变更   → util.SetAutoStart()
  ├─ HideDockIcon 变更 → util.HideDockIcon() / ShowDockIcon()
  ├─ store.SaveConfig() 写入 config.json
  ├─ EnableHoliday 变更/HolidayApiUrl 变更 → goroutine: CheckAndDownloadHoliday
  ├─ scheduler.Update() 重启调度器
  └─ logUpdateFunc(cfg) 更新日志轮转参数
```

---

## 数据存储结构

```
<baseDir>/                         # 数据根目录
  config.json                      # 应用配置
  current_wallpaper_<ID>_<ts>.jpg  # 临时合成壁纸（每次应用时更新）

  data/                            # 壁纸数据（按日期组织）
    index.json                     # 历史索引（所有 HistoryItem 列表）
    YYYY-MM-DD/
      original.jpg(或.png)         # 原始下载图（未修改）
      watermark_16_9.png           # 16:9 比例的元数据水印叠加层（透明 PNG）
      watermark_4_3.png            # 4:3 比例的元数据水印叠加层（透明 PNG）
      calendar_cache_WxHh?_16_9.png # 日历叠加层（按分辨率+是否含节假日缓存）
      meta.json                    # 壁纸元数据（标题/版权/变体等）
    holiday/
      2026.json                    # 节假日数据（从 GitHub 下载）

  thumbnails/                      # 等比缩略图（宽度限 400px）
    data/
      YYYY-MM-DD/
        original.jpg               # 对应原图的缩略图

  logs/
    app.log                        # 当前日志
    app.log.1 app.log.2 ...        # 按大小轮转的旧日志
```

---

## 调度器状态机

```
                ┌───────────────────────────────────┐
                │                                   │
                ▼                                   │
           ┌────────┐   Start() (mode≠off)   ┌─────────────┐
  New() ──►│ STOPPED│──────────────────────►│   RUNNING   │
           └────────┘                        └─────────────┘
                ▲                                   │
                │          Stop() / Update()        │
                └───────────────────────────────────┘
                         重新创建 stopChan

RUNNING 内部 goroutine:
  ┌────────────────────────────────────────────────────┐
  │  for {                                             │
  │    nextRun = calculateNext()    (daily/interval)   │
  │    select {                                        │
  │      case <-time.After(nextRun): execute()         │
  │      case <-stopChan:           return             │
  │    }                                               │
  │  }                                                 │
  └────────────────────────────────────────────────────┘

wakeup 模式不进入 goroutine：
  OnWake 回调 → [mode=="wakeup"] → execute()

execute() 防重入：
  isExecuting=true → 执行 task() → isExecuting=false
  若 isExecuting 已=true → slog.Warn + 跳过
```

---

## 分辨率选择算法

`bing.SelectVariant` 按以下优先级为目标屏幕选择最优图片变体：

```
输入：meta.Variants 列表、屏幕物理像素 (screenW×screenH)、forceUHD

Step 1: forceUHD=true → 直接返回 UHD 变体（若存在）

Step 2: 计算屏幕宽高比 screenAspect = screenW / screenH

Step 3: 找出宽高比与 screenAspect 最接近的变体（允许 2% 误差）→ candidates

Step 4: 在 candidates 中按优先级选择：
  P1. 若有 UHD 变体 → 返回 UHD
  P2. 分辨率 ≥ 屏幕 → 选最小的（节省带宽）
  P3. 分辨率 < 屏幕 → 选最大的（最佳画质）
```

**示例**（1920×1080 屏幕，DPR=1.5 → physicalW=2880，H=1620）：

| 候选变体 | 宽高比 | 是否 ≥ 屏幕 | 选择 |
|---------|--------|-----------|------|
| 1920×1080 | 1.778 | 否（2880>1920） | ✗ |
| 3840×2160 | 1.778 | 是 | ✓（最小 ≥ 屏幕）|
| UHD | 1.778 | 是 | ✓✓（P1 优先） |

---

## 跨平台差异矩阵

| 功能 | Windows | macOS | Linux |
|------|---------|-------|-------|
| 壁纸设置 | `IDesktopWallpaper` COM（多显示器）/ `SystemParametersInfo`（回退） | AppleScript `System Events` | `gsettings`（GNOME） |
| 多显示器 | ✅ COM 接口原生支持 | ⚠️ 需 AppleScript 枚举 | ❌ 无分屏支持 |
| 开机自启 | 注册表 `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` | LaunchAgent plist | ❌ 未实现 |
| 唤醒监听 | Win32 `WM_POWERBROADCAST` 消息（隐藏窗口） | NSWorkspace `NSWorkspaceDidWakeNotification` | ❌ 未实现 |
| Dock 图标 | N/A | `NSApp.setActivationPolicy(.accessory/.regular)` | N/A |
| 托盘图标格式 | ICO（从嵌入资源） | PNG 22×22（Template Image） | PNG |
| 数据目录默认 | 可执行文件目录（若只读则 `%APPDATA%`） | `~/Library/Application Support/BingPaperDesktop/` | 可执行文件目录 |

---

