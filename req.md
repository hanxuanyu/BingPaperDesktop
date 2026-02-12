你是资深桌面端工程师。请用 **Wails v2 + Go 1.22+** 开发一个跨平台桌面应用（Windows/macOS/Linux）。前端使用 **Vite + React + TypeScript + TailwindCSS + shadcn-ui** 搭建界面。产出必须是一个完整可运行工程（含 `go.mod`、Wails 配置、前端工程、README、必要脚本），并满足以下“便携版”约束与功能需求。

---

## 0) 关键约束（便携版存储）
**配置文件、数据文件、运行时日志必须全部存放在“应用可执行文件所在目录”下**，便于整体拷贝迁移：

- `./config.json`
- `./data/`
- `./data/index.json`
- `./data/YYYY-MM-DD/{original.jpg, watermarked.jpg, meta.json}`
- `./logs/app.log`（运行时日志必须落盘，滚动可选但至少追加写入）

实现“可执行文件目录”：
- 使用 `os.Executable()` + `filepath.EvalSymlinks` + `filepath.Dir`
- 所有读写路径都必须基于该目录拼接
- 若目录不可写（macOS `.app` 常见）：UI 给出明确错误并提示用户将应用放到可写目录；本版本不做自动回退到用户目录

---

## 1) 功能概述
应用启动后：
1) 从配置的 API 拉取“今日壁纸元数据”
2) 获取主屏幕分辨率
3) 在 `variants` 中选择最匹配且尽量清晰的图片
4) 下载图片保存到 `data/`，写入 `index.json`
5) 在主界面显示壁纸预览（大图/背景）
6) 提供按钮：手动“设为壁纸”
7) 提供开关：开启后每次获取成功自动设为壁纸
8) 支持定时：每天定时 or 按间隔自动获取（并可自动设置）
9) 支持历史：浏览/预览/设为壁纸/删除/清空
10) 可选叠加元数据（水印）：生成 `watermarked.jpg`，设置壁纸时优先使用水印图
11) 所有关键操作写运行时日志到 `./logs/app.log`（启动、配置加载、调度、拉取、选图、下载、设壁纸、清理、错误堆栈/原因）

默认元数据 API：
- `https://bing.coding.icu/api/v1/image/today/meta`

---

## 2) API 数据模型（后端必须严格实现）
```go
type Meta struct {
  Copyright     string    `json:"copyright"`
  CopyrightLink string    `json:"copyrightlink"`
  Date          string    `json:"date"`
  FullStartDate string    `json:"fullstartdate"`
  Hsh           string    `json:"hsh"`
  Mkt           string    `json:"mkt"`
  Quiz          string    `json:"quiz"`
  StartDate     string    `json:"startdate"`
  Title         string    `json:"title"`
  Variants      []Variant `json:"variants"`
}
type Variant struct {
  Format     string `json:"format"`
  Size       int64  `json:"size"`
  StorageKey string `json:"storage_key"`
  URL        string `json:"url"`
  Variant    string `json:"variant"` // "1920x1080" or "UHD"
}
```

---

## 3) 配置文件（./config.json）
启动读取，不存在则创建默认并落盘：
```json
{
  "api_meta_url": "https://bing.coding.icu/api/v1/image/today/meta",
  "auto_apply": false,
  "overlay_metadata": false,
  "prefer_aspect_match": true,
  "force_uhd": false,
  "schedule_mode": "daily",
  "daily_time": "08:30",
  "interval_minutes": 0,
  "retain_days": 0
}
```
约束：
- `schedule_mode`: `"off" | "daily" | "interval"`
- `interval_minutes >= 15`（interval 模式下必填）
- `retain_days`: 0 永久；>0 支持清理旧文件

---

## 4) 变体选择（分辨率匹配算法，必须实现）
- 获取主屏幕分辨率（宽高）：
  - 优先：前端通过 `window.screen.width/height` 与 `devicePixelRatio` 计算物理像素：`w = round(screen.width * dpr)`, `h = round(screen.height * dpr)`，传给后端用于选择
  - 后端也应允许无输入时用合理回退（例如选择最大非 UHD 或 UHD）
- 解析 `variants[].variant`：
  - `"UHD"` 作为候选（比较时按 3840x2160 估算，仅用于排序；下载用其 URL）
  - `"WxH"` 解析为整数
- 默认策略：
  1) `force_uhd=true` 且存在 UHD：直接选 UHD
  2) 否则（`prefer_aspect_match=true`）优先选宽高比最接近屏幕的候选（差值绝对值；2% 阈值判定“接近”）
  3) 在接近候选中，优先 **宽>=屏幕宽 且 高>=屏幕高** 里像素最小的
  4) 若都小于屏幕：选像素最大的
- 将 `chosen_variant` 写入历史索引与 `meta.json`

---

## 5) 下载与落盘（必须健壮）
- HTTP：超时 15s，重试 2 次（0.5s / 1s 退避），自定义 User-Agent，允许重定向
- 下载到 `.part` 临时文件，成功后原子 rename
- 校验：文件 size > 0；可选 `image.DecodeConfig` 读尺寸（失败则报错）
- 每日目录：`data/YYYY-MM-DD/`
  - `meta.json` 保存完整 Meta
  - `original.jpg`（按 format 扩展名）
  - 若开启水印：生成 `watermarked.jpg`

---

## 6) 历史索引（data/index.json，必须实现）
结构示例：
```json
{
  "items": [
    {
      "key": "2026-02-12_196137d1cf20534689772f22fbd8ece6",
      "date": "2026-02-12",
      "title": "聚焦进化",
      "copyright": "...",
      "chosen_variant": "1920x1080",
      "image_path": "data/2026-02-12/original.jpg",
      "watermark_path": "data/2026-02-12/watermarked.jpg",
      "created_at": "2026-02-12T08:31:10+08:00"
    }
  ]
}
```
要求：
- 去重：相同 key 不重复下载，直接复用本地文件
- 删除/清空要同步更新 index.json 并删除对应文件夹
- 提供“按 retain_days 清理”能力（手动触发按钮即可）

---

## 7) 设置桌面壁纸（后端 OS 封装必须清晰）
实现 `internal/wallpaper`：
- `Set(path string) error`
- `Supported() (bool, string)`

实现策略：
- Windows：`SystemParametersInfoW(SPI_SETDESKWALLPAPER)`
- macOS：`osascript` 设置每个 desktop 的 picture
- Linux：优先 GNOME（gsettings picture-uri / picture-uri-dark）；否则返回“不支持”
- UI 必须显示设置结果（成功/失败原因）；Linux 不支持时提示并提供“打开文件夹”能力

---

## 8) 元数据水印（overlay_metadata=true）
建议允许引入：`golang.org/x/image/font` 等用于绘制文字（允许该依赖）。
- 文本：title、date、copyright
- 位置：左下角；半透明黑底；白字；自动换行；最大宽度 70%
- 输出：`watermarked.jpg`
- 设壁纸时：若 watermarked 存在则优先使用 watermarked，否则 original

---

## 9) 定时调度（后端必须实现且与 UI 配置联动）
- `off`：关闭
- `daily`：按 `daily_time` 计算下次触发（跨天）
- `interval`：`time.Ticker`，分钟数 >=15
- 同时只允许一个 fetch 任务运行（mutex/状态机）；新触发到来时若正在执行，默认跳过并记日志

---

## 10) 前端技术栈与 UI 形态（必须按此实现）
前端：**React + shadcn-ui** 相关组件，界面尽量简洁，整体为“单页应用”：

### 10.1 单页布局（Home Only）
- 背景/大图预览区：显示当前壁纸（尽量铺满，带暗色渐变遮罩以保证按钮可读性）
- 右下角浮动操作区（简洁）：
  - `Button`：“设为壁纸”
  - `Button`：“刷新”
- 左上/右上角简洁状态与入口：
  - `Switch`：自动设置
  - `Switch`：叠加元数据
  - `Button`：“设置”（打开模态框）
  - `Button`：“历史”（打开底部抽屉）
- 状态提示：
  - 使用 `sonner`（若 shadcn-ui 标配）或 `Toast` 组件：显示“正在获取/下载完成/设壁纸成功/失败原因”
  - 页面上可有一行小字显示当前 date/title/variant

### 10.2 设置以“模态框”展示
使用 shadcn-ui 的 `Dialog` + 表单控件（Input/Select/Switch/Button）：
- API URL
- schedule_mode（Select）
- daily_time（time input）
- interval_minutes（number input，>=15）
- prefer_aspect_match、force_uhd
- retain_days
- 操作按钮：保存、立即清理、打开 data 目录、打开 logs 目录

### 10.3 壁纸历史以“底部抽屉”展示
使用 shadcn-ui 的 `Drawer`：
- 列表（简洁）：日期 + 标题 + variant
- 点击条目：在抽屉内显示预览与元数据（可只在选中时加载大图）
- 操作：设为壁纸、删除
- 抽屉底部：清空全部（需 `AlertDialog` 二次确认）

---

## 11) 前后端通信（必须）
后端导出 Wails bindings（示例，允许你调整返回结构但要完整可用）：
- `GetConfig() (Config, error)`
- `SaveConfig(cfg Config) error`
- `FetchToday(screenW int, screenH int, dpr float64) (CurrentResult, error)`
- `ApplyWallpaper(preferWatermarked bool) error`（对当前图片）
- `ListHistory() ([]HistoryItem, error)`
- `GetHistory(key string) (HistoryItemDetail, error)`
- `ApplyHistory(key string, preferWatermarked bool) error`
- `DeleteHistory(key string) error`
- `ClearHistory() error`
- `CleanupByRetainDays() (deleted int, error)`
- `OpenDataDir() error`
- `OpenLogsDir() error`
- `GetWallpaperSupport() (supported bool, note string)`

图片展示要求：
- 需要实现一个“前端可用的图片来源方式”。优先实现：
  - 后端方法：`GetImageDataURL(path string) (string, error)` 返回 `data:image/jpeg;base64,...`
  - 注意：大图可能占内存；仅对当前预览与历史选中项使用；历史列表不加载图片或加载缩略图（可选实现缩略图缓存到 `data/thumbs/`）

---

## 12) 运行时日志（必须）
- 使用 `log/slog` 写入 `./logs/app.log`（追加写入）
- 建议同时输出到 stdout（dev 模式方便），但必须落盘
- 至少记录：
  - 启动、版本、OS、可执行目录
  - 配置读写
  - 调度器启动/停止/下一次触发时间
  - 每次 fetch 的开始/结束、耗时
  - 选图结果（屏幕分辨率、chosen_variant）
  - 下载 URL（可只记录域名+path，避免过长）、保存路径、文件大小
  - 设壁纸调用与结果
  - 清理删除了哪些目录/数量
  - 错误必须带 `error` 字段，必要时 `%+v` 堆栈（若未引入 errors 包则记录上下文）

---

## 13) 工程结构（必须产出）
```
/ (project root)
  go.mod
  wails.json
  main.go (Wails entry)
  /frontend
    package.json
    vite.config.ts
    tsconfig.json
    tailwind.config.*
    src/
      main.tsx
      App.tsx
      lib/utils.ts
      components/... (shadcn-ui)
  /internal
    /app        // bindings + 状态 + 编排
    /bing       // api client, variant select, download
    /store      // portable paths, config, index, cleanup
    /wallpaper  // os-specific
    /scheduler  // daily/interval
    /overlay    // watermark
    /util       // open folder, retry, helpers, base64
```

---

## 14) README（必须包含）
- 开发运行：`wails dev`
- 构建：`wails build`
- 便携存储说明：`config.json`、`data/`、`logs/` 均在可执行文件目录
- Linux 桌面环境支持说明
- macOS `.app` 写入权限提示（便携模式限制）
- 日志位置与排查建议

---

## 15) 完成标准（必须可验收）
- `wails dev` 可启动，单页 UI 可操作（设置 Dialog、历史 Drawer）
- 启动自动获取并显示今日壁纸
- 刷新、手动设壁纸可用；自动设置开关生效
- 定时 daily/interval 生效，且不会并发重复执行
- 历史可浏览、可删除、可清空
- 配置、数据、日志均在可执行文件目录下生成并可随目录整体迁移

请直接输出完整工程代码与所有文件内容，不要省略关键实现。