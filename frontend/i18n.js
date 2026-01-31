import { createI18n } from "vue-i18n"

const STORAGE_KEY = "wincron.locale"

function normalizeLocale(value) {
  if (value === "zh") return "zh"
  if (value === "ja") return "ja"
  return "en"
}

function detectLocale() {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      return normalizeLocale(stored)
    }
  } catch {
  }

  const langs = Array.isArray(navigator?.languages) ? navigator.languages : []
  const candidate = String(langs[0] || navigator?.language || "")
  const lc = candidate.toLowerCase()
  if (lc.startsWith("zh")) return "zh"
  if (lc.startsWith("ja")) return "ja"
  return "en"
}

const messages = {
  en: {
    app: {
      language: "Language",
    },
    nav: {
      back: "Back",
      settings: "Settings",
    },
    route: {
      home: "Home",
      settings: "Settings",
    },
    global: {
      label: "Master Switch",
      enabled: "WinCron enabled",
      disabled: "WinCron disabled",
      disabled_title: "WinCron disabled",
      enable_wincorn: "Enable WinCron",
      disable_wincorn: "Disable WinCron",
    },
    common: {
      refresh: "Refresh",
      new: "New",
      edit: "Edit",
      delete: "Delete",
      run_now: "Run Now",
      save: "Save",
      run: "Run",
      enabled: "Enabled",
      disabled: "Disabled",
      enable: "Enable",
      disable: "Disable",
      cancel: "Cancel",
      export: "Export",
      import: "Import",
      ok: "OK",
      fail: "FAIL",
    },
    main: {
      jobs: {
        title: "Jobs",
        subtitle: "Schedule & run commands",
        empty: "No jobs yet",
        sort: {
          title: "Sort jobs",
          name: "Name",
          executed_count: "Executed",
          last_executed: "Last executed",
          next_run: "Next run",
          asc: "Ascending",
          desc: "Descending",
        },
      },
      editor: {
        title: "Editor",
        subtitle: "Create or edit a job",
      },
      fields: {
        name: "Name",
        cron: "Cron",
        command: "Command",
        args: "Args",
        preview: "Preview",
        workdir: "WorkDir",
        enabled: "Enabled",
        max_failures: "Disable after consecutive failures",
      },
      placeholders: {
        name: "Task name",
        cron: "0 * * * *",
        command: "C:\\Windows\\System32\\notepad.exe",
        arg: "arg",
        workdir: "C:\\\\",
        max_failures: "3",
      },
      next_run: {
        calculating: "Next run: calculating...",
        display: "Next run: {value}",
      },
      enabled_help: "Run on schedule",
      enabled_help_create: "Enable after creating",
      max_failures_help: "Auto-disable when reached",
      advanced: {
        show: "Show advanced options",
        hide: "Hide advanced options",
      },
      logs: {
        title: "Logs",
        subtitle: "Latest executions (max 100)",
        empty: "No logs",
        clear_title: "Clear logs",
      },
    },
    settings: {
      title: "Settings",
      subtitle: "Import/Export configuration & reset",
      reset_all: "Clear All Data",
      reset_confirm: "Are you sure you want to clear all data? This action cannot be undone.",
      export_yaml: "Export YAML Config",
      export_options_title: "Export YAML Config",
      export_options_subtitle: "Choose export options",
      export_settings: "Export settings",
      export_only_enabled: "Only enabled jobs",
      import_yaml: "Import YAML Config",
      import_title: "Import YAML Config",
      import_subtitle: "If job names conflict, choose a strategy",
      import_file: "File: {name}",
      import_strategy: {
        coexist: "Coexist",
        overwrite: "Overwrite",
      },
      import_strategy_help: {
        coexist: "Keep existing jobs. Imported jobs with the same name will be renamed (e.g. \"(imported)\").",
        overwrite: "Replace existing jobs that have the same name with the imported ones.",
      },
      conflicts: "Conflicts ({count})",
      no_conflicts: "No conflicts detected.",
      open_data_dir: "Open Data Directory",
      shortcut_guide: "Shortcut Guide",
      shortcuts_title: "Keyboard Shortcuts",
      shortcuts_subtitle: "Available shortcuts",
      shortcuts: {
        save: "Save",
        close_dialog: "Close dialog",
      },
      startup: "Startup",
      run_on_boot: "Run on boot",
      run_on_boot_help: "Create a shortcut in the Windows Startup folder.",
      silent_start: "Silent start",
      silent_start_help: "Start in tray without showing the main window.",
      lightweight_mode: "Lightweight mode",
      lightweight_mode_help: "When running in tray, unload the Webview process to reduce resource usage.",
      window: "Window",
      close_behavior: "Close button behavior",
      exit_application: "Exit application",
      hide_to_tray: "Hide to tray",
      hide_to_tray_help: "If set to “Hide to tray”, the app continues running in the background.",
      import_note: "Import will only prompt when job name conflicts are detected.",
    },
    toast: {
      saving: "Saving...",
      saved: "Saved",
      exporting: "Exporting...",
      export_cancelled: "Export cancelled",
      exported: "Exported",
      exported_with_path: "Exported: {path}",
      clearing: "Clearing...",
      cleared: "Cleared",
      importing: "Importing...",
      imported: "Imported",
      opened_data_dir: "Opened data directory",
      opened_data_dir_with_path: "Opened data directory: {dir}",
    },
    errors: {
      failed_to_save_job: "failed to save job",
      failed_to_update_job: "failed to update job",
      failed_to_run_job: "failed to run job",
      failed_to_run_preview: "failed to run preview",
    },
  },
  zh: {
    app: {
      language: "语言",
    },
    nav: {
      back: "返回",
      settings: "设置",
    },
    route: {
      home: "主页",
      settings: "设置",
    },
    global: {
      label: "总开关",
      enabled: "WinCron 已启用",
      disabled: "WinCron 已禁用",
      disabled_title: "WinCron已禁用",
      enable_wincorn: "启用WinCron",
      disable_wincorn: "禁用WinCron",
    },
    common: {
      refresh: "刷新",
      new: "新建",
      edit: "编辑",
      delete: "删除",
      run_now: "立即运行",
      save: "保存",
      run: "运行",
      enabled: "已启用",
      disabled: "已禁用",
      enable: "启用",
      disable: "禁用",
      cancel: "取消",
      export: "导出",
      import: "导入",
      ok: "成功",
      fail: "失败",
    },
    main: {
      jobs: {
        title: "任务",
        subtitle: "定时并运行命令",
        empty: "暂无任务",
        sort: {
          title: "排序",
          name: "名称",
          executed_count: "执行次数",
          last_executed: "最后执行",
          next_run: "下次运行",
          asc: "正序",
          desc: "倒序",
        },
      },
      editor: {
        title: "编辑器",
        subtitle: "创建或编辑任务",
      },
      fields: {
        name: "名称",
        cron: "Cron",
        command: "命令",
        args: "参数",
        preview: "预览",
        workdir: "工作目录",
        enabled: "启用",
        max_failures: "连续失败后禁用",
      },
      placeholders: {
        name: "任务名称",
        cron: "0 * * * *",
        command: "C:\\Windows\\System32\\notepad.exe",
        arg: "参数",
        workdir: "C:\\\\",
        max_failures: "3",
      },
      next_run: {
        calculating: "下次运行：计算中...",
        display: "下次运行：{value}",
      },
      enabled_help: "按计划运行",
      enabled_help_create: "创建后启用",
      max_failures_help: "达到后自动禁用",
      advanced: {
        show: "显示高级选项",
        hide: "隐藏高级选项",
      },
      logs: {
        title: "日志",
        subtitle: "最近执行（最多 100 条）",
        empty: "暂无日志",
        clear_title: "清空日志",
      },
    },
    settings: {
      title: "设置",
      subtitle: "导入/导出配置与重置",
      reset_all: "清除所有数据",
      reset_confirm: "你确定要清除所有数据吗？此操作无法撤销。",
      export_yaml: "导出 YAML 配置",
      export_options_title: "导出 YAML 配置",
      export_options_subtitle: "选择导出选项",
      export_settings: "导出设置",
      export_only_enabled: "仅导出已启用的任务",
      import_yaml: "导入 YAML 配置",
      import_title: "导入 YAML 配置",
      import_subtitle: "如果任务名称冲突，请选择策略",
      import_file: "文件：{name}",
      import_strategy: {
        coexist: "共存",
        overwrite: "覆盖",
      },
      import_strategy_help: {
        coexist: "保留现有任务。导入的同名任务会被重命名（例如 \"(imported)\"）。",
        overwrite: "用导入的任务替换现有的同名任务。",
      },
      conflicts: "冲突（{count}）",
      no_conflicts: "未检测到冲突。",
      open_data_dir: "打开数据目录",
      shortcut_guide: "快捷键指南",
      shortcuts_title: "快捷键",
      shortcuts_subtitle: "主页面可用快捷键",
      shortcuts: {
        save: "保存",
        close_dialog: "关闭弹窗",
      },
      startup: "启动",
      run_on_boot: "开机启动",
      run_on_boot_help: "在 Windows 启动文件夹中创建快捷方式。",
      silent_start: "静默启动",
      silent_start_help: "启动后隐藏到托盘，不显示主窗口。",
      lightweight_mode: "轻量模式",
      lightweight_mode_help: "托盘运行时卸载 Webview 进程，减少资源占用。",
      window: "窗口",
      close_behavior: "关闭按钮行为",
      exit_application: "退出程序",
      hide_to_tray: "隐藏到托盘",
      hide_to_tray_help: "如果设置为“隐藏到托盘”，程序会在后台继续运行。",
      import_note: "仅在检测到任务名称冲突时才会弹出导入提示。",
    },
    toast: {
      saving: "保存中...",
      saved: "已保存",
      exporting: "导出中...",
      export_cancelled: "已取消导出",
      exported: "已导出",
      exported_with_path: "已导出：{path}",
      clearing: "清理中...",
      cleared: "已清理",
      importing: "导入中...",
      imported: "已导入",
      opened_data_dir: "已打开数据目录",
      opened_data_dir_with_path: "已打开数据目录：{dir}",
    },
    errors: {
      failed_to_save_job: "保存任务失败",
      failed_to_update_job: "更新任务失败",
      failed_to_run_job: "运行任务失败",
      failed_to_run_preview: "运行预览失败",
    },
  },
  ja: {
    app: {
      language: "言語",
    },
    nav: {
      back: "戻る",
      settings: "設定",
    },
    route: {
      home: "ホーム",
      settings: "設定",
    },
    global: {
      label: "マスタースイッチ",
      enabled: "WinCron 有効",
      disabled: "WinCron 無効",
      disabled_title: "WinCron 無効",
      enable_wincorn: "WinCronを有効化",
      disable_wincorn: "WinCronを無効化",
    },
    common: {
      refresh: "更新",
      new: "新規",
      edit: "編集",
      delete: "削除",
      run_now: "今すぐ実行",
      save: "保存",
      run: "実行",
      enabled: "有効",
      disabled: "無効",
      enable: "有効化",
      disable: "無効化",
      cancel: "キャンセル",
      export: "エクスポート",
      import: "インポート",
      ok: "OK",
      fail: "失敗",
    },
    main: {
      jobs: {
        title: "ジョブ",
        subtitle: "コマンドをスケジュールして実行",
        empty: "ジョブがありません",
        sort: {
          title: "並び替え",
          name: "名前",
          executed_count: "実行回数",
          last_executed: "最終実行",
          next_run: "次回実行",
          asc: "昇順",
          desc: "降順",
        },
      },
      editor: {
        title: "エディター",
        subtitle: "ジョブの作成・編集",
      },
      fields: {
        name: "名前",
        cron: "Cron",
        command: "コマンド",
        args: "引数",
        preview: "プレビュー",
        workdir: "作業ディレクトリ",
        enabled: "有効",
        max_failures: "連続失敗後に無効化",
      },
      placeholders: {
        name: "わかりやすい名前",
        cron: "0 * * * *",
        command: "C:\\Windows\\System32\\notepad.exe",
        arg: "引数",
        workdir: "C:\\\\",
        max_failures: "3",
      },
      next_run: {
        calculating: "次回実行：計算中...",
        display: "次回実行：{value}",
      },
      enabled_help: "スケジュール通りに実行",
      enabled_help_create: "作成後に有効化",
      max_failures_help: "到達すると自動で無効化",
      advanced: {
        show: "詳細オプションを表示",
        hide: "詳細オプションを非表示",
      },
      logs: {
        title: "ログ",
        subtitle: "最新の実行（最大 100 件）",
        empty: "ログがありません",
        clear_title: "ログをクリア",
      },
    },
    settings: {
      title: "設定",
      subtitle: "設定のインポート/エクスポートとリセット",
      reset_all: "すべてのデータを消去",
      reset_confirm: "本当にすべてのデータを消去しますか？この操作は元に戻せません。",
      export_yaml: "YAML 設定をエクスポート",
      export_options_title: "YAML 設定をエクスポート",
      export_options_subtitle: "エクスポートのオプションを選択",
      export_settings: "設定をエクスポート",
      export_only_enabled: "有効なジョブのみ",
      import_yaml: "YAML 設定をインポート",
      import_title: "YAML 設定をインポート",
      import_subtitle: "ジョブ名が衝突する場合は方式を選択してください",
      import_file: "ファイル：{name}",
      import_strategy: {
        coexist: "共存",
        overwrite: "上書き",
      },
      import_strategy_help: {
        coexist: "既存のジョブを保持します。同名のインポート済みジョブはリネームされます（例：\"(imported)\"）。",
        overwrite: "同名の既存ジョブをインポートした内容で置き換えます。",
      },
      conflicts: "衝突（{count}）",
      no_conflicts: "衝突は検出されませんでした。",
      open_data_dir: "データフォルダを開く",
      shortcut_guide: "ショートカットガイド",
      shortcuts_title: "ショートカット",
      shortcuts_subtitle: "利用可能なショートカット",
      shortcuts: {
        save: "保存",
        close_dialog: "ダイアログを閉じる",
      },
      startup: "起動",
      run_on_boot: "Windows 起動時に実行",
      run_on_boot_help: "Windows のスタートアップフォルダにショートカットを作成します。",
      silent_start: "サイレント起動",
      silent_start_help: "メインウィンドウを表示せずにトレイで起動します。",
      lightweight_mode: "軽量モード",
      lightweight_mode_help: "トレイで実行中は Webview プロセスを解放し、リソース使用量を減らします。",
      window: "ウィンドウ",
      close_behavior: "閉じるボタンの動作",
      exit_application: "アプリを終了",
      hide_to_tray: "トレイに最小化",
      hide_to_tray_help: "“トレイに最小化” にすると、アプリはバックグラウンドで動作し続けます。",
      import_note: "ジョブ名の衝突が検出された場合のみインポート確認が表示されます。",
    },
    toast: {
      saving: "保存中...",
      saved: "保存しました",
      exporting: "エクスポート中...",
      export_cancelled: "エクスポートをキャンセルしました",
      exported: "エクスポートしました",
      exported_with_path: "エクスポートしました：{path}",
      clearing: "クリア中...",
      cleared: "クリアしました",
      importing: "インポート中...",
      imported: "インポートしました",
      opened_data_dir: "データフォルダを開きました",
      opened_data_dir_with_path: "データフォルダを開きました：{dir}",
    },
    errors: {
      failed_to_save_job: "ジョブの保存に失敗しました",
      failed_to_update_job: "ジョブの更新に失敗しました",
      failed_to_run_job: "ジョブの実行に失敗しました",
      failed_to_run_preview: "プレビューの実行に失敗しました",
    },
  },
}

const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: detectLocale(),
  fallbackLocale: "en",
  messages,
})

export function setAppLocale(locale) {
  const v = normalizeLocale(locale)
  i18n.global.locale.value = v
  try {
    localStorage.setItem(STORAGE_KEY, v)
  } catch {
  }
  return v
}

export default i18n
