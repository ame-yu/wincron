import { computed, ref } from "vue"
import { normalizeJobIds } from "../ui/selectionTokens.js"

const MIME_FOLDER = "application/x-wincron-folder"
const MIME_JOB = "application/x-wincron-job"
const MIME_JOBS = "application/x-wincron-jobs"
const TEXT_FOLDER_PREFIX = "wincron-folder:"
const TEXT_JOB_PREFIX = "wincron-job:"

const createEmptyPayload = () => ({
  kind: "",
  folderName: "",
  jobIds: [],
  primaryJobId: "",
})

const clonePayload = (payload) => ({
  kind: payload?.kind || "",
  folderName: String(payload?.folderName || ""),
  jobIds: Array.isArray(payload?.jobIds) ? [...payload.jobIds] : [],
  primaryJobId: String(payload?.primaryJobId || ""),
})

const getTransferTypes = (dt) => (Array.isArray(dt?.types) ? dt.types : Array.from(dt?.types || []))

const setTransferData = (dt, type, value) => {
  try {
    dt?.setData?.(type, value)
  } catch {}
}

export function useDragTransfer(options = {}) {
  const normalizeFolderName =
    typeof options?.normalizeFolderName === "function" ? options.normalizeFolderName : (v) => String(v || "").trim()

  const activeDrag = ref(createEmptyPayload())

  const isDragging = computed(() => !!activeDrag.value.kind)

  function clearDragState() {
    activeDrag.value = createEmptyPayload()
  }

  function createFolderPayload(folderName) {
    const name = normalizeFolderName(folderName)
    return name ? { kind: "folder", folderName: name, jobIds: [], primaryJobId: "" } : createEmptyPayload()
  }

  function createJobsPayload(jobIds, primaryJobId = "") {
    const ids = normalizeJobIds(jobIds)
    const fallbackPrimary = String(primaryJobId || ids[0] || "")
    const primary = ids.includes(fallbackPrimary) ? fallbackPrimary : ids[0] || ""
    return ids.length && primary ? { kind: "jobs", folderName: "", jobIds: ids, primaryJobId: primary } : createEmptyPayload()
  }

  function readDataTransfer(dt) {
    if (!dt) return createEmptyPayload()

    const types = getTransferTypes(dt)
    if (types.includes(MIME_FOLDER)) {
      return createFolderPayload(dt.getData(MIME_FOLDER))
    }
    if (types.includes(MIME_JOBS)) {
      try {
        const ids = JSON.parse(dt.getData(MIME_JOBS) || "[]")
        const payload = createJobsPayload(ids, dt.getData(MIME_JOB) || "")
        if (payload.kind) return payload
      } catch {}
    }
    if (types.includes(MIME_JOB)) {
      const id = dt.getData(MIME_JOB) || ""
      const payload = createJobsPayload([id], id)
      if (payload.kind) return payload
    }

    const raw = dt.getData("text/plain") || ""
    if (raw.startsWith(TEXT_FOLDER_PREFIX)) {
      return createFolderPayload(raw.slice(TEXT_FOLDER_PREFIX.length))
    }
    if (raw.startsWith(TEXT_JOB_PREFIX)) {
      const id = raw.slice(TEXT_JOB_PREFIX.length)
      const payload = createJobsPayload([id], id)
      if (payload.kind) return payload
    }

    const fallback = createJobsPayload([raw], raw)
    return fallback.kind ? fallback : createEmptyPayload()
  }

  function getDragPayload(e) {
    const payload = readDataTransfer(e?.dataTransfer)
    return payload.kind ? payload : clonePayload(activeDrag.value)
  }

  function getDragJobIds(e) {
    const payload = getDragPayload(e)
    return payload.kind === "jobs" ? payload.jobIds : []
  }

  function getDragJobId(e) {
    const payload = getDragPayload(e)
    return payload.kind === "jobs" ? payload.primaryJobId || payload.jobIds[0] || "" : ""
  }

  function getDragFolderName(e) {
    const payload = getDragPayload(e)
    return payload.kind === "folder" ? normalizeFolderName(payload.folderName) : ""
  }

  function onFolderDragStart(e, folderName) {
    const payload = createFolderPayload(folderName)
    if (!payload.kind) return

    activeDrag.value = payload
    if (e?.dataTransfer) {
      e.dataTransfer.effectAllowed = "move"
      setTransferData(e.dataTransfer, MIME_FOLDER, payload.folderName)
      setTransferData(e.dataTransfer, "text/plain", `${TEXT_FOLDER_PREFIX}${payload.folderName}`)
    }
  }

  function onDragStart(e, jobId, jobIds = [jobId]) {
    const payload = createJobsPayload(jobIds, jobId)
    if (!payload.kind) return

    activeDrag.value = payload
    if (e?.dataTransfer) {
      e.dataTransfer.effectAllowed = "move"
      setTransferData(e.dataTransfer, MIME_JOB, payload.primaryJobId)
      setTransferData(e.dataTransfer, MIME_JOBS, JSON.stringify(payload.jobIds))
      setTransferData(e.dataTransfer, "text/plain", `${TEXT_JOB_PREFIX}${payload.primaryJobId}`)
    }
  }

  return {
    activeDrag,
    isDragging,
    clearDragState,
    getDragPayload,
    getDragJobIds,
    getDragJobId,
    getDragFolderName,
    onFolderDragStart,
    onDragStart,
  }
}
