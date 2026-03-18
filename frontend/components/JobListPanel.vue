<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { storeToRefs } from "pinia"
import { useI18n } from "vue-i18n"
import { useCronStore } from "../stores/cron.js"
import { useDialogs } from "../composables/useDialogs.js"
import { useFolderManager } from "../composables/useFolderManager.js"
import { useJobListDrag } from "../composables/useJobListDrag.js"
import { formatDateTime } from "../ui/datetime.js"
import { btnPrimaryOutline } from "../ui/buttonClasses.js"
import { getLastSelectedJobId, getSelectedJobIdsExpanded } from "../ui/selectionTokens.js"
import AppScrollbar from "./AppScrollbar.vue"
import SplitMenuButton from "./SplitMenuButton.vue"
import JobCardItem from "./JobCardItem.vue"
import ModalShell from "./ModalShell.vue"
import ContextMenu from "./ContextMenu.vue"

const props = defineProps({
  btn: { type: String, required: true },
  btnPrimary: { type: String, required: true },
  btnDanger: { type: String, required: true },
})

const cron = useCronStore()
const { jobs, runningJobIds, selectedJobId, logFocusJobId, editorVisible } = storeToRefs(cron)

const { t } = useI18n()

const dialogs = useDialogs()
const {
  textDialogVisible,
  textDialogTitle,
  textDialogLabel,
  textDialogValue,
  textDialogInput,
  closeTextDialog,
  confirmDialogVisible,
  confirmDialogTitle,
  confirmDialogMessage,
  confirmDialogDanger,
  closeConfirmDialog,
} = dialogs

const folderManager = useFolderManager({ i18n: { t }, dialogs })
const {
  folderNames,
  folderItems,
  jobsGrouped,
  normalizeFolder,
  normalizeList,
  tokenFor,
  getFolderJobIds,
  isFolderOpen,
  toggleFolder,
  createFolder,
  renameFolder,
  deleteFolder,
  rootOrder,
} = folderManager

const contextMenuRef = ref(null)
const panelRef = ref(null)
const selectedTokens = ref([])
const flashJobId = ref("")
let flashTimer = null

const toJobToken = (id) => tokenFor("job", id)
const toFolderToken = (name) => tokenFor("folder", name)
const isMetaSelect = (event) => !!(event?.ctrlKey || event?.metaKey)
const sameList = (a, b) => a.length === b.length && a.every((value, index) => value === b[index])
const setSelected = (list) => { selectedTokens.value = normalizeList(list) }
const isSelected = (token) => !!token && selectedTokens.value.includes(token)
const toggleSelected = (token) => {
  if (!token) return
  setSelected(isSelected(token) ? selectedTokens.value.filter((value) => value !== token) : [...selectedTokens.value, token])
}
const isJobSelected = (id) => isSelected(toJobToken(id))
const isFolderSelected = (name) => isSelected(toFolderToken(name))
const focusLogsFromSelection = () => { cron.focusLogs(getLastSelectedJobId(selectedTokens.value)) }
const syncContextSelection = () => { contextMenuRef.value?.setSelection(selectedTokens, getFolderJobIds) }

const { clearDragState, isDragging, onFolderDragStart, onJobDragStart, onDropToFolder, onDropToJob, onDropToUnfiled } =
  useJobListDrag({
    cron,
    jobs,
    folderManager,
    isSelected,
    getSelectedJobIdsExpanded: () => getSelectedJobIdsExpanded(selectedTokens.value, getFolderJobIds, normalizeFolder),
  })

function onFolderClick(event, folderName) {
  if (isMetaSelect(event)) {
    event.preventDefault()
    event.stopPropagation()
    toggleSelected(toFolderToken(folderName))
    focusLogsFromSelection()
    return
  }
  toggleFolder(folderName)
}

async function clearSelection() {
  if (!selectedTokens.value.length && !selectedJobId.value && !logFocusJobId.value && !editorVisible.value) {
    return
  }
  setSelected([])
  await cron.selectJob("")
}

function clearFlashTimer() {
  if (!flashTimer) {
    return
  }
  clearTimeout(flashTimer)
  flashTimer = null
}

function flashJob(jobId) {
  const id = String(jobId || "").trim()
  if (!id) return
  flashJobId.value = id
  clearFlashTimer()
  flashTimer = setTimeout(() => {
    if (flashJobId.value === id) flashJobId.value = ""
    flashTimer = null
  }, 1800)
}

function findJobCardElement(jobId) {
  return [...(panelRef.value?.querySelectorAll("[data-job-id]") || [])].find((element) => element?.dataset?.jobId === jobId) || null
}

async function revealJobCard(event) {
  const jobId = String(event?.detail?.jobId || "")
  if (!jobId) return

  const job = (Array.isArray(jobs.value) ? jobs.value : []).find((item) => String(item?.id || "") === jobId)
  const folderName = normalizeFolder(job?.folder)
  if (folderName && !isFolderOpen(folderName)) {
    toggleFolder(folderName)
  }

  await nextTick()
  const card = findJobCardElement(jobId)
  if (card) {
    card.scrollIntoView({ behavior: "smooth", block: "nearest" })
    flashJob(jobId)
  }
}

const windowEvents = [
  ["wincron:new-folder", createFolder],
  ["wincron:clear-selection", clearSelection],
  ["wincron:reveal-job", revealJobCard],
]

onMounted(() => {
  windowEvents.forEach(([name, handler]) => window.addEventListener(name, handler))
  syncContextSelection()
})

onBeforeUnmount(() => {
  windowEvents.forEach(([name, handler]) => window.removeEventListener(name, handler))
  clearFlashTimer()
})

const createMenuItems = computed(() => [
  { key: "job", label: t("main.folders.new_job"), default: true },
  { key: "folder", label: t("main.folders.new_folder") },
])

const onCreateSelect = (key) => (key === "folder" ? createFolder() : cron.resetForm())

const displayItems = computed(() => {
  const jobItems = jobsGrouped.value.unfiled.map((job) => ({ type: "job", job }))
  const tokenMap = new Map([
    ...folderItems.value.map((item) => [toFolderToken(item.name), item]),
    ...jobItems.map((item) => [toJobToken(item.job?.id), item]),
  ])
  const ordered = normalizeList(rootOrder.value).map((token) => tokenMap.get(token)).filter(Boolean)
  const seen = new Set(ordered.map((item) => (item.type === "folder" ? toFolderToken(item.name) : toJobToken(item.job?.id))))
  return [
    ...ordered,
    ...folderItems.value.filter((item) => !seen.has(toFolderToken(item.name))),
    ...jobItems.filter((item) => !seen.has(toJobToken(item.job?.id))),
  ]
})

const runningJobIdSet = computed(() => new Set((runningJobIds.value || []).map((id) => String(id || "")).filter(Boolean)))

watch(selectedTokens, syncContextSelection)

watch(selectedJobId, (value) => {
  const token = toJobToken(value)
  if (token && !sameList(selectedTokens.value, [token])) setSelected([token])
}, { immediate: true })

watch(
  [jobs, folderNames],
  ([jobList, foldersList]) => {
    const ids = new Set((jobList || []).map((job) => String(job?.id || "")).filter(Boolean))
    const foldersSet = new Set((foldersList || []).map((name) => normalizeFolder(name)).filter(Boolean))
    const next = selectedTokens.value.filter((token) => {
      if (token.startsWith("job:")) return ids.has(token.slice(4))
      if (token.startsWith("folder:")) return foldersSet.has(normalizeFolder(token.slice(7)))
      return false
    })
    if (!sameList(selectedTokens.value, next)) setSelected(next)
  },
  { immediate: true },
)

function onJobSelect(event, jobId) {
  const token = toJobToken(jobId)
  if (!token) return
  if (isMetaSelect(event)) {
    event.preventDefault()
    event.stopPropagation()
    toggleSelected(token)
    focusLogsFromSelection()
    return
  }
  setSelected([token])
  cron.selectJob(String(jobId || ""))
}

async function selectSingleJob(jobId) {
  const token = toJobToken(jobId)
  if (!token) return false
  setSelected([token])
  await cron.selectJob(String(jobId || ""))
  return true
}

async function withSingleSelection(job, fn) {
  if (!job) return
  if (await selectSingleJob(job.id)) await fn(job)
}

const onJobActionToggle = (job) => withSingleSelection(job, cron.toggleJob)
const onJobActionRun = (job) => withSingleSelection(job, (currentJob) => cron.runNow(currentJob.id))
const onJobActionTerminate = (job) => withSingleSelection(job, (currentJob) => cron.terminateRunningJob(currentJob.id))

const folderCardClass =
  "rounded-xl border border-slate-200 bg-white p-3 data-[selected=true]:border-blue-600/45 data-[selected=true]:ring-4 data-[selected=true]:ring-blue-600/10"

const formatJobNextRun = (job) => formatDateTime(job?.nextRunAt)

function jobCardProps(job, inFolder) {
  return {
    job,
    running: runningJobIdSet.value.has(String(job?.id || "")),
    selected: isJobSelected(job?.id),
    inFolder,
    btn: props.btn,
    btnPrimary: props.btnPrimary,
    btnDanger: props.btnDanger,
    formatNextRun: formatJobNextRun,
  }
}

function openSelectionMenu(event, token, payload) {
  if (!token) return
  if (isMetaSelect(event)) toggleSelected(token)
  else if (!isSelected(token)) setSelected([token])
  focusLogsFromSelection()
  contextMenuRef.value?.openMenu(event, payload)
}

function openContextMenu(event, job) {
  openSelectionMenu(event, toJobToken(job?.id), { kind: "job", job })
}

const jobCardListeners = {
  dragstart: onJobDragStart,
  dragend: clearDragState,
  drop: onDropToJob,
  select: onJobSelect,
  edit: (job) => cron.editJob(job),
  toggle: onJobActionToggle,
  run: onJobActionRun,
  terminate: onJobActionTerminate,
  delete: cron.deleteJob,
  contextmenu: openContextMenu,
}

function openFolderContextMenu(event, folderName) {
  const name = normalizeFolder(folderName)
  if (!name) return
  openSelectionMenu(event, toFolderToken(name), { kind: "folder", folder: name })
}

const onContextMenuFolderRename = (name) => renameFolder(name)
const onContextMenuFolderDelete = async (name) => { await deleteFolder(name) }

</script>

<template>
  <aside ref="panelRef" class="w-full min-h-0 max-h-[40vh] rounded-2xl border border-slate-200 bg-white p-2.5 shadow-[0_10px_30px_rgba(2,6,23,0.08)] sm:max-h-[45vh] sm:p-1 md:max-h-full md:w-[340px] md:self-stretch md:h-full md:p-2 lg:w-[380px] lg:self-stretch lg:h-full lg:max-h-full flex flex-col">
    <div class="flex flex-wrap items-center justify-between gap-2 px-2 pb-1.5 sm:gap-3 sm:px-3 sm:pb-2">
      <div>
        <h2 class="text-sm sm:text-base">{{ $t("main.jobs.title") }}</h2>
      </div>
      <div class="flex shrink-0 flex-wrap items-center gap-2">
        <SplitMenuButton
          :btn-primary="btnPrimaryOutline"
          compact
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
      view-class="flex flex-col gap-2 px-2 pb-2 sm:gap-2.5 sm:px-2.5 sm:pb-2.5"
      :is-dragging="isDragging"
      @drop.prevent="onDropToUnfiled"
    >
      <template v-for="item in displayItems" :key="item.type === 'folder' ? `folder:${item.name}` : item.job.id">
        <div
          v-if="item.type === 'folder'"
          :class="folderCardClass"
          :data-selected="isFolderSelected(item.name)"
          @dragover.prevent
          @drop.prevent.stop="onDropToFolder($event, item.name)"
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
              v-bind="jobCardProps(job, true)"
              :data-job-id="job.id"
              :data-flash="flashJobId === String(job.id)"
              v-on="jobCardListeners"
            />
          </div>
        </div>

        <JobCardItem
          v-else
          v-bind="jobCardProps(item.job, false)"
          :data-job-id="item.job.id"
          :data-flash="flashJobId === String(item.job.id)"
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

    <ContextMenu
      ref="contextMenuRef"
      :btn="btn"
      :btn-primary="btnPrimary"
      @folder-rename="onContextMenuFolderRename"
      @folder-delete="onContextMenuFolderDelete"
    />
  </aside>
</template>
