import { Dialogs } from "@wailsio/runtime"

export function createConfigActions(ctx) {
  async function exportConfig(options = {}) {
    ctx.error.value = ""
    ctx.showToast(ctx.t("toast.exporting"), "info")
    try {
      const exportJobs = options.exportJobs == null ? true : !!options.exportJobs
      const exportSettings = !!options.exportSettings
      const onlyEnabled = !!options.onlyEnabled

      const d = new Date()
      const pad2 = (n) => String(n).padStart(2, "0")
      const ts = `${d.getFullYear()}${pad2(d.getMonth() + 1)}${pad2(d.getDate())}-${pad2(d.getHours())}${pad2(
        d.getMinutes(),
      )}${pad2(d.getSeconds())}`
      const defaultName = `wincron-config-${ts}.yml`

      const filePath = await Dialogs.SaveFile({
        Title: ctx.t("settings.export_yaml"),
        ButtonText: ctx.t("common.export"),
        Filename: defaultName,
        Filters: [{ DisplayName: "YAML", Pattern: "*.yml;*.yaml" }],
      })

      if (!filePath) {
        ctx.showToast(ctx.t("toast.export_cancelled"), "info")
        return
      }

      const path = await ctx.callConfigT(5000, "ExportYAMLToFile", filePath, exportJobs, exportSettings, onlyEnabled)
      ctx.showToast(path ? ctx.t("toast.exported_with_path", { path }) : ctx.t("toast.exported"), "success")
    } catch (e) {
      ctx.reportError(e, { rethrow: true })
    }
  }

  async function checkImportConflicts(text) {
    const conflictsRaw = await ctx.callConfigT(5000, "CheckImportYAMLConflicts", text)
    return ctx.normalizeStringArrayResult(conflictsRaw)
  }

  async function importConfig(text, conflictStrategy = "coexist") {
    ctx.error.value = ""
    ctx.showToast(ctx.t("toast.importing"), "info")
    try {
      const strategy = conflictStrategy === "overwrite" ? "overwrite" : "coexist"
      await ctx.callConfigT(5000, "ImportYAML", text, strategy)
      ctx.resetFormWithEditor(false)
      await ctx.refreshJobs()
      await ctx.loadGlobalEnabled()
      await ctx.loadSettings()
      await ctx.focusLogs("")
      ctx.showToast(ctx.t("toast.imported"), "success")
    } catch (e) {
      ctx.reportError(e, { rethrow: true })
    }
  }

  return {
    exportConfig,
    checkImportConflicts,
    importConfig,
  }
}
