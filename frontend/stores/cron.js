import { reactive, ref, watch } from "vue"
import { defineStore } from "pinia"
import { Call, Dialogs, Events } from "@wailsio/runtime"
import i18n from "../i18n.js"

export const useCronStore = defineStore("cron", () => {
  const t = (...args) => i18n.global.t(...args)
  const error = ref("")
  const toast = ref("")
  const toastKind = ref("info")
  const toastActionLabel = ref("")

  const editorPulse = ref(null)

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
    folder: "",
    cron: "0 * * * *",
    command: "",
    args: [""],
    workDir: "",
    flagProcessCreation: "",
    timeout: 0,
    concurrencyPolicy: "skip",
    enabled: true,
    maxConsecutiveFailures: 3,
  })

  let formBaseline = ""

  const formDirty = ref(false)

  const draftKey = "wincron.jobDraft.v3"
  let draftSaveTimer = null

  const readDraftRaw = () => {
    try {
      return localStorage.getItem(draftKey) || ""
    } catch {
      return ""
    }
  }

  const parseDraftRaw = (raw) => {
    if (!raw || typeof raw !== "string") {
      return null
    }
    try {
      const data = JSON.parse(raw)
      const formData = data?.form && typeof data.form === "object" ? data.form : null
      const baseline = typeof data?.baseline === "string" ? data.baseline : ""
      if (!formData) {
        return null
      }
      return { form: formData, baseline, ts: Number(data?.ts) || 0 }
    } catch {
      return null
    }
  }

  let toastTimer = null
  let toastAction = null
  let toastOnDismiss = null
  let offJobExecuted = null
  const pendingDeleteJobs = new Map()

  const cronServiceName = "main.CronService"
  const settingsServiceName = "main.SettingsService"
  const configServiceName = "main.ConfigService"

  function call(serviceName, methodName, ...args) {
    return Call.ByName(`${serviceName}.${methodName}`, ...args)
  }

  async function callSetJobFolder(id, folder) {
    const updatedRaw = await callCronT(5000, "SetJobFolder", id, folder)
    const updated = normalizeObjectResult(updatedRaw)
    if (!updated?.id) {
      throw new Error(t("errors.failed_to_update_job"))
    }
    return updated
  }

  async function setJobFolder(jobId, folder) {
    error.value = ""
    try {
      const id = typeof jobId === "string" ? jobId : ""
      if (!id) {
        return
      }
      const f = typeof folder === "string" ? folder : ""
      const updated = await callSetJobFolder(id, f)
      await refreshJobs()
      if (selectedJobId.value === updated.id && !isFormDirty()) {
        loadJobToForm(updated)
      }
    } catch (e) {
      error.value = String(e)
    }
  }

  async function setJobsFolder(jobIds, folder) {
    error.value = ""
    try {
      const ids = Array.isArray(jobIds) ? jobIds.map((v) => String(v || "")).filter((v) => v) : []
      if (!ids.length) {
        return
      }
      const f = typeof folder === "string" ? folder : ""

      for (const id of ids) {
        await callSetJobFolder(id, f)
      }

      await refreshJobs()
      const sid = selectedJobId.value
      if (sid && ids.includes(sid)) {
        const job = Array.isArray(jobs.value) ? jobs.value.find((j) => String(j?.id || "") === sid) : null
        if (job && !isFormDirty()) loadJobToForm(job)
      }
    } catch (e) {
      error.value = String(e)
    }
  }

  function callWithTimeout(serviceName, methodName, timeoutMs, ...args) {
    return withTimeout(call(serviceName, methodName, ...args), timeoutMs)
  }

  const callCronT = (timeoutMs, methodName, ...args) => callWithTimeout(cronServiceName, methodName, timeoutMs, ...args)

  const callSettingsT = (timeoutMs, methodName, ...args) => callWithTimeout(settingsServiceName, methodName, timeoutMs, ...args)

  const callConfigT = (timeoutMs, methodName, ...args) => callWithTimeout(configServiceName, methodName, timeoutMs, ...args)

  const parseJson = (value) => {
    if (typeof value !== "string") {
      return undefined
    }
    try {
      return JSON.parse(value)
    } catch {
      return undefined
    }
  }

  async function copyJob(job) {
    error.value = ""
    try {
      if (!job || typeof job !== "object") {
        return
      }

      const srcName = String(job?.name || "").trim()
      const srcCommand = String(job?.command || "").trim()
      const baseName = srcName || srcCommand || "Job"
      const copiedName = `${baseName} (${t("common.copy")})`

      const args = Array.isArray(job?.args) ? job.args.filter((s) => s !== "") : []

      const savedRaw = await callCronT(5000, "UpsertJob", {
        id: "",
        name: copiedName,
        folder: job?.folder ?? "",
        cron: job?.cron ?? "0 * * * *",
        command: job?.command ?? "",
        args,
        workDir: job?.workDir ?? "",
        flagProcessCreation: String(job?.flagProcessCreation ?? ""),
        timeout: Number(job?.timeout) || 0,
        concurrencyPolicy: job?.concurrencyPolicy ? String(job.concurrencyPolicy) : "skip",
        enabled: !!job?.enabled,
        maxConsecutiveFailures: Number(job?.maxConsecutiveFailures) || 3,
      })

      const saved = normalizeObjectResult(savedRaw)
      if (!saved?.id) {
        throw new Error(t("errors.failed_to_save_job"))
      }

      await refreshJobs()
      loadJobToForm(saved)
      await loadLogs(saved.id)
      showToast(t("toast.copied_with_name", { name: saved?.name || copiedName }), "success")
    } catch (e) {
      reportError(e)
    }
  }

  function normalize(result, { kind, keys, defaultValue }) {
    if (kind === "array") {
      if (Array.isArray(result)) {
        return result
      }
      if (typeof result === "string") {
        const parsed = parseJson(result)
        return Array.isArray(parsed) ? parsed : defaultValue
      }
      if (result && typeof result === "object") {
        const candidate = pick(result, keys)
        return Array.isArray(candidate) ? candidate : defaultValue
      }
      return defaultValue
    }

    if (kind === "settings") {
      if (!result) {
        return defaultValue
      }
      if (typeof result === "string") {
        return parseJson(result) ?? defaultValue
      }
      if (result && typeof result === "object") {
        return result.settings ?? result.data ?? result.result ?? result
      }
      return defaultValue
    }

    // kind === "object"
    if (!result) {
      return defaultValue
    }
    if (typeof result === "string") {
      return parseJson(result) ?? defaultValue
    }
    if (result && typeof result === "object") {
      return pick(result, keys) ?? result
    }
    return defaultValue
  }

  function normalizeArrayResult(result, keys = ["result", "data", "jobs", "items"]) {
    return normalize(result, { kind: "array", keys, defaultValue: [] })
  }

  function normalizeSettingsResult(result) {
    return normalize(result, { kind: "settings", keys: [], defaultValue: { closeBehavior: "tray" } })
  }

  function normalizeObjectResult(result, keys = ["result", "data", "item"]) {
    return normalize(result, { kind: "object", keys, defaultValue: null })
  }

  function normalizeStringArrayResult(result) {
    return normalizeArrayResult(result, ["result", "data", "items"]).filter((v) => typeof v === "string")
  }

  function pick(obj, keys) {
    for (const key of keys) {
      const v = obj?.[key]
      if (v != null) {
        return v
      }
    }
    return undefined
  }

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

  function clearToastTimer() {
    if (!toastTimer) {
      return
    }
    clearTimeout(toastTimer)
    toastTimer = null
  }

  function clearToast() {
    toast.value = ""
    toastActionLabel.value = ""
    toastAction = null
    toastOnDismiss = null
  }

  function dismissToast() {
    if (!toast.value) {
      clearToastTimer()
      return
    }
    const onDismiss = toastOnDismiss
    clearToast()
    clearToastTimer()
    if (typeof onDismiss === "function") {
      onDismiss()
    }
  }

  function triggerToastAction() {
    const action = toastAction
    toastOnDismiss = null
    clearToast()
    clearToastTimer()
    if (typeof action === "function") {
      action()
    }
  }

  function showToast(message, kind = "info", options = {}) {
    if (toast.value) {
      dismissToast()
    }
    toast.value = message
    toastKind.value = kind

    const label = typeof options?.actionLabel === "string" ? options.actionLabel : ""
    toastActionLabel.value = label
    toastAction = typeof options?.onAction === "function" ? options.onAction : null
    toastOnDismiss = typeof options?.onDismiss === "function" ? options.onDismiss : null

    const durationMs = Number(options?.durationMs)
    const ms = Number.isFinite(durationMs) && durationMs > 0 ? durationMs : 3000

    clearToastTimer()
    toastTimer = setTimeout(() => {
      dismissToast()
    }, ms)
  }

  function reportError(e, { kind = "danger", rethrow = false } = {}) {
    const message = String(e)
    error.value = message
    showToast(message, kind)
    if (rethrow) {
      throw e
    }
  }

  function triggerEditorPulse(kind) {
    const k = kind === "success" ? "success" : kind === "error" ? "error" : ""
    if (!k) {
      return
    }
    editorPulse.value = { kind: k, ts: Date.now() }
  }

  const getFormSnapshot = () =>
    JSON.stringify({
      id: String(form.id || ""),
      name: String(form.name ?? ""),
      folder: String(form.folder ?? ""),
      cron: String(form.cron ?? ""),
      command: String(form.command ?? ""),
      args: Array.isArray(form.args) ? form.args.map((s) => String(s ?? "")).filter((s) => s !== "") : [],
      workDir: String(form.workDir ?? ""),
      flagProcessCreation: String(form.flagProcessCreation ?? ""),
      timeout: Number(form.timeout) || 0,
      concurrencyPolicy: String(form.concurrencyPolicy || "skip"),
      enabled: !!form.enabled,
      maxConsecutiveFailures: Number(form.maxConsecutiveFailures) || 3,
    })

  const setDirtyState = (value) => {
    const v = !!value
    if (formDirty.value === v) {
      return
    }
    formDirty.value = v
  }

  const markFormClean = () => {
    formBaseline = getFormSnapshot()
    setDirtyState(false)
  }

  const isFormDirty = () => !!formDirty.value

  markFormClean()

  const loadDraft = () => parseDraftRaw(readDraftRaw())

  const restoreDraft = () => {
    const draft = loadDraft()
    if (!draft) {
      return false
    }

    try {
      const d = draft.form
      editorVisible.value = true
      form.id = String(d.id || "")
      form.name = String(d.name ?? "")
      form.folder = String(d.folder ?? "")
      form.cron = String(d.cron ?? "0 * * * *")
      form.command = String(d.command ?? "")
      form.args = Array.isArray(d.args) && d.args.length ? d.args.map((v) => String(v ?? "")) : [""]
      form.workDir = String(d.workDir ?? "")
      form.flagProcessCreation = String(d.flagProcessCreation ?? "")
      form.timeout = Number(d.timeout) || 0
      form.concurrencyPolicy = String(d.concurrencyPolicy || "skip")
      form.enabled = !!d.enabled
      form.maxConsecutiveFailures = Number(d.maxConsecutiveFailures) || 3
      selectedJobId.value = String(d.id || "")
      logs.value = []

      if (typeof draft.baseline === "string") {
        formBaseline = draft.baseline
      }

      try {
        if (window.location.hash !== "#/") {
          window.location.hash = "#/"
        }
      } catch {
        // noop
      }
      clearDraft()
      return true
    } catch {
      return false
    }
  }

  const promptDraftRecovery = (force = false) => {
    const raw = readDraftRaw()
    const draft = raw && parseDraftRaw(raw)
    if (raw && !draft) {
      clearDraft()
      return false
    }
    if (!draft || (!force && isFormDirty())) return false

    showToast(t("toast.draft_found"), "info", {
      actionLabel: t("toast.draft_resume"),
      onAction: restoreDraft,
      onDismiss: () => {
        clearDraft()
      },
      durationMs: 8000,
    })
    return true
  }

  const saveDraftNow = () => {
    try {
      const snapshotStr = getFormSnapshot()
      if (snapshotStr === formBaseline) {
        return
      }
      const snapshot = JSON.parse(snapshotStr)
      localStorage.setItem(
        draftKey,
        JSON.stringify({
          form: snapshot,
          baseline: formBaseline,
          ts: Date.now(),
        }),
      )
    } catch {
      // noop
    }
  }

  const scheduleDraftSave = () => {
    if (draftSaveTimer) {
      clearTimeout(draftSaveTimer)
    }
    draftSaveTimer = setTimeout(() => {
      draftSaveTimer = null
      saveDraftNow()
    }, 300)
  }

  watch(
    getFormSnapshot,
    (snapshotStr) => {
      const dirty = snapshotStr !== formBaseline
      setDirtyState(dirty)
      if (dirty) {
        scheduleDraftSave()
      } else if (draftSaveTimer) {
        clearTimeout(draftSaveTimer)
        draftSaveTimer = null
      }
    },
    { immediate: true },
  )

  const clearDraft = () => {
    try {
      localStorage.removeItem(draftKey)
    } catch {
      // noop
    }
  }

  const flushDraft = () => {
    if (draftSaveTimer) {
      clearTimeout(draftSaveTimer)
      draftSaveTimer = null
    }
    saveDraftNow()
  }

  async function updateSetting(callName, value, apply) {
    try {
      await callSettingsT(5000, callName, value)
      apply(value)
      showToast(t("toast.saved"), "success")
    } catch (e) {
      reportError(e)
      await loadSettings()
    }
  }

  async function refreshJobs() {
    try {
      const result = await callCronT(5000, "ListJobs")
      const listed = normalizeArrayResult(result)
      const pending = pendingDeleteJobs.size ? new Set(pendingDeleteJobs.keys()) : null
      jobs.value = pending ? listed.filter((j) => !pending.has(String(j?.id || ""))) : listed
    } catch (e) {
      jobs.value = []
      reportError(e)
    }
  }

  async function loadSettings() {
    try {
      const result = await callSettingsT(5000, "GetSettings")
      const settings = normalizeSettingsResult(result)
      closeBehavior.value = settings?.closeBehavior === "exit" ? "exit" : "tray"

      silentStart.value = !!settings?.silentStart
      lightweightMode.value = !!settings?.lightweightMode
      autoStart.value = !!settings?.autoStart
    } catch (e) {
      reportError(e)
    }
  }

  async function loadGlobalEnabled() {
    try {
      const result = await callCronT(5000, "GetGlobalEnabled")
      globalEnabled.value = !!result
    } catch (e) {
      reportError(e)
    }
  }

  async function setGlobalEnabled(enabled) {
    const v = !!enabled
    try {
      await callCronT(5000, "SetGlobalEnabled", v)
      globalEnabled.value = v
      showToast(v ? t("global.enabled") : t("global.disabled"), "success")
    } catch (e) {
      reportError(e)
    }
  }

  async function previewNextRun(cronExpr) {
    const expr = typeof cronExpr === "string" ? cronExpr : ""
    const result = await callCronT(3000, "PreviewNextRun", expr)
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
    await updateSetting("SetCloseBehavior", normalized, (v) => (closeBehavior.value = v))
  }

  async function setSilentStart(enabled) {
    const v = !!enabled

    await updateSetting("SetSilentStart", v, (next) => {
      silentStart.value = next
    })
  }

  async function setLightweightMode(enabled) {
    const v = !!enabled
    await updateSetting("SetLightweightMode", v, (next) => (lightweightMode.value = next))
  }

  async function setAutoStart(enabled) {
    const v = !!enabled
    await updateSetting("SetAutoStart", v, (next) => (autoStart.value = next))
  }

  async function openDataDir() {
    error.value = ""
    try {
      const result = await callSettingsT(5000, "OpenDataDir")
      const dir = typeof result === "string" ? result : ""
      showToast(dir ? t("toast.opened_data_dir_with_path", { dir }) : t("toast.opened_data_dir"), "success")
      return dir
    } catch (e) {
      reportError(e)
    }
  }

  async function openEnvironmentVariables() {
    error.value = ""
    try {
      await callSettingsT(5000, "OpenEnvironmentVariables")
      showToast(t("toast.opened_environment_variables"), "success")
    } catch (e) {
      reportError(e)
    }
  }

  function loadJobToForm(job) {
    saveDraftNow()
    selectedJobId.value = job.id
    editorVisible.value = true
    form.id = job.id
    form.name = job.name ?? ""
    form.folder = job.folder ?? ""
    form.cron = job.cron ?? "0 * * * *"
    form.command = job.command ?? ""
    form.args = Array.isArray(job.args) && job.args.length ? [...job.args] : [""]
    form.workDir = job.workDir ?? ""

    form.flagProcessCreation = String(job.flagProcessCreation ?? "")

    const timeout = Number(job.timeout)
    form.timeout = Number.isFinite(timeout) && timeout > 0 ? timeout : 0
    form.concurrencyPolicy = job.concurrencyPolicy ? String(job.concurrencyPolicy) : "skip"
    form.enabled = !!job.enabled
    const mcf = Number(job.maxConsecutiveFailures)
    form.maxConsecutiveFailures = Number.isFinite(mcf) && mcf > 0 ? mcf : 3
    markFormClean()
    return true
  }

  function resetForm() {
    saveDraftNow()
    selectedJobId.value = ""
    editorVisible.value = true
    form.id = ""
    form.name = ""
    form.folder = ""
    form.cron = "0 * * * *"
    form.command = ""
    form.args = [""]
    form.workDir = ""
    form.flagProcessCreation = ""
    form.timeout = 0
    form.concurrencyPolicy = "skip"
    form.enabled = true
    form.maxConsecutiveFailures = 3
    logs.value = []
    markFormClean()
    return true
  }

  async function editJob(job) {
    if (!loadJobToForm(job)) {
      return
    }
    await loadLogs(job.id)
  }

  async function selectJob(jobId) {
    const id = typeof jobId === "string" ? jobId : ""

    saveDraftNow()
    selectedJobId.value = id
    editorVisible.value = false
    if (!id) {
      logs.value = []
      return true
    }
    await loadLogs(id)
    return true
  }

  async function saveJob() {
    error.value = ""
    try {
      const args = Array.isArray(form.args) ? form.args.filter((s) => s !== "") : []

      const savedRaw = await callCronT(5000, "UpsertJob", {
        id: form.id,
        name: form.name,
        folder: form.folder,
        cron: form.cron,
        command: form.command,
        args,
        workDir: form.workDir,
        flagProcessCreation: String(form.flagProcessCreation ?? ""),
        timeout: Number(form.timeout) || 0,
        concurrencyPolicy: form.concurrencyPolicy,
        enabled: form.enabled,
        maxConsecutiveFailures: Number(form.maxConsecutiveFailures) || 3,
      })

      const saved = normalizeObjectResult(savedRaw)
      if (!saved?.id) {
        throw new Error(t("errors.failed_to_save_job"))
      }

      await refreshJobs()
      clearDraft()
      loadJobToForm(saved)
      await loadLogs(saved.id)
      dismissToast()
      triggerEditorPulse("success")
    } catch (e) {
      error.value = String(e)
      dismissToast()
      triggerEditorPulse("error")
    }
  }

  async function deleteJob(id) {
    error.value = ""
    try {
      const jobId = typeof id === "string" ? id : ""
      if (!jobId) {
        return
      }
      if (pendingDeleteJobs.has(jobId)) {
        return
      }

      const deleting = Array.isArray(jobs.value) ? jobs.value.find((j) => String(j?.id || "") === jobId) : null
      const deletingName = String(deleting?.name || "").trim() || String(deleting?.command || "").trim() || jobId

      const wasSelected = selectedJobId.value === jobId

      jobs.value = Array.isArray(jobs.value) ? jobs.value.filter((j) => String(j?.id || "") !== jobId) : []
      if (wasSelected) {
        resetForm()
        logs.value = []
      }

      const undo = async () => {
        const pending = pendingDeleteJobs.get(jobId)
        if (!pending) {
          return
        }
        clearTimeout(pending.timer)
        pendingDeleteJobs.delete(jobId)
        await refreshJobs()
        if (wasSelected) {
          const restored = Array.isArray(jobs.value) ? jobs.value.find((j) => String(j?.id || "") === jobId) : null
          if (restored) {
            loadJobToForm(restored)
            await loadLogs(jobId)
          }
        }
        showToast(t("toast.delete_undone"), "success")
      }

      showToast(t("toast.deleted_with_name", { name: deletingName }), "info", {
        actionLabel: t("common.undo"),
        onAction: undo,
        durationMs: 5000,
      })

      const timer = setTimeout(async () => {
        pendingDeleteJobs.delete(jobId)
        try {
          await callCronT(5000, "DeleteJob", jobId)
        } catch (e) {
          reportError(e)
        }
        await refreshJobs()
      }, 5000)

      pendingDeleteJobs.set(jobId, { timer })
    } catch (e) {
      error.value = String(e)
    }
  }

  async function toggleJob(job) {
    error.value = ""
    try {
      const updatedRaw = await callCronT(5000, "SetJobEnabled", job.id, !job.enabled)
      const updated = normalizeObjectResult(updatedRaw)
      if (!updated?.id) {
        throw new Error(t("errors.failed_to_update_job"))
      }
      await refreshJobs()
      if (selectedJobId.value === updated.id && !isFormDirty()) {
        loadJobToForm(updated)
      }
    } catch (e) {
      error.value = String(e)
    }
  }

  async function setJobsEnabled(jobIds, enabled) {
    error.value = ""
    try {
      const ids = Array.isArray(jobIds) ? jobIds.map((v) => String(v || "")).filter((v) => v) : []
      if (!ids.length) {
        return
      }
      const v = !!enabled
      for (const id of ids) {
        const updatedRaw = await callCronT(5000, "SetJobEnabled", id, v)
        const updated = normalizeObjectResult(updatedRaw)
        if (!updated?.id) {
          throw new Error(t("errors.failed_to_update_job"))
        }
      }

      await refreshJobs()
      const sid = selectedJobId.value
      if (sid && ids.includes(sid) && !isFormDirty()) {
        const job = Array.isArray(jobs.value) ? jobs.value.find((j) => String(j?.id || "") === sid) : null
        if (job) {
          loadJobToForm(job)
        }
      }
    } catch (e) {
      error.value = String(e)
    }
  }

  async function runNow(jobId) {
    error.value = ""
    try {
      const job = Array.isArray(jobs.value) ? jobs.value.find((j) => j?.id === jobId) : null
      const jobTimeout = Number(job?.timeout)
      const timeoutMs = Number.isFinite(jobTimeout) && jobTimeout > 0 ? jobTimeout * 1000 + 5000 : 0
      const entryRaw = await callCronT(timeoutMs > 0 ? Math.max(60000, timeoutMs) : 0, "RunNow", jobId)
      const entry = normalizeObjectResult(entryRaw)
      if (!entry) {
        throw new Error(t("errors.failed_to_run_job"))
      }
      if (!selectedJobId.value || selectedJobId.value === jobId) {
        logs.value = [...logs.value, entry]
      }
    } catch (e) {
      reportError(e)
    }
  }

  async function runPreviewFromForm() {
    error.value = ""
    try {
      const args = Array.isArray(form.args) ? form.args.filter((s) => s !== "") : []
      const previewTimeout = Number(form.timeout)
      const timeoutMs = Number.isFinite(previewTimeout) && previewTimeout > 0 ? previewTimeout * 1000 + 5000 : 0
      const entryRaw = await callCronT(timeoutMs > 0 ? Math.max(60000, timeoutMs) : 0, "RunPreview", {
        command: form.command,
        args,
        workDir: form.workDir,
        flagProcessCreation: String(form.flagProcessCreation ?? ""),
        timeout: Number(form.timeout) || 0,
        jobId: form.id,
        jobName: form.name,
      })
      const entry = normalizeObjectResult(entryRaw)
      if (!entry) {
        throw new Error(t("errors.failed_to_run_preview"))
      }
      logs.value = [...logs.value, entry]
    } catch (e) {
      reportError(e)
    }
  }

  async function loadLogs(jobId) {
    try {
      const result = await callCronT(5000, "ListLogs", jobId, 100)
      logs.value = normalizeArrayResult(result)
    } catch (e) {
      logs.value = []
      reportError(e)
    }
  }

  async function clearLogs() {
    error.value = ""
    showToast(t("toast.clearing"), "info")
    try {
      await callCronT(5000, "ClearLogs")
      logs.value = []
      showToast(t("toast.cleared"), "success")
    } catch (e) {
      reportError(e)
    }
  }

  async function resetAll() {
    error.value = ""
    showToast(t("toast.clearing"), "info")
    try {
      await callCronT(5000, "ResetAll")
      clearDraft()
      resetForm()
      logs.value = []
      await refreshJobs()
      showToast(t("toast.cleared"), "success")
    } catch (e) {
      reportError(e)
    }
  }

  async function exportConfig(options = {}) {
    error.value = ""
    showToast(t("toast.exporting"), "info")
    try {
      const exportJobs = options.exportJobs == null ? true : !!options.exportJobs
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

      const path = await callConfigT(5000, "ExportYAMLToFile", filePath, exportJobs, exportSettings, onlyEnabled)
      showToast(path ? t("toast.exported_with_path", { path }) : t("toast.exported"), "success")
    } catch (e) {
      reportError(e, { rethrow: true })
    }
  }

  async function checkImportConflicts(text) {
    const conflictsRaw = await callConfigT(5000, "CheckImportYAMLConflicts", text)
    return normalizeStringArrayResult(conflictsRaw)
  }

  async function importConfig(text, conflictStrategy = "coexist") {
    error.value = ""
    showToast(t("toast.importing"), "info")
    try {
      const strategy = conflictStrategy === "overwrite" ? "overwrite" : "coexist"
      await callConfigT(5000, "ImportYAML", text, strategy)
      clearDraft()
      resetForm()
      logs.value = []
      await refreshJobs()
      await loadGlobalEnabled()
      await loadSettings()
      showToast(t("toast.imported"), "success")
    } catch (e) {
      reportError(e, { rethrow: true })
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
        if (job && !isFormDirty()) {
          loadJobToForm(job)
        }
      }

      if (selectedJobId.value && entry.jobId === selectedJobId.value) {
        await loadLogs(selectedJobId.value)
      }
    })

    promptDraftRecovery()
  }

  function dispose() {
    if (offJobExecuted) {
      offJobExecuted()
      offJobExecuted = null
    }
    dismissToast()
    if (pendingDeleteJobs.size) {
      for (const pending of pendingDeleteJobs.values()) {
        if (pending?.timer) {
          clearTimeout(pending.timer)
        }
      }
      pendingDeleteJobs.clear()
    }
    if (draftSaveTimer) {
      clearTimeout(draftSaveTimer)
      draftSaveTimer = null
    }
  }

  return {
    error,
    toast,
    toastKind,
    toastActionLabel,
    editorPulse,
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
    isFormDirty,
    restoreDraft,
    promptDraftRecovery,
    flushDraft,
    refreshJobs,
    loadJobToForm,
    editJob,
    selectJob,
    resetForm,
    saveJob,
    copyJob,
    deleteJob,
    setJobFolder,
    setJobsFolder,
    toggleJob,
    setJobsEnabled,
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
    openEnvironmentVariables,
    init,
    triggerToastAction,
    dispose,
  }
})
