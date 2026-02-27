<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { storeToRefs } from "pinia"
import { useI18n } from "vue-i18n"
import { useCronStore } from "../stores/cron.js"
import { useDialogs } from "../composables/useDialogs.js"
import { useDragTransfer } from "../composables/useDragTransfer.js"
import { getMenuPosition } from "../ui/menuPosition.js"
import { formatDateTime } from "../ui/datetime.js"
import AppScrollbar from "./AppScrollbar.vue"
import SplitMenuButton from "./SplitMenuButton.vue"
import JobCardItem from "./JobCardItem.vue"
import ModalShell from "./ModalShell.vue"

const props = defineProps({
  btn: { type: String, required: true },
  btnPrimary: { type: String, required: true },
  btnDanger: { type: String, required: true },
})

const cron = useCronStore()
const { jobs, jobsLoaded, selectedJobId } = storeToRefs(cron)

const { t } = useI18n()

const contextVisible = ref(false)
const contextJob = ref(null)
const contextKind = ref("job")
const contextFolder = ref("")
const contextX = ref(0)
const contextY = ref(0)

const hotkeyDialogVisible = ref(false)
const hotkeyDialogJob = ref(null)
const hotkeyDialogValue = ref("")
const hotkeyInputRef = ref(null)

const hotkeyDialogValueTrimmed = computed(() => String(hotkeyDialogValue.value || "").trim())
const isHotkeyWaiting = computed(() => hotkeyDialogVisible.value && !hotkeyDialogValueTrimmed.value)

function focusHotkeyInput() {
  hotkeyInputRef.value?.focus?.()
  requestAnimationFrame(() => hotkeyInputRef.value?.focus?.())
}

function normalizeFolderName(v) {
  return String(v || "").trim()
}

function readLocalStorageJson(key, fallback) {
  try {
    const raw = localStorage.getItem(key)
    return raw ? JSON.parse(raw) : fallback
  } catch {
    return fallback
  }
}

function removeJobsFromRootOrder(jobIds) {
  const ids = Array.isArray(jobIds) ? jobIds.map((v) => String(v || "")).filter((v) => v) : []
  if (!ids.length) return

  const current = normalizeStringList(rootOrder.value)
  const remove = new Set(ids.map((id) => `job:${id}`))
  if (![...remove].some((t) => current.includes(t))) return
  persistRootOrder(current.filter((t) => !remove.has(t)))
}

function ensureJobsInRootOrder(jobIds) {
  const ids = Array.isArray(jobIds) ? jobIds.map((v) => String(v || "")).filter((v) => v) : []
  if (!ids.length) return

  const current = normalizeStringList(rootOrder.value)
  const add = ids.map((id) => `job:${id}`)
  const next = normalizeStringList([...current, ...add])
  if (next.length !== current.length || next.some((t, i) => t !== current[i])) {
    persistRootOrder(next)
  }
}

function writeLocalStorageJson(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {}
}

function normalizeStringList(list) {
  const seen = new Set()
  const out = []
  const arr = Array.isArray(list) ? list : []
  for (const raw of arr) {
    const s = String(raw || "")
    if (!s || seen.has(s)) continue
    seen.add(s)
    out.push(s)
  }
  return out
}

const selectedTokens = ref([])

function tokenForJob(jobId) {
  const id = String(jobId || "")
  return id ? `job:${id}` : ""
}

function tokenForFolder(folderName) {
  const name = normalizeFolderName(folderName)
  return name ? `folder:${name}` : ""
}

function setSelectedTokens(next) {
  selectedTokens.value = normalizeStringList(next)
}

function isTokenSelected(token) {
  return !!token && selectedTokens.value.includes(token)
}

function toggleTokenSelected(token) {
  if (!token) return
  const current = selectedTokens.value
  if (current.includes(token)) {
    setSelectedTokens(current.filter((t) => t !== token))
    return
  }
  setSelectedTokens([...current, token])
}

function isJobSelected(jobId) {
  return isTokenSelected(tokenForJob(jobId))
}

function isFolderSelected(folderName) {
  return isTokenSelected(tokenForFolder(folderName))
}

const { clearDragState, getDragJobId, getDragFolderName, onFolderDragStart, onDragStart: baseOnDragStart } = useDragTransfer({ normalizeFolderName })

function getDragJobIds(e) {
  const dt = e?.dataTransfer
  if (dt) {
    const types = Array.isArray(dt.types) ? dt.types : Array.from(dt.types || [])
    if (types.includes("application/x-wincron-jobs")) {
      try {
        const raw = dt.getData("application/x-wincron-jobs")
        const parsed = raw ? JSON.parse(raw) : []
        const ids = Array.isArray(parsed) ? parsed.map((v) => String(v || "")).filter((v) => v) : []
        if (ids.length) return normalizeStringList(ids)
      } catch {}
    }
  }

  const single = getDragJobId(e)
  return single ? [single] : []
}

function onJobDragStart(e, jobId) {
  const id = typeof jobId === "string" ? jobId : ""
  if (!id) return

  const token = tokenForJob(id)
  const multi = isTokenSelected(token) && selectedTokens.value.length > 1
  const ids = multi ? getSelectedJobIdsExpanded() : [id]

  baseOnDragStart(e, id)
  if (e?.dataTransfer) {
    try {
      e.dataTransfer.setData("application/x-wincron-jobs", JSON.stringify(ids))
    } catch {}
  }
}

const jobOrderStorageKey = "wincron.jobOrder"
const storedJobOrder = normalizeStringList(readLocalStorageJson(jobOrderStorageKey, []))

const jobOrder = ref(storedJobOrder)

function persistJobOrder(next) {
  jobOrder.value = next
  writeLocalStorageJson(jobOrderStorageKey, next)
}

const rootOrderStorageKey = "wincron.rootOrder"
const storedRootOrder = normalizeStringList(readLocalStorageJson(rootOrderStorageKey, []))

const rootOrder = ref(storedRootOrder)

function persistRootOrder(next) {
  rootOrder.value = next
  writeLocalStorageJson(rootOrderStorageKey, next)
}

const folderStorageKey = "wincron.folders"
const storedFolders = (() => {
  const v = readLocalStorageJson(folderStorageKey, [])
  return Array.isArray(v) ? v.filter((s) => typeof s === "string") : []
})()

const folders = ref(storedFolders)
const folderOpen = ref({})

function clearJobsStorageCache() {
  jobOrder.value = []
  rootOrder.value = []
  folders.value = []
  folderOpen.value = {}
  setSelectedTokens([])
  ;[jobOrderStorageKey, rootOrderStorageKey, folderStorageKey].forEach((key) => {
    try {
      localStorage.removeItem(key)
    } catch {}
  })
}

const {
  textDialogVisible,
  textDialogTitle,
  textDialogLabel,
  textDialogValue,
  textDialogInput,
  openTextDialog,
  closeTextDialog,
  confirmDialogVisible,
  confirmDialogTitle,
  confirmDialogMessage,
  confirmDialogDanger,
  openConfirmDialog,
  closeConfirmDialog,
} = useDialogs()

watch(
  jobs,
  (list) => {
    const jobList = Array.isArray(list) ? list : []
    if (!jobList.length) {
      if (!jobsLoaded.value) {
        return
      }
      clearJobsStorageCache()
      return
    }
    const allIds = new Set(jobList.map((j) => String(j?.id || "")).filter((id) => id))

    const current = normalizeStringList(jobOrder.value)
    const kept = current.filter((id) => allIds.has(id))

    const missing = jobList
      .map((j) => String(j?.id || ""))
      .filter((id) => id && !kept.includes(id))

    const next = [...kept, ...missing]
    if (next.length !== current.length || next.some((id, i) => id !== current[i])) {
      persistJobOrder(next)
    }
  },
  { immediate: true },
)

function persistFolders(next) {
  folders.value = next
  writeLocalStorageJson(folderStorageKey, next)
}

function ensureFolder(name) {
  const n = normalizeFolderName(name)
  if (!n) return ""
  const list = Array.isArray(folders.value) ? folders.value : []
  if (!list.includes(n)) {
    persistFolders([...list, n])
  }
  return n
}

function isFolderOpen(name) {
  return !!folderOpen.value?.[name]
}

function toggleFolder(name) {
  folderOpen.value = { ...folderOpen.value, [name]: !isFolderOpen(name) }
}

function onFolderClick(e, folderName) {
  if (e && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    e.stopPropagation()
    toggleTokenSelected(tokenForFolder(folderName))
    focusLogsFromSelection()
    return
  }
  toggleFolder(folderName)
}

async function createFolder() {
  const raw = await openTextDialog({
    title: t("main.folders.new_folder"),
    label: t("main.folders.prompt_name"),
  })
  const name = normalizeFolderName(raw)
  if (!name) return
  ensureFolder(name)
  folderOpen.value = { ...folderOpen.value, [name]: true }
}

onMounted(() => {
  window.addEventListener("wincron:new-folder", createFolder)
  window.addEventListener("wincron:clear-selection", clearSelection)
})

onBeforeUnmount(() => {
  window.removeEventListener("wincron:new-folder", createFolder)
  window.removeEventListener("wincron:clear-selection", clearSelection)
})

const createMenuItems = computed(() => [
  { key: "job", label: t("main.folders.new_job"), default: true },
  { key: "folder", label: t("main.folders.new_folder") },
])

function clearSelection() {
  setSelectedTokens([])
  cron.selectJob("")
}

function onCreateSelect(key) {
  if (key === "folder") {
    createFolder()
    return
  }
  cron.resetForm()
}

const sortedJobs = computed(() => {
  const list = Array.isArray(jobs.value) ? [...jobs.value] : []
  const order = normalizeStringList(jobOrder.value)
  const index = new Map(order.map((id, i) => [id, i]))
  list.sort((a, b) => {
    const ai = index.get(String(a?.id || ""))
    const bi = index.get(String(b?.id || ""))
    const aHas = typeof ai === "number"
    const bHas = typeof bi === "number"
    if (aHas && bHas && ai !== bi) return ai - bi
    if (aHas !== bHas) return aHas ? -1 : 1
    const aid = String(a?.id || "")
    const bid = String(b?.id || "")
    return aid.localeCompare(bid)
  })
  return list
})

const folderNames = computed(() => {
  const set = new Set()
  for (const f of Array.isArray(folders.value) ? folders.value : []) {
    const n = normalizeFolderName(f)
    if (n) set.add(n)
  }
  for (const j of Array.isArray(jobs.value) ? jobs.value : []) {
    const n = normalizeFolderName(j?.folder)
    if (n) set.add(n)
  }
  return [...set]
})

const jobsGrouped = computed(() => {
  const by = {}
  for (const name of folderNames.value) {
    by[name] = []
  }
  const unfiled = []
  for (const j of sortedJobs.value) {
    const f = normalizeFolderName(j?.folder)
    if (f) {
      if (!by[f]) by[f] = []
      by[f].push(j)
    } else {
      unfiled.push(j)
    }
  }
  return { by, unfiled }
})

watch(
  [folderNames, () => jobsGrouped.value.unfiled],
  ([foldersList, unfiledJobs]) => {
    const folderTokens = (Array.isArray(foldersList) ? foldersList : []).map((n) => `folder:${normalizeFolderName(n)}`).filter((t) => t !== "folder:")
    const unfiledIds = (Array.isArray(unfiledJobs) ? unfiledJobs : []).map((j) => String(j?.id || "")).filter((id) => id)
    const jobTokens = unfiledIds.map((id) => `job:${id}`)

    if (!folderTokens.length && !jobTokens.length) {
      return
    }

    const jobList = Array.isArray(jobs.value) ? jobs.value : []
    const current = normalizeStringList(rootOrder.value)
    if (!jobList.length && current.some((t) => t.startsWith("job:"))) {
      return
    }

    const folderSet = new Set(folderTokens)
    const jobSet = new Set(jobTokens)

    const kept = current.filter((t) => folderSet.has(t) || jobSet.has(t))

    const missingFolders = folderTokens.filter((t) => !kept.includes(t))
    const missingJobs = jobTokens.filter((t) => !kept.includes(t))

    const next = [...kept, ...missingFolders, ...missingJobs]
    if (next.length !== current.length || next.some((t, i) => t !== current[i])) {
      persistRootOrder(next)
    }
  },
  { immediate: true },
)

const folderItems = computed(() => {
  return folderNames.value.map((name) => ({ type: "folder", name, jobs: jobsGrouped.value.by[name] || [] }))
})

const displayItems = computed(() => {
  const jobItems = jobsGrouped.value.unfiled.map((job) => ({ type: "job", job }))
  const folderMap = new Map(folderItems.value.map((it) => [normalizeFolderName(it.name), it]))
  const jobMap = new Map(jobItems.map((it) => [String(it.job?.id || ""), it]))

  const tokens = normalizeStringList(rootOrder.value)
  const out = []
  for (const token of tokens) {
    if (token.startsWith("folder:")) {
      const name = normalizeFolderName(token.slice("folder:".length))
      const item = folderMap.get(name)
      if (item) out.push(item)
    } else if (token.startsWith("job:")) {
      const id = token.slice("job:".length)
      const item = jobMap.get(id)
      if (item) out.push(item)
    }
  }

  const includedFolders = new Set(out.filter((it) => it.type === "folder").map((it) => normalizeFolderName(it.name)))
  const includedJobs = new Set(out.filter((it) => it.type === "job").map((it) => String(it.job?.id || "")))

  for (const it of folderItems.value) {
    const name = normalizeFolderName(it.name)
    if (name && !includedFolders.has(name)) out.push(it)
  }
  for (const it of jobItems) {
    const id = String(it.job?.id || "")
    if (id && !includedJobs.has(id)) out.push(it)
  }

  return out
})

watch(
  [jobs, folderNames],
  ([jobList, foldersList]) => {
    const ids = new Set((Array.isArray(jobList) ? jobList : []).map((j) => String(j?.id || "")).filter((id) => id))
    const foldersSet = new Set((Array.isArray(foldersList) ? foldersList : []).map((n) => normalizeFolderName(n)).filter((n) => n))

    const next = selectedTokens.value.filter((t) => {
      if (t.startsWith("job:")) return ids.has(t.slice("job:".length))
      if (t.startsWith("folder:")) return foldersSet.has(normalizeFolderName(t.slice("folder:".length)))
      return false
    })

    if (next.length !== selectedTokens.value.length || next.some((t, i) => t !== selectedTokens.value[i])) {
      setSelectedTokens(next)
    }
  },
  { immediate: true },
)

function focusLogsFromSelection() {
  const lastJob = [...selectedTokens.value].reverse().find((t) => t.startsWith("job:"))
  cron.focusLogs(lastJob ? lastJob.slice("job:".length) : "")
}

function onJobSelect(e, jobId) {
  const token = tokenForJob(jobId)
  if (!token) return
  if (e && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    e.stopPropagation()
    toggleTokenSelected(token)
    focusLogsFromSelection()
    return
  }
  setSelectedTokens([token])
  cron.selectJob(String(jobId || ""))
}

async function selectSingleJob(jobId) {
  const token = tokenForJob(jobId)
  if (!token) return false
  setSelectedTokens([token])
  await cron.selectJob(String(jobId || ""))
  return true
}

async function withSingleSelection(job, fn) {
  if (!job) return
  await selectSingleJob(job.id)
  await fn(job)
}

const onJobActionEdit = (job) => withSingleSelection(job, cron.editJob)
const onJobActionToggle = (job) => withSingleSelection(job, cron.toggleJob)
const onJobActionRun = (job) => withSingleSelection(job, (j) => cron.runNow(j.id))

async function onDropToFolderCardBlank(e, folderName) {
  const draggedFolder = getDragFolderName(e)
  if (draggedFolder) {
    onDropToFolderOrder(e, folderName)
    clearDragState()
    return
  }
  await onDropToFolder(e, folderName)
}

async function onDropToFolder(e, folderName) {
  const draggedFolder = getDragFolderName(e)
  if (draggedFolder) {
    onDropToFolderOrder(e, folderName)
    clearDragState()
    return
  }
  const ids = getDragJobIds(e)
  if (!ids.length) return
  const f = ensureFolder(folderName)
  await cron.setJobsFolder(ids, f)
  folderOpen.value = { ...folderOpen.value, [folderName]: true }
  removeJobsFromRootOrder(ids)
  clearDragState()
}

function onDropToFolderOrder(e, folderName) {
  const dragged = getDragFolderName(e)
  const target = normalizeFolderName(folderName)
  if (!dragged || !target || dragged === target) return

  ensureFolder(dragged)
  ensureFolder(target)

  const dragToken = `folder:${normalizeFolderName(dragged)}`
  const targetToken = `folder:${normalizeFolderName(target)}`
  const base = normalizeStringList(rootOrder.value)
  const ensured = base.slice()
  if (!ensured.includes(dragToken)) ensured.push(dragToken)
  if (!ensured.includes(targetToken)) ensured.push(targetToken)

  const without = ensured.filter((t) => t !== dragToken)
  const targetIndexWithout = without.indexOf(targetToken)
  if (targetIndexWithout < 0) {
    return
  }
  const at = targetIndexWithout
  persistRootOrder([...without.slice(0, at), dragToken, ...without.slice(at)])
}

async function onDropToUnfiled(e) {
  const ids = getDragJobIds(e)
  if (!ids.length) return
  await cron.setJobsFolder(ids, "")
  ensureJobsInRootOrder(ids)
  clearDragState()
}

async function onDropToJob(e, targetJobId) {
  const draggedFolderName = getDragFolderName(e)
  const draggedIds = getDragJobIds(e)
  const draggedId = draggedIds.length === 1 ? draggedIds[0] : ""
  const targetId = typeof targetJobId === "string" ? targetJobId : ""

  if (draggedFolderName) {
    if (!targetId) return
    const jobList = Array.isArray(jobs.value) ? jobs.value : []
    const target = jobList.find((j) => String(j?.id || "") === targetId)
    const targetFolder = normalizeFolderName(target?.folder)
    if (targetFolder) return

    const dragToken = `folder:${normalizeFolderName(draggedFolderName)}`
    const targetToken = `job:${targetId}`
    const base = normalizeStringList(rootOrder.value)
    const dragIndex = base.indexOf(dragToken)
    const targetIndex = base.indexOf(targetToken)
    if (dragIndex >= 0 && targetIndex >= 0 && dragToken !== targetToken) {
      const without = base.filter((t) => t !== dragToken)
      const targetIndexWithout = without.indexOf(targetToken)
      const insertBelow = dragIndex < targetIndex
      const at = insertBelow ? targetIndexWithout + 1 : targetIndexWithout
      persistRootOrder([...without.slice(0, at), dragToken, ...without.slice(at)])
    }
    clearDragState()
    return
  }

  if (draggedIds.length > 1) {
    if (!targetId) return
    const jobList = Array.isArray(jobs.value) ? jobs.value : []
    const target = jobList.find((j) => String(j?.id || "") === targetId)
    if (!target) return

    const targetFolder = normalizeFolderName(target?.folder)
    await cron.setJobsFolder(draggedIds, targetFolder)
    if (targetFolder) {
      removeJobsFromRootOrder(draggedIds)
    } else {
      ensureJobsInRootOrder(draggedIds)
    }

    clearDragState()
    return
  }

  if (!draggedId || !targetId || draggedId === targetId) return

  const jobList = Array.isArray(jobs.value) ? jobs.value : []
  const dragged = jobList.find((j) => String(j?.id || "") === draggedId)
  const target = jobList.find((j) => String(j?.id || "") === targetId)
  if (!target) return

  const draggedFolder = normalizeFolderName(dragged?.folder)
  const targetFolder = normalizeFolderName(target?.folder)
  if (draggedFolder !== targetFolder) {
    await cron.setJobFolder(draggedId, targetFolder)
  }

  const baseOrder = normalizeStringList(jobOrder.value)

  const draggedIndex = baseOrder.indexOf(draggedId)
  const targetIndex = baseOrder.indexOf(targetId)

  const without = baseOrder.filter((id) => id !== draggedId)
  const targetIndexWithout = without.indexOf(targetId)
  if (targetIndexWithout < 0) {
    return
  }

  let insertBelow = draggedIndex >= 0 && targetIndex >= 0 ? draggedIndex < targetIndex : true
  if (!targetFolder) {
    const base = normalizeStringList(rootOrder.value)
    const dragToken = `job:${draggedId}`
    const targetToken = `job:${targetId}`
    const di = base.indexOf(dragToken)
    const ti = base.indexOf(targetToken)
    if (di >= 0 && ti >= 0) {
      insertBelow = di < ti
    }
  }
  const at = insertBelow ? targetIndexWithout + 1 : targetIndexWithout
  const next = normalizeStringList([...without.slice(0, at), draggedId, ...without.slice(at)])

  persistJobOrder(next)
  const current = normalizeStringList(rootOrder.value)
  const dragToken = `job:${draggedId}`
  const targetToken = `job:${targetId}`

  if (targetFolder) {
    if (current.includes(dragToken)) {
      persistRootOrder(current.filter((t) => t !== dragToken))
    }
  } else {
    const base = current.includes(dragToken) ? current : [...current, dragToken]
    const dragIndex = base.indexOf(dragToken)
    const targetIndex = base.indexOf(targetToken)
    if (targetIndex >= 0 && dragIndex >= 0 && dragToken !== targetToken) {
      const without = base.filter((t) => t !== dragToken)
      const targetIndexWithout = without.indexOf(targetToken)
      const insertBelowRoot = dragIndex < targetIndex
      const at = insertBelowRoot ? targetIndexWithout + 1 : targetIndexWithout
      persistRootOrder([...without.slice(0, at), dragToken, ...without.slice(at)])
    } else {
      persistRootOrder(base)
    }
  }
  clearDragState()
}

const folderCardClass = "rounded-xl border border-slate-200 bg-white p-3 data-[selected=true]:border-blue-600/45 data-[selected=true]:ring-4 data-[selected=true]:ring-blue-600/10"

const formatJobNextRun = (job) => {
  return formatDateTime(job?.nextRunAt)
}

function editJob(job) {
  cron.editJob(job)
}

function jobCardProps(job, inFolder) {
  return {
    job,
    selected: isJobSelected(job?.id),
    inFolder,
    btn: props.btn,
    btnPrimary: props.btnPrimary,
    btnDanger: props.btnDanger,
    formatNextRun: formatJobNextRun,
  }
}

const jobCardListeners = {
  dragstart: onJobDragStart,
  dragend: clearDragState,
  drop: onDropToJob,
  select: onJobSelect,
  edit: onJobActionEdit,
  toggle: onJobActionToggle,
  run: onJobActionRun,
  delete: cron.deleteJob,
  contextmenu: openContextMenu,
}

function getFolderJobIds(name) {
  const n = normalizeFolderName(name)
  const list = jobsGrouped.value.by[n] || []
  return Array.isArray(list) ? list.map((j) => String(j?.id || "")).filter((id) => id) : []
}

async function renameFolder(oldName) {
  const current = normalizeFolderName(oldName)
  if (!current) {
    return
  }
  const raw = await openTextDialog({
    title: t("main.folders.rename"),
    label: t("main.folders.rename_prompt"),
    initial: current,
  })
  const next = normalizeFolderName(raw)
  if (!next || next === current) {
    return
  }

  const list = Array.isArray(folders.value) ? folders.value : []
  const remaining = list.filter((n) => normalizeFolderName(n) !== current)
  if (!remaining.includes(next)) {
    remaining.push(next)
  }
  persistFolders(remaining)

  const wasOpen = isFolderOpen(current)
  const nextOpen = { ...(folderOpen.value || {}) }
  delete nextOpen[current]
  if (wasOpen) {
    nextOpen[next] = true
  }
  folderOpen.value = nextOpen

  const ids = getFolderJobIds(current)
  await cron.setJobsFolder(ids, next)
}

async function deleteFolder(name) {
  const f = normalizeFolderName(name)
  if (!f) {
    return
  }

  const ids = getFolderJobIds(f)

  if (ids.length > 0) {
    const ok = await openConfirmDialog({
      title: t("main.folders.delete_folder"),
      message: t("main.folders.delete_confirm", { name: f }),
      danger: true,
    })
    if (!ok) {
      return
    }
  }

  const list = Array.isArray(folders.value) ? folders.value : []
  persistFolders(list.filter((n) => normalizeFolderName(n) !== f))

  const nextOpen = { ...(folderOpen.value || {}) }
  delete nextOpen[f]
  folderOpen.value = nextOpen

  if (ids.length > 0) {
    await cron.setJobsFolder(ids, "")
  }
}

function closeContextMenu() {
  contextVisible.value = false
  contextJob.value = null
  contextKind.value = "job"
  contextFolder.value = ""
}

function openMenuAt(e, menuHeight) {
  const pos = getMenuPosition(e, { menuWidth: 220, menuHeight, padding: 8 })
  contextX.value = pos.x
  contextY.value = pos.y
  contextVisible.value = true
}

const isBulkContext = computed(() => selectedTokens.value.length > 1)

function getSelectedFolderNames() {
  return selectedTokens.value
    .filter((t) => t.startsWith("folder:"))
    .map((t) => normalizeFolderName(t.slice("folder:".length)))
    .filter((n) => n)
}

function getSelectedDirectJobIds() {
  return selectedTokens.value
    .filter((t) => t.startsWith("job:"))
    .map((t) => String(t.slice("job:".length) || ""))
    .filter((id) => id)
}

function getSelectedJobIdsExpanded() {
  const out = new Set(getSelectedDirectJobIds())
  for (const name of getSelectedFolderNames()) {
    for (const id of getFolderJobIds(name)) {
      out.add(id)
    }
  }
  return [...out]
}

async function onContextEnableSelected() {
  const ids = getSelectedJobIdsExpanded()
  closeContextMenu()
  await cron.setJobsEnabled(ids, true)
}

async function onContextDisableSelected() {
  const ids = getSelectedJobIdsExpanded()
  closeContextMenu()
  await cron.setJobsEnabled(ids, false)
}

async function onContextDeleteSelected() {
  const jobIds = getSelectedDirectJobIds()
  const foldersToDelete = getSelectedFolderNames()
  closeContextMenu()

  setSelectedTokens([])
  for (const id of jobIds) {
    cron.deleteJob(id)
  }
  for (const name of foldersToDelete) {
    await deleteFolder(name)
  }
}

function openContextMenu(e, job) {
  contextKind.value = "job"
  contextFolder.value = ""
  const token = tokenForJob(job?.id)

  if (e && (e.ctrlKey || e.metaKey)) {
    toggleTokenSelected(token)
  } else {
    if (!isTokenSelected(token)) {
      setSelectedTokens([token])
    }
  }

  contextJob.value = job
  focusLogsFromSelection()
  openMenuAt(e, isBulkContext.value ? 160 : 240)
}

function openFolderContextMenu(e, folderName) {
  const f = normalizeFolderName(folderName)
  if (!f) return

  contextKind.value = "folder"
  contextFolder.value = f
  contextJob.value = null

  const token = tokenForFolder(f)
  if (e && (e.ctrlKey || e.metaKey)) {
    toggleTokenSelected(token)
  } else {
    if (!isTokenSelected(token)) {
      setSelectedTokens([token])
    }
  }

  focusLogsFromSelection()
  openMenuAt(e, isBulkContext.value ? 160 : 200)
}

function onContextEdit() {
  if (!contextJob.value) return
  editJob(contextJob.value)
  closeContextMenu()
}

function onContextToggle() {
  if (!contextJob.value) return
  cron.toggleJob(contextJob.value)
  closeContextMenu()
}

function onContextCopy() {
  if (!contextJob.value) return
  cron.copyJob(contextJob.value)
  closeContextMenu()
}

async function onContextBindHotkey() {
  if (!contextJob.value) return
  const job = contextJob.value
  const existingHotkey = String(job?.hotkey || "")
  const shouldAutoRecord = !existingHotkey.trim()
  hotkeyDialogJob.value = job
  hotkeyDialogValue.value = existingHotkey
  closeContextMenu()
  await cron.pauseHotkeys()
  hotkeyDialogVisible.value = true
  await nextTick()
  if (shouldAutoRecord) {
    focusHotkeyInput()
  }
}

function closeHotkeyDialog() {
  hotkeyDialogVisible.value = false
  hotkeyDialogJob.value = null
  hotkeyDialogValue.value = ""
  cron.resumeHotkeys()
}

function clearHotkeyDialogValue() {
  hotkeyDialogValue.value = ""
}

function onHotkeyInputClick() {
  if (!hotkeyDialogValueTrimmed.value) {
    return
  }
  hotkeyDialogValue.value = ""
  nextTick(() => {
    focusHotkeyInput()
  })
}

function normalizeHotkeyKey(key) {
  const k = String(key || "")
  if (!k) return ""
  if (k === " ") return "Space"
  const map = {
    Escape: "Esc",
    Enter: "Enter",
    Tab: "Tab",
    Backspace: "Backspace",
    Delete: "Del",
    Insert: "Ins",
    Home: "Home",
    End: "End",
    PageUp: "PgUp",
    PageDown: "PgDn",
    ArrowUp: "Up",
    ArrowDown: "Down",
    ArrowLeft: "Left",
    ArrowRight: "Right",
  }
  if (map[k]) return map[k]
  if (/^F\d{1,2}$/i.test(k)) return k.toUpperCase()
  if (k.length === 1) return k.toUpperCase()
  return ""
}

function normalizeHotkeyEventKey(e) {
  const code = String(e?.code || "")
  if (code === "Space") return "Space"
  if (/^Key[A-Z]$/.test(code)) return code.slice(3)
  if (/^Digit[0-9]$/.test(code)) return code.slice(5)
  if (/^Numpad[0-9]$/.test(code)) return code.slice(6)
  return normalizeHotkeyKey(e?.key)
}

function onHotkeyKeyDown(e) {
  if (!e) return
  e.preventDefault()
  e.stopPropagation()

  const key = String(e.key || "")
  if (key === "Control" || key === "Shift" || key === "Alt" || key === "Meta") {
    return
  }

  const mods = []
  if (e.ctrlKey) mods.push("Ctrl")
  if (e.altKey) mods.push("Alt")
  if (e.shiftKey) mods.push("Shift")
  if (e.metaKey) mods.push("Win")

  if (!mods.length) {
    return
  }

  const k = normalizeHotkeyEventKey(e)
  if (!k) {
    return
  }

  hotkeyDialogValue.value = [...mods, k].join("+")
}

async function saveHotkey() {
  const job = hotkeyDialogJob.value
  if (!job) return
  const raw = String(hotkeyDialogValue.value || "")
  const normalized = raw ? await cron.validateJobHotkey(raw) : ""
  await cron.setJobHotkey(job.id, normalized)
  closeHotkeyDialog()
}

function onContextDelete() {
  if (!contextJob.value) return
  cron.deleteJob(contextJob.value.id)
  closeContextMenu()
}

async function onContextRenameFolder() {
  const name = contextFolder.value
  closeContextMenu()
  await renameFolder(name)
}

async function onContextEnableFolder() {
  const name = contextFolder.value
  closeContextMenu()
  const ids = getFolderJobIds(name)
  await cron.setJobsEnabled(ids, true)
}

async function onContextDisableFolder() {
  const name = contextFolder.value
  closeContextMenu()
  const ids = getFolderJobIds(name)
  await cron.setJobsEnabled(ids, false)
}

async function onContextDeleteFolder() {
  const name = contextFolder.value
  closeContextMenu()
  await deleteFolder(name)
}
</script>

<template>
  <aside class="w-full rounded-2xl border border-slate-200 bg-white p-3 shadow-[0_10px_30px_rgba(2,6,23,0.08)] sm:p-3.5 lg:w-[380px] lg:self-stretch lg:h-full lg:max-h-full flex flex-col">
    <div class="flex items-start justify-between gap-3 px-3 pt-3 pb-2">
      <div>
        <h2>{{ $t("main.jobs.title") }}</h2>
        <div class="mt-0.5 text-xs text-slate-500">{{ $t("main.jobs.subtitle") }}</div>
      </div>
      <div class="flex shrink-0 items-center gap-2 whitespace-nowrap">
        <SplitMenuButton
          :btn-primary="btnPrimary"
          :primary-label="$t('common.new')"
          :menu-items="createMenuItems"
          @primary="cron.resetForm"
          @select="onCreateSelect"
        />
      </div>
    </div>

    <AppScrollbar
      root-class="flex flex-col flex-1 min-h-0"
      wrap-class="min-h-0"
      view-class="flex flex-col gap-2.5 px-2.5 pb-2.5"
      @dragover.prevent
      @drop.prevent="onDropToUnfiled"
    >
      <template v-for="item in displayItems" :key="item.type === 'folder' ? `folder:${item.name}` : item.job.id">
        <div
          v-if="item.type === 'folder'"
          :class="folderCardClass"
          data-wincron-keep-selection="1"
          :data-selected="isFolderSelected(item.name)"
          @dragover.prevent
          @drop.prevent.stop="onDropToFolderCardBlank($event, item.name)"
          @contextmenu.prevent.stop="openFolderContextMenu($event, item.name)"
        >
          <button
            type="button"
            class="flex w-full items-center justify-between gap-2 text-left text-xs active:cursor-grabbing"
            draggable="true"
            @dragstart="onFolderDragStart($event, item.name)"
            @dragend="clearDragState"
            @click="onFolderClick($event, item.name)"
            @dragover.prevent.stop
            @drop.prevent.stop="onDropToFolder($event, item.name)"
          >
            <div class="flex min-w-0 items-center gap-2">
              <span class="text-slate-500">{{ isFolderOpen(item.name) ? "📂" : "📁" }}</span>
              <span class="min-w-0 truncate font-semibold text-slate-900">{{ item.name }}</span>
            </div>
            <span class="text-slate-500">{{ item.jobs.length }}</span>
          </button>

          <div v-if="isFolderOpen(item.name)" class="mt-2 flex flex-col gap-2">
            <JobCardItem
              v-for="job in item.jobs"
              :key="job.id"
              data-wincron-keep-selection="1"
              v-bind="jobCardProps(job, true)"
              v-on="jobCardListeners"
            />
          </div>
        </div>

        <JobCardItem
          v-else
          data-wincron-keep-selection="1"
          v-bind="jobCardProps(item.job, false)"
          v-on="jobCardListeners"
        />
      </template>

      <div v-if="!jobs.length" class="p-2.5 text-sm text-slate-500">{{ $t("main.jobs.empty") }}</div>
    </AppScrollbar>

    <ModalShell v-model="textDialogVisible" :max-width="520" @close="closeTextDialog('')">
      <div>
        <h3>{{ textDialogTitle }}</h3>
      </div>

      <div class="mt-3">
        <label class="text-xs text-slate-500">{{ textDialogLabel }}</label>
        <input
          ref="textDialogInput"
          v-model="textDialogValue"
          class="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2 text-sm outline-none focus:border-slate-400"
          type="text"
          @keydown.enter.prevent="closeTextDialog(textDialogValue)"
          @keydown.esc.prevent="closeTextDialog('')"
        />
      </div>

      <div class="mt-4 flex justify-end gap-2">
        <button :class="btn" type="button" @click="closeTextDialog('')">{{ $t("common.cancel") }}</button>
        <button :class="btnPrimary" type="button" @click="closeTextDialog(textDialogValue)">{{ $t("common.ok") }}</button>
      </div>
    </ModalShell>

    <ModalShell v-model="confirmDialogVisible" :max-width="520" @close="closeConfirmDialog(false)">
      <div>
        <h3>{{ confirmDialogTitle }}</h3>
      </div>

      <div class="mt-3 whitespace-pre-line text-sm text-slate-600">
        {{ confirmDialogMessage }}
      </div>

      <div class="mt-4 flex justify-end gap-2">
        <button :class="btn" type="button" @click="closeConfirmDialog(false)">{{ $t("common.cancel") }}</button>
        <button :class="confirmDialogDanger ? btnDanger : btnPrimary" type="button" @click="closeConfirmDialog(true)">{{ $t("common.ok") }}</button>
      </div>
    </ModalShell>

    <teleport to="body">
      <div v-if="contextVisible" data-wincron-keep-selection="1" class="fixed inset-0 z-40" @click="closeContextMenu" @contextmenu.prevent="closeContextMenu" />
      <div
        v-if="contextVisible"
        data-wincron-keep-selection="1"
        class="fixed z-50 w-[220px] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.18)]"
        :style="{ left: contextX + 'px', top: contextY + 'px' }"
      >
        <template v-if="isBulkContext">
          <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-green-800 hover:bg-green-50" @click="onContextEnableSelected">
            <span>{{ $t("main.folders.enable_all") }}</span>
          </button>
          <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-slate-600 hover:bg-slate-50" @click="onContextDisableSelected">
            <span>{{ $t("main.folders.disable_all") }}</span>
          </button>
          <div class="h-px bg-slate-200/70" />
          <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-rose-700 hover:bg-rose-50" @click="onContextDeleteSelected">
            <span>{{ $t("common.delete") }}</span>
          </button>
        </template>

        <template v-else-if="contextKind === 'folder'">
          <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextRenameFolder">
            <span>{{ $t("main.folders.rename") }}</span>
          </button>
          <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-green-800 hover:bg-green-50" @click="onContextEnableFolder">
            <span>{{ $t("main.folders.enable_all") }}</span>
          </button>
          <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-slate-600 hover:bg-slate-50" @click="onContextDisableFolder">
            <span>{{ $t("main.folders.disable_all") }}</span>
          </button>
          <div class="h-px bg-slate-200/70" />
          <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-rose-700 hover:bg-rose-50" @click="onContextDeleteFolder">
            <span>{{ $t("main.folders.delete_folder") }}</span>
          </button>
        </template>

        <template v-else>
          <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextEdit">
            <span>{{ $t("common.edit") }}</span>
          </button>
          <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextToggle">
            <span>{{ contextJob?.enabled ? $t("common.disable") : $t("common.enable") }}</span>
          </button>
          <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextCopy">
            <span>{{ $t("common.copy") }}</span>
          </button>
          <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextBindHotkey">
            <span>{{ $t("main.context.bind_hotkey") }}</span>
            <span v-if="contextJob?.hotkey" class="text-slate-500">{{ contextJob.hotkey }}</span>
          </button>
          <div class="h-px bg-slate-200/70" />
          <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-rose-700 hover:bg-rose-50" @click="onContextDelete">
            <span>{{ $t("common.delete") }}</span>
          </button>
        </template>
      </div>
    </teleport>

    <ModalShell v-model="hotkeyDialogVisible" :max-width="520" @close="closeHotkeyDialog">
      <div>
        <h3>{{ $t("main.context.bind_hotkey") }}</h3>
      </div>

      <div class="mt-3 hotkey-input-wrap" :data-waiting="isHotkeyWaiting">
        <label class="text-xs text-slate-500">{{ hotkeyDialogJob?.name || hotkeyDialogJob?.command || hotkeyDialogJob?.id }}</label>
        <div class="relative mt-2">
          <input
            ref="hotkeyInputRef"
            v-model="hotkeyDialogValue"
            class="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm outline-none focus:border-slate-400 hotkey-record-input"
            type="text"
            :placeholder="$t('main.placeholders.hotkey')"
            readonly
            @click="onHotkeyInputClick"
            @keydown="onHotkeyKeyDown"
          />
          <span class="hotkey-wait-dots pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-slate-400" aria-hidden="true" />
        </div>
      </div>

      <div class="mt-4 flex justify-end gap-2">
        <button :class="btn" type="button" @click="clearHotkeyDialogValue">{{ $t("common.clear") }}</button>
        <button :class="btn" type="button" @click="closeHotkeyDialog">{{ $t("common.cancel") }}</button>
        <button :class="btnPrimary" type="button" @click="saveHotkey">{{ $t("common.save") }}</button>
      </div>
    </ModalShell>
  </aside>
</template>

<style scoped>
.hotkey-wait-dots {
  display: none;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}

.hotkey-wait-dots::after {
  content: "......";
  display: inline-block;
  overflow: hidden;
  width: 1ch;
  animation: hotkey-wait-dots 2000ms steps(6, end) infinite;
}

.hotkey-input-wrap[data-waiting="true"] .hotkey-wait-dots {
  display: inline-block;
}

.hotkey-input-wrap[data-waiting="true"] .hotkey-record-input::placeholder {
  color: transparent;
}

@keyframes hotkey-wait-dots {
  from {
    width: 1ch;
  }
  to {
    width: 6ch;
  }
}
</style>
