import { reactive, ref, watch } from "vue"

function normalizeMaxConsecutiveFailures(value, fallback = 3) {
  if (value === "") {
    return fallback
  }
  const n = Number(value)
  if (!Number.isFinite(n)) {
    return fallback
  }
  if (n < 0) {
    return 0
  }
  return Math.trunc(n)
}

export function createCronState() {
  const error = ref("")
  const toast = ref("")
  const toastKind = ref("info")
  const toastActionLabel = ref("")
  const editorPulse = ref(null)

  const silentStart = ref(false)
  const lightweightMode = ref(false)
  const autoStart = ref(false)
  const runInTray = ref(true)
  const globalEnabled = ref(true)

  const jobs = ref([])
  const jobsLoaded = ref(false)
  const runningJobIds = ref([])
  const selectedJobId = ref("")
  const logFocusJobId = ref("")
  const logs = ref([])
  const logsLoading = ref(false)
  const logsLoadingMore = ref(false)
  const logsHasMore = ref(false)
  const logsTotalCount = ref(0)
  const editorVisible = ref(false)

  const form = reactive({
    id: "",
    name: "",
    folder: "",
    cron: "0 * * * *",
    command: "",
    args: [""],
    workDir: "",
    inheritEnv: false,
    flagProcessCreation: "",
    timeout: 0,
    concurrencyPolicy: "skip",
    enabled: true,
    maxConsecutiveFailures: 3,
  })

  let formBaseline = ""
  const formDirty = ref(false)
  let toastTimer = null
  let toastAction = null
  let toastOnDismiss = null
  let offJobStarted = null
  let offJobExecuted = null
  const pendingDeleteJobs = new Map()

  const getFormSnapshot = () =>
    JSON.stringify({
      id: String(form.id || ""),
      name: String(form.name ?? ""),
      folder: String(form.folder ?? ""),
      cron: String(form.cron ?? ""),
      command: String(form.command ?? ""),
      args: Array.isArray(form.args) ? form.args.map((s) => String(s ?? "")).filter((s) => s !== "") : [],
      workDir: String(form.workDir ?? ""),
      inheritEnv: form.inheritEnv !== false,
      flagProcessCreation: String(form.flagProcessCreation ?? ""),
      timeout: Number(form.timeout) || 0,
      concurrencyPolicy: String(form.concurrencyPolicy || "skip"),
      enabled: !!form.enabled,
      maxConsecutiveFailures: normalizeMaxConsecutiveFailures(form.maxConsecutiveFailures),
    })

  const setDirtyState = (value) => {
    if (formDirty.value !== !!value) formDirty.value = !!value
  }

  const markFormClean = () => {
    formBaseline = getFormSnapshot()
    setDirtyState(false)
  }

  const isFormDirty = () => !!formDirty.value

  function clearToastTimer() {
    if (toastTimer) {
      clearTimeout(toastTimer)
      toastTimer = null
    }
  }

  function clearToast() {
    toast.value = ""
    toastActionLabel.value = ""
    toastAction = null
    toastOnDismiss = null
  }

  function dismissToast() {
    if (!toast.value) return clearToastTimer()
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
    toastTimer = setTimeout(dismissToast, ms)
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
    if (kind === "success" || kind === "error") {
      editorPulse.value = { kind, ts: Date.now() }
    }
  }

  function setOffJobStarted(disposer) {
    offJobStarted = typeof disposer === "function" ? disposer : null
  }

  function getOffJobStarted() {
    return offJobStarted
  }

  function setOffJobExecuted(disposer) {
    offJobExecuted = typeof disposer === "function" ? disposer : null
  }

  function getOffJobExecuted() {
    return offJobExecuted
  }

  markFormClean()

  watch(
    getFormSnapshot,
    (snapshotStr) => {
      setDirtyState(snapshotStr !== formBaseline)
    },
    { immediate: true },
  )

  return {
    error,
    toast,
    toastKind,
    toastActionLabel,
    editorPulse,
    silentStart,
    lightweightMode,
    autoStart,
    runInTray,
    globalEnabled,
    jobs,
    jobsLoaded,
    runningJobIds,
    selectedJobId,
    logFocusJobId,
    logs,
    logsLoading,
    logsLoadingMore,
    logsHasMore,
    logsTotalCount,
    editorVisible,
    form,
    pendingDeleteJobs,
    normalizeMaxConsecutiveFailures,
    isFormDirty,
    markFormClean,
    dismissToast,
    triggerToastAction,
    showToast,
    reportError,
    triggerEditorPulse,
    setOffJobStarted,
    getOffJobStarted,
    setOffJobExecuted,
    getOffJobExecuted,
  }
}
