import { computed, ref, watch } from "vue"
import { useCronStore } from "../stores/cron.js"
import { normalizeJobIds, normalizeList, tokenFor } from "../ui/selectionTokens.js"
import { useDialogs } from "./useDialogs.js"

const normalizeFolder = (v) => String(v || "").trim()
const readStorage = (key, fallback) => {
  try {
    const r = localStorage.getItem(key)
    return r ? JSON.parse(r) : fallback
  } catch {
    return fallback
  }
}
const writeStorage = (key, v) => {
  try {
    localStorage.setItem(key, JSON.stringify(v))
  } catch {}
}

/**
 * Folder management composable
 * Handles folder state, operations, and persistence
 */
export function useFolderManager(options = {}) {
  const { t } = options.i18n || { t: (key) => key }
  const cron = useCronStore()
  const { openTextDialog, openConfirmDialog } = options.dialogs || useDialogs()

  // State
  const folders = ref((readStorage("wincron.folders", []) || []).filter((s) => typeof s === "string"))
  const folderOpen = ref({})

  // Job order state (shared with parent component)
  const jobOrder = ref(normalizeList(readStorage("wincron.jobOrder", [])))
  const rootOrder = ref(normalizeList(readStorage("wincron.rootOrder", [])))

  // Persistence functions
  const persistFolders = (v) => {
    folders.value = v
    writeStorage("wincron.folders", v)
  }
  const persistJobOrder = (v) => {
    jobOrder.value = v
    writeStorage("wincron.jobOrder", v)
  }
  const persistRootOrder = (v) => {
    rootOrder.value = v
    writeStorage("wincron.rootOrder", v)
  }
  // Folder operations
  const ensureFolder = (name) => {
    const n = normalizeFolder(name)
    if (!n) return ""
    const list = folders.value || []
    if (!list.includes(n)) persistFolders([...list, n])
    return n
  }

  const isFolderOpen = (name) => !!folderOpen.value?.[name]
  const toggleFolder = (name) => {
    folderOpen.value = { ...folderOpen.value, [name]: !isFolderOpen(name) }
  }

  // Computed: all folder names (from folders list + jobs with folder)
  const folderNames = computed(() => {
    const set = new Set()
    for (const f of folders.value || []) {
      const n = normalizeFolder(f)
      if (n) set.add(n)
    }
    for (const j of cron.jobs || []) {
      const n = normalizeFolder(j?.folder)
      if (n) set.add(n)
    }
    return [...set]
  })

  // Computed: jobs grouped by folder
  const sortedJobs = computed(() => {
    const list = Array.isArray(cron.jobs) ? [...cron.jobs] : []
    const order = normalizeList(jobOrder.value)
    const idx = new Map(order.map((id, i) => [id, i]))
    list.sort((a, b) => {
      const ai = idx.get(String(a?.id || "")),
        bi = idx.get(String(b?.id || ""))
      if (ai !== undefined && bi !== undefined) return ai - bi
      if (ai !== undefined) return -1
      if (bi !== undefined) return 1
      return String(a?.id || "").localeCompare(String(b?.id || ""))
    })
    return list
  })

  const jobsGrouped = computed(() => {
    const by = Object.fromEntries(folderNames.value.map((n) => [n, []])),
      unfiled = []
    for (const j of sortedJobs.value) {
      const f = normalizeFolder(j?.folder)
      ;(f ? (by[f] ??= []) : unfiled).push(j)
    }
    return { by, unfiled }
  })

  const folderItems = computed(() =>
    folderNames.value.map((name) => ({
      type: "folder",
      name,
      jobs: jobsGrouped.value.by[name] || [],
    })),
  )

  // Get job IDs in a folder
  const getFolderJobIds = (name) =>
    normalizeJobIds((jobsGrouped.value.by[normalizeFolder(name)] || []).map((j) => String(j?.id || "")))

  // Folder CRUD operations
  async function createFolder() {
    const name = normalizeFolder(
      await openTextDialog({ title: t("main.folders.new_folder"), label: t("main.folders.prompt_name") }),
    )
    if (!name) return
    ensureFolder(name)
    folderOpen.value = { ...folderOpen.value, [name]: true }
  }

  async function renameFolder(oldName) {
    const current = normalizeFolder(oldName)
    if (!current) return
    const next = normalizeFolder(
      await openTextDialog({
        title: t("main.folders.rename"),
        label: t("main.folders.rename_prompt"),
        initial: current,
      }),
    )
    if (!next || next === current) return
    const list = folders.value || []
    persistFolders([...list.filter((n) => normalizeFolder(n) !== current), next])
    const wasOpen = isFolderOpen(current),
      nextOpen = { ...folderOpen.value }
    delete nextOpen[current]
    if (wasOpen) nextOpen[next] = true
    folderOpen.value = nextOpen
    await cron.setJobsFolder(getFolderJobIds(current), next)
  }

  async function deleteFolder(name) {
    const f = normalizeFolder(name)
    if (!f) return
    const ids = getFolderJobIds(f)
    if (
      ids.length &&
      !(await openConfirmDialog({
        title: t("main.folders.delete_folder"),
        message: t("main.folders.delete_confirm", { name: f }),
        danger: true,
      }))
    )
      return
    persistFolders((folders.value || []).filter((n) => normalizeFolder(n) !== f))
    const nextOpen = { ...folderOpen.value }
    delete nextOpen[f]
    folderOpen.value = nextOpen
    if (ids.length) await cron.setJobsFolder(ids, "")
  }

  // Root order management
  const updateRootOrder = (remove, add) => {
    const cur = normalizeList(rootOrder.value)
    const rmSet = new Set(remove || [])
    let next = rmSet.size ? cur.filter((t) => !rmSet.has(t)) : cur
    if (add?.length) next = normalizeList([...next, ...add])
    if (next.length !== cur.length || next.some((t, i) => t !== cur[i])) persistRootOrder(next)
  }

  const removeJobsFromRootOrder = (ids) =>
    updateRootOrder(
      (ids || [])
        .map((id) => `job:${id}`)
        .filter(Boolean),
    )
  const ensureJobsInRootOrder = (ids) =>
    updateRootOrder(
      null,
      (ids || [])
        .map((id) => `job:${id}`)
        .filter(Boolean),
    )

  // Clear cache
  const clearFolderCache = () => {
    jobOrder.value = []
    rootOrder.value = []
    folders.value = []
    folderOpen.value = {}
    ;["wincron.jobOrder", "wincron.rootOrder", "wincron.folders"].forEach((k) => {
      try {
        localStorage.removeItem(k)
      } catch {}
    })
  }

  // Watch jobs to update jobOrder
  watch(
    () => cron.jobs,
    (list) => {
      const arr = Array.isArray(list) ? list : []
      if (!arr.length && cron.jobsLoaded) return clearFolderCache()
      if (!arr.length) return
      const allIds = new Set(
        arr
          .map((j) => String(j?.id || ""))
          .filter(Boolean),
      )
      const cur = normalizeList(jobOrder.value)
      const kept = cur.filter((id) => allIds.has(id))
      const missing = arr
        .map((j) => String(j?.id || ""))
        .filter((id) => id && !kept.includes(id))
      const next = [...kept, ...missing]
      if (next.length !== cur.length || next.some((id, i) => id !== cur[i])) persistJobOrder(next)
    },
    { immediate: true },
  )

  // Watch folderNames and unfiled jobs to update rootOrder
  watch(
    [folderNames, () => jobsGrouped.value.unfiled],
    ([foldersList, unfiledJobs]) => {
      const folderTokens = (foldersList || []).map((n) => tokenFor("folder", n)).filter(Boolean)
      const jobTokens = (unfiledJobs || []).map((j) => tokenFor("job", j?.id)).filter(Boolean)
      if (!folderTokens.length && !jobTokens.length) return
      const jobList = cron.jobs || []
      const cur = normalizeList(rootOrder.value)
      if (!jobList.length && cur.some((t) => t.startsWith("job:"))) return
      const folderSet = new Set(folderTokens),
        jobSet = new Set(jobTokens)
      const kept = cur.filter((t) => folderSet.has(t) || jobSet.has(t))
      const next = [
        ...kept,
        ...folderTokens.filter((t) => !kept.includes(t)),
        ...jobTokens.filter((t) => !kept.includes(t)),
      ]
      if (next.length !== cur.length || next.some((t, i) => t !== cur[i])) persistRootOrder(next)
    },
    { immediate: true },
  )

  return {
    // State
    folders,
    folderOpen,
    jobOrder,
    rootOrder,
    // Computed
    folderNames,
    jobsGrouped,
    folderItems,
    sortedJobs,
    // Helpers
    normalizeFolder,
    normalizeList,
    tokenFor,
    getFolderJobIds,
    // Folder operations
    ensureFolder,
    isFolderOpen,
    toggleFolder,
    createFolder,
    renameFolder,
    deleteFolder,
    // Order operations
    persistJobOrder,
    persistRootOrder,
    updateRootOrder,
    removeJobsFromRootOrder,
    ensureJobsInRootOrder,
    // Cache
    clearFolderCache,
  }
}
