# wincron

一个面向 Windows 的轻量级任务调度器。我希望能比windows自带的任务计划程序用起来更直观。

## 功能/特性

- 轻量设计：空闲时低开销
- 使用 cron 表达式定时执行任务
- 支持设置命令参数与工作目录
- 支持查看执行日志
- 失败保护：连续失败达到阈值后自动禁用任务
- 支持 YAML 导入/导出任务（可选导出设置）
- 托盘友好：最小化到托盘、可选静默启动、支持开机自启

## 截图

![Screenshot1-HomePage](./docs/preview1.png)
![Screenshot-Settings](./docs/preview2.png)

## 安装方式

### 下载 Release

- 从 GitHub Releases 页面下载 zip。

### 从源码构建

1. 安装 Wails3：

   `go install -v github.com/wailsapp/wails/v3/cmd/wails3@latest`

2. 构建，默认编译到bin/：

   `wails3 build`

## 开发环境

- Go：`1.25`（见 `go.mod`）
- Wails：`v3.0.0-alpha.60`（见 `go.mod`）
- 前端运行时：`@wailsio/runtime@3.0.0-alpha.78`（见 `frontend/package.json`）
- Bun：推荐使用（仓库包含 `frontend/bun.lock`）
- Node.js：可选（如果你不使用 Bun 而改用 npm/pnpm，建议 18+）
