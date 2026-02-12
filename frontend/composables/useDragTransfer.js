import { ref } from "vue"

export function useDragTransfer(options = {}) {
  const normalizeFolderName =
    typeof options?.normalizeFolderName === "function" ? options.normalizeFolderName : (v) => String(v || "").trim()

  const activeDragJobId = ref("")
  const activeDragFolder = ref("")

  function clearDragState() {
    activeDragJobId.value = ""
    activeDragFolder.value = ""
  }

  function getDragJobId(e) {
    const dt = e?.dataTransfer
    if (!dt) return activeDragJobId.value || ""
    const types = Array.isArray(dt.types) ? dt.types : Array.from(dt.types || [])
    if (types.includes("application/x-wincron-folder")) {
      return ""
    }
    if (types.includes("application/x-wincron-job")) {
      return dt.getData("application/x-wincron-job") || activeDragJobId.value || ""
    }

    const raw = dt.getData("text/plain") || ""
    if (raw.startsWith("wincron-folder:")) {
      return ""
    }
    if (raw.startsWith("wincron-job:")) {
      return raw.slice("wincron-job:".length)
    }
    return raw || activeDragJobId.value || ""
  }

  function getDragFolderName(e) {
    const dt = e?.dataTransfer
    if (!dt) return normalizeFolderName(activeDragFolder.value)
    const types = Array.isArray(dt.types) ? dt.types : Array.from(dt.types || [])
    if (types.includes("application/x-wincron-folder")) {
      return normalizeFolderName(dt.getData("application/x-wincron-folder")) || normalizeFolderName(activeDragFolder.value)
    }

    const raw = dt.getData("text/plain") || ""
    if (raw.startsWith("wincron-folder:")) {
      return normalizeFolderName(raw.slice("wincron-folder:".length))
    }
    return normalizeFolderName(activeDragFolder.value)
  }

  function onFolderDragStart(e, folderName) {
    const name = normalizeFolderName(folderName)
    if (!name) return
    activeDragFolder.value = name
    activeDragJobId.value = ""
    if (e?.dataTransfer) {
      e.dataTransfer.effectAllowed = "move"
      try {
        e.dataTransfer.setData("application/x-wincron-folder", name)
      } catch {}
      try {
        e.dataTransfer.setData("text/plain", `wincron-folder:${name}`)
      } catch {}
    }
  }

  function onDragStart(e, jobId) {
    const id = typeof jobId === "string" ? jobId : ""
    if (!id) return
    activeDragJobId.value = id
    activeDragFolder.value = ""
    if (e?.dataTransfer) {
      e.dataTransfer.effectAllowed = "move"
      try {
        e.dataTransfer.setData("application/x-wincron-job", id)
      } catch {}
      try {
        e.dataTransfer.setData("text/plain", `wincron-job:${id}`)
      } catch {}
    }
  }

  return {
    activeDragJobId,
    activeDragFolder,
    clearDragState,
    getDragJobId,
    getDragFolderName,
    onFolderDragStart,
    onDragStart,
  }
}
