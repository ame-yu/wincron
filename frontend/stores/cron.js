import { reactive, ref } from "vue"
import { defineStore } from "pinia"
import { Call, Dialogs, Events } from "@wailsio/runtime"
import i18n from "../i18n.js"

export const useCronStore = defineStore("cron", () => {
  const t = (...args) => i18n.global.t(...args)
  const error = ref("")
  const toast = ref("")
  const toastKind = ref("info")

  const closeBehavior = ref("tray")

  const silentStart = ref(false)
  const lightweightMode = ref(false)
  const autoStart = ref(false)

  const globalEnabled = ref(true)

  const jobs = ref([])
  const selectedJobId = ref("")
  const logs = ref([])
  const editorVisible = ref(true)

  const form = reactive({
    id: "",
    name: "",
    cron: "0 * * * *",
    command: "",
    args: [""],
    workDir: "",
    enabled: true,
    maxConsecutiveFailures: 3,
  })

  let toastTimer = null
  let offJobExecuted = null

  const cronServiceName = "main.CronService"
  const settingsServiceName = "main.SettingsService"
  const configServiceName = "main.ConfigService"

  function withTimeout(promise, ms) {
    if (!ms || ms <= 0) {
      return promise
    }
    let timer = null
    return Promise.race([
      promise,
      new Promise((_, reject) => {
        timer = setTimeout(() => reject(new Error(`timeout after ${ms}ms`)), ms)
      }),
    ]).finally(() => {
      if (timer) {
        clearTimeout(timer)
      }
    })
  }

  function normalizeArrayResult(result) {
    if (Array.isArray(result)) {
      return result
    }
    if (typeof result === "string") {
      try {
        const parsed = JSON.parse(result)
        return Array.isArray(parsed) ? parsed : []
      } catch {
        return []
      }
    }
    if (result && typeof result === "object") {
      const candidate = result.result ?? result.data ?? result.jobs ?? result.items
      return Array.isArray(candidate) ? candidate : []
    }
    return []
  }

  function normalizeSettingsResult(result) {
    if (!result) {
      return { closeBehavior: "tray" }
    }
    if (typeof result === "string") {
      try {
        return JSON.parse(result)
      } catch {
        return { closeBehavior: "tray" }
      }
    }
    if (result && typeof result === "object") {
      return result.settings ?? result.data ?? result.result ?? result
    }
    return { closeBehavior: "tray" }
  }

  function normalizeObjectResult(result) {
    if (!result) {
      return null
    }
    if (typeof result === "string") {
      try {
        return JSON.parse(result)
      } catch {
        return null
      }
    }
    if (result && typeof result === "object") {
      return result.result ?? result.data ?? result.item ?? result
    }
    return null
  }

  function normalizeStringArrayResult(result) {
    if (Array.isArray(result)) {
      return result.filter((v) => typeof v === "string")
    }
    if (typeof result === "string") {
      try {
        const parsed = JSON.parse(result)
        return Array.isArray(parsed) ? parsed.filter((v) => typeof v === "string") : []
      } catch {
        return []
      }
    }
    if (result && typeof result === "object") {
      const candidate = result.result ?? result.data ?? result.items
      return Array.isArray(candidate) ? candidate.filter((v) => typeof v === "string") : []
    }
    return []
  }

  function showToast(message, kind = "info") {
    toast.value = message
    toastKind.value = kind
    if (toastTimer) {
      clearTimeout(toastTimer)
    }
    toastTimer = setTimeout(() => {
      toast.value = ""
    }, 3000)
  }

  async function refreshJobs() {
    try {
      const result = await withTimeout(Call.ByName(`${cronServiceName}.ListJobs`), 5000)
      jobs.value = normalizeArrayResult(result)
    } catch (e) {
      const message = String(e)
      error.value = message
      jobs.value = []
      showToast(message, "danger")
    }
  }

  async function loadSettings() {
    try {
      const result = await withTimeout(Call.ByName(`${settingsServiceName}.GetSettings`), 5000)
      const settings = normalizeSettingsResult(result)
      closeBehavior.value = settings?.closeBehavior === "exit" ? "exit" : "tray"

      silentStart.value = !!settings?.silentStart
      lightweightMode.value = !!settings?.lightweightMode
      autoStart.value = !!settings?.autoStart
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
    }
  }

  async function loadGlobalEnabled() {
    try {
      const result = await withTimeout(Call.ByName(`${cronServiceName}.GetGlobalEnabled`), 5000)
      globalEnabled.value = !!result
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
    }
  }

  async function setGlobalEnabled(enabled) {
    const v = !!enabled
    try {
      await withTimeout(Call.ByName(`${cronServiceName}.SetGlobalEnabled`, v), 5000)
      globalEnabled.value = v
      showToast(v ? t("global.enabled") : t("global.disabled"), "success")
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
      throw e
    }
  }

  async function previewNextRun(cronExpr) {
    const expr = typeof cronExpr === "string" ? cronExpr : ""
    const result = await withTimeout(Call.ByName(`${cronServiceName}.PreviewNextRun`, expr), 3000)
    if (typeof result === "string") {
      return result
    }
    if (result && typeof result === "object") {
      const candidate = result.result ?? result.data ?? result.value
      return typeof candidate === "string" ? candidate : ""
    }
    return ""
  }

  async function setCloseBehavior(behavior) {
    const normalized = behavior === "exit" ? "exit" : "tray"
    try {
      await withTimeout(Call.ByName(`${settingsServiceName}.SetCloseBehavior`, normalized), 5000)
      closeBehavior.value = normalized
      showToast(t("toast.saved"), "success")
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
      throw e
    }
  }

  async function setSilentStart(enabled) {
    const v = !!enabled
    try {
      await withTimeout(Call.ByName(`${settingsServiceName}.SetSilentStart`, v), 5000)
      silentStart.value = v
      if (!v) {
        lightweightMode.value = false
      }
      showToast(t("toast.saved"), "success")
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
      throw e
    }
  }

  async function setLightweightMode(enabled) {
    const v = !!enabled
    try {
      await withTimeout(Call.ByName(`${settingsServiceName}.SetLightweightMode`, v), 5000)
      lightweightMode.value = v
      showToast(t("toast.saved"), "success")
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
      throw e
    }
  }

  async function setAutoStart(enabled) {
    const v = !!enabled
    try {
      await withTimeout(Call.ByName(`${settingsServiceName}.SetAutoStart`, v), 5000)
      autoStart.value = v
      showToast(t("toast.saved"), "success")
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
      throw e
    }
  }

  async function openDataDir() {
    error.value = ""
    try {
      const result = await withTimeout(Call.ByName(`${settingsServiceName}.OpenDataDir`), 5000)
      const dir = typeof result === "string" ? result : ""
      showToast(dir ? t("toast.opened_data_dir_with_path", { dir }) : t("toast.opened_data_dir"), "success")
      return dir
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
      throw e
    }
  }

  function loadJobToForm(job) {
    selectedJobId.value = job.id
    editorVisible.value = true
    form.id = job.id
    form.name = job.name ?? ""
    form.cron = job.cron ?? "0 * * * *"
    form.command = job.command ?? ""
    form.args = Array.isArray(job.args) && job.args.length ? [...job.args] : [""]
    form.workDir = job.workDir ?? ""
    form.enabled = !!job.enabled
    const mcf = Number(job.maxConsecutiveFailures)
    form.maxConsecutiveFailures = Number.isFinite(mcf) && mcf > 0 ? mcf : 3
  }

  function resetForm() {
    selectedJobId.value = ""
    editorVisible.value = true
    form.id = ""
    form.name = ""
    form.cron = "0 * * * *"
    form.command = ""
    form.args = [""]
    form.workDir = ""
    form.enabled = true
    form.maxConsecutiveFailures = 3
    logs.value = []
  }

  async function selectJob(jobId) {
    const id = typeof jobId === "string" ? jobId : ""
    selectedJobId.value = id
    editorVisible.value = false
    if (!id) {
      logs.value = []
      return
    }
    await loadLogs(id)
  }

  async function saveJob() {
    error.value = ""
    showToast(t("toast.saving"), "info")
    try {
      const args = Array.isArray(form.args) ? form.args.filter((s) => s !== "") : []

      const savedRaw = await withTimeout(
        Call.ByName(`${cronServiceName}.UpsertJob`, {
          id: form.id,
          name: form.name,
          cron: form.cron,
          command: form.command,
          args,
          workDir: form.workDir,
          enabled: form.enabled,
          maxConsecutiveFailures: Number(form.maxConsecutiveFailures) || 3,
        }),
        5000,
      )

      const saved = normalizeObjectResult(savedRaw)
      if (!saved?.id) {
        throw new Error(t("errors.failed_to_save_job"))
      }

      await refreshJobs()
      loadJobToForm(saved)
      await loadLogs(saved.id)
      showToast(t("toast.saved"), "success")
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
    }
  }

  async function deleteJob(id) {
    error.value = ""
    try {
      await withTimeout(Call.ByName(`${cronServiceName}.DeleteJob`, id), 5000)
      await refreshJobs()
      if (selectedJobId.value === id) {
        resetForm()
        logs.value = []
      }
    } catch (e) {
      error.value = String(e)
    }
  }

  async function toggleJob(job) {
    error.value = ""
    try {
      const updatedRaw = await withTimeout(Call.ByName(`${cronServiceName}.SetJobEnabled`, job.id, !job.enabled), 5000)
      const updated = normalizeObjectResult(updatedRaw)
      if (!updated?.id) {
        throw new Error(t("errors.failed_to_update_job"))
      }
      await refreshJobs()
      if (selectedJobId.value === updated.id) {
        loadJobToForm(updated)
      }
    } catch (e) {
      error.value = String(e)
    }
  }

  async function runNow(jobId) {
    error.value = ""
    try {
      const entryRaw = await withTimeout(Call.ByName(`${cronServiceName}.RunNow`, jobId), 60000)
      const entry = normalizeObjectResult(entryRaw)
      if (!entry) {
        throw new Error(t("errors.failed_to_run_job"))
      }
      if (!selectedJobId.value || selectedJobId.value === jobId) {
        logs.value = [...logs.value, entry]
      }
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
    }
  }

  async function runPreviewFromForm() {
    error.value = ""
    try {
      const args = Array.isArray(form.args) ? form.args.filter((s) => s !== "") : []
      const entryRaw = await withTimeout(
        Call.ByName(`${cronServiceName}.RunPreview`, {
          command: form.command,
          args,
          workDir: form.workDir,
          jobId: form.id,
          jobName: form.name,
        }),
        60000,
      )
      const entry = normalizeObjectResult(entryRaw)
      if (!entry) {
        throw new Error(t("errors.failed_to_run_preview"))
      }
      logs.value = [...logs.value, entry]
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
    }
  }

  async function loadLogs(jobId) {
    try {
      const result = await withTimeout(Call.ByName(`${cronServiceName}.ListLogs`, jobId, 100), 5000)
      logs.value = normalizeArrayResult(result)
    } catch (e) {
      const message = String(e)
      error.value = message
      logs.value = []
      showToast(message, "danger")
    }
  }

  async function clearLogs() {
    error.value = ""
    showToast(t("toast.clearing"), "info")
    try {
      await withTimeout(Call.ByName(`${cronServiceName}.ClearLogs`), 5000)
      logs.value = []
      showToast(t("toast.cleared"), "success")
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
      throw e
    }
  }

  async function resetAll() {
    error.value = ""
    showToast(t("toast.clearing"), "info")
    try {
      await withTimeout(Call.ByName(`${cronServiceName}.ResetAll`), 5000)
      resetForm()
      logs.value = []
      await refreshJobs()
      showToast(t("toast.cleared"), "success")
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
      throw e
    }
  }

  async function exportConfig(options = {}) {
    error.value = ""
    showToast(t("toast.exporting"), "info")
    try {
      const exportSettings = !!options.exportSettings
      const onlyEnabled = !!options.onlyEnabled

      const d = new Date()
      const pad2 = (n) => String(n).padStart(2, "0")
      const ts = `${d.getFullYear()}${pad2(d.getMonth() + 1)}${pad2(d.getDate())}-${pad2(d.getHours())}${pad2(d.getMinutes())}${pad2(d.getSeconds())}`
      const defaultName = `wincron-config-${ts}.yml`

      const filePath = await Dialogs.SaveFile({
        Title: t("settings.export_yaml"),
        ButtonText: t("common.export"),
        Filename: defaultName,
        Filters: [{ DisplayName: "YAML", Pattern: "*.yml;*.yaml" }],
      })

      if (!filePath) {
        showToast(t("toast.export_cancelled"), "info")
        return
      }

      const path = await withTimeout(
        Call.ByName(`${configServiceName}.ExportYAMLToFile`, filePath, exportSettings, onlyEnabled),
        5000,
      )
      showToast(path ? t("toast.exported_with_path", { path }) : t("toast.exported"), "success")
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
      throw e
    }
  }

  async function checkImportConflicts(text) {
    const conflictsRaw = await withTimeout(Call.ByName(`${configServiceName}.CheckImportYAMLConflicts`, text), 5000)
    return normalizeStringArrayResult(conflictsRaw)
  }

  async function importConfig(text, conflictStrategy = "coexist") {
    error.value = ""
    showToast(t("toast.importing"), "info")
    try {
      const strategy = conflictStrategy === "overwrite" ? "overwrite" : "coexist"
      await withTimeout(Call.ByName(`${configServiceName}.ImportYAML`, text, strategy), 5000)
      resetForm()
      logs.value = []
      await refreshJobs()
      await loadGlobalEnabled()
      await loadSettings()
      showToast(t("toast.imported"), "success")
    } catch (e) {
      const message = String(e)
      error.value = message
      showToast(message, "danger")
      throw e
    }
  }

  async function init() {
    if (offJobExecuted) {
      return
    }

    await loadSettings()

    await loadGlobalEnabled()

    await refreshJobs()

    offJobExecuted = Events.On("jobExecuted", async (event) => {
      const entry = event?.data
      if (!entry) {
        return
      }

      const ok = entry.exitCode === 0
      showToast(
        `${entry.jobName}: ${ok ? t("common.ok") : `${t("common.fail")} (exit=${entry.exitCode})`}`,
        ok ? "success" : "danger",
      )

      await refreshJobs()
      if (selectedJobId.value) {
        const job = Array.isArray(jobs.value) ? jobs.value.find((j) => j?.id === selectedJobId.value) : null
        if (job) {
          loadJobToForm(job)
        }
      }

      if (selectedJobId.value && entry.jobId === selectedJobId.value) {
        await loadLogs(selectedJobId.value)
      }
    })
  }

  function dispose() {
    if (offJobExecuted) {
      offJobExecuted()
      offJobExecuted = null
    }
    if (toastTimer) {
      clearTimeout(toastTimer)
      toastTimer = null
    }
  }

  return {
    error,
    toast,
    toastKind,
    closeBehavior,
    silentStart,
    lightweightMode,
    autoStart,
    globalEnabled,
    jobs,
    selectedJobId,
    logs,
    editorVisible,
    form,
    refreshJobs,
    loadJobToForm,
    selectJob,
    resetForm,
    saveJob,
    deleteJob,
    toggleJob,
    runNow,
    runPreviewFromForm,
    loadLogs,
    clearLogs,
    resetAll,
    exportConfig,
    checkImportConflicts,
    importConfig,
    loadSettings,
    loadGlobalEnabled,
    setCloseBehavior,
    setSilentStart,
    setLightweightMode,
    setAutoStart,
    setGlobalEnabled,
    previewNextRun,
    openDataDir,
    init,
    dispose,
  }
})
