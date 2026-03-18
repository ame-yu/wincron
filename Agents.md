# Repository Guidelines

## 项目结构与模块组织

本仓库是基于 Go、Wails 3、Bun 和 Vue 3 的 Windows 桌面任务调度器。

- 根目录下的 `*.go` 文件负责 Wails 应用入口、定时任务服务、存储、IPC 以及 Windows 平台集成。
- `cmd/ipcctl/` 是配套的命令行工具，用于 IPC 控制。
- `frontend/` 是 Vue 前端，主要包含 `components/`、`pages/`、`stores/`、`composables/`、`ui/` 和生成的 `bindings/`。
- `build/` 存放 Wails 构建配置、Windows 打包资源、图标、清单文件和 Taskfile。
- `docs/` 存放截图和说明文档，构建产物输出到 `bin/`。

## 构建、测试与开发命令

- `wails3 build`：标准验证命令，会生成前端资源并编译桌面应用。
修改代码后，提交前必须至少执行一次 `wails3 build`。
- Windows 构建产物默认包含 GUI 程序 `bin/wincron.exe` 和 CLI 工具 `bin/wincronctl.exe`。

## 代码风格与命名约定

- 项目仅适用于windows，切勿加入go:build !windows代码
- Go 代码使用标准 `gofmt` 风格，保留制表符缩进；导出符号使用 `PascalCase`，内部辅助函数使用 `camelCase`。
- Vue 和 JavaScript 使用 2 空格缩进、ES Module 语法，变量和函数使用 `camelCase`。
- 页面和组件文件优先使用 `PascalCase`；状态仓库和组合式函数采用描述性命名，例如 `useCronStore`。
- 如果修改显示文字，需要检查i18n.js做好国际化。
