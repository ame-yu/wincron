# wincron

一个面向 Windows 的轻量级任务调度器。我希望能比任务计划程序来的更直观。

## 功能/特性

- 轻量设计：空闲时低开销
- 使用 cron 表达式定时执行命令（robfig/cron）
- 支持设置命令参数与工作目录
- 支持立即运行 + 预览运行（在正式定时前先验证命令）
- 支持查看执行日志（stdout/stderr/退出码），并可清理日志
- 失败保护：连续失败达到阈值后自动禁用任务
- 支持 YAML 导入/导出任务（可选导出设置），并提供冲突处理策略
- 支持打开本地数据目录，方便备份/迁移
- 托盘友好：最小化到托盘、可选静默启动、支持开机自启
- 基于 Wails3 + Vue3 的桌面 UI

## 截图/GIF

![Screenshot1-HomePage](./docs/preview1.png)
![Screenshot-Settings](./docs/preview2.png)

## 安装方式

### 下载 Release

- 从 GitHub Releases 页面下载 zip。

### 从源码构建

1. 安装 Wails3：

   `go install -v github.com/wailsapp/wails/v3/cmd/wails3@latest`

2. 安装前端依赖：

   `bun install`

3. 以开发模式运行：

   `task dev`

## 快速开始

1. 启动应用。
2. 新建一个任务：
   - 名称
   - Cron 表达式（例如 `*/5 * * * *`）
   - 命令与参数
   - 工作目录（可选）
3. 保存并启用该任务。

## 开发环境

- Go：`1.25`（见 `go.mod`）
- Wails：`v3.0.0-alpha.60`（见 `go.mod`）
- 前端运行时：`@wailsio/runtime@3.0.0-alpha.78`（见 `frontend/package.json`）
- Bun：推荐使用（仓库包含 `frontend/bun.lock`）
- Node.js：可选（如果你不使用 Bun 而改用 npm/pnpm，建议 18+）
