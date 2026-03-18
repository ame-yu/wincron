<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { storeToRefs } from "pinia"
import { useI18n } from "vue-i18n"
import { useCronStore } from "../stores/cron.js"
import { getMenuPosition } from "../ui/menuPosition.js"
import { formatDateTime } from "../ui/datetime.js"
import CollapsibleLog from "./CollapsibleLog.vue"

const MAX_REVEAL_STEPS = 20
const LOG_FILTER_KEYS = ["all", "running", "success", "failed"]

const cron = useCronStore()
const { t } = useI18n()
const {
  logs,
  logsLoading,
  logsLoadingMore,
  logsHasMore,
  logsTotalCount,
  editorVisible,
  jobs,
  logFocusJobId,
} = storeToRefs(cron)

const panelRef = ref(null)
const contextVisible = ref(false)
const contextLog = ref(null)
const contextX = ref(0)
const contextY = ref(0)
const panelClearConfirm = ref(false)
const now = ref(Date.now())
const activeFilter = ref("all")
const flashLogId = ref("")
let flashTimer = null
let nowTimer = null
let filterPrefetching = false

const toId = (value) => String(value || "")
const getTimeMs = (value) => {
  const ms = Date.parse(value || "")
  return Number.isFinite(ms) ? ms : 0
}
const resetFilter = () => {
  activeFilter.value = "all"
}
const getLogStatus = (entry) => (isRunning(entry) ? "running" : Number(entry?.exitCode) === 0 ? "success" : "failed")
const sourceLogs = computed(() => (Array.isArray(logs.value) ? logs.value : []))

const sortedLogs = computed(() =>
  [...sourceLogs.value].sort(
    (a, b) => getTimeMs(b?.finishedAt || b?.startedAt) - getTimeMs(a?.finishedAt || a?.startedAt),
  ),
)
const filteredLogs = computed(() =>
  activeFilter.value === "all"
    ? sortedLogs.value
    : sortedLogs.value.filter((entry) => getLogStatus(entry) === activeFilter.value),
)
const loadedCount = computed(() => sourceLogs.value.length)
const visibleCount = computed(() => filteredLogs.value.length)
const needsMoreLogs = computed(() =>
  logsHasMore.value || loadedCount.value < Math.max(Number(logsTotalCount.value) || 0, loadedCount.value),
)
const showInitialLoading = computed(() => logsLoading.value && !loadedCount.value)
const showFilteredEmpty = computed(() => !showInitialLoading.value && !!loadedCount.value && !visibleCount.value)
const hasRunningLogs = computed(() => sourceLogs.value.some((entry) => isRunning(entry)))

const findJobById = (jobId) => (Array.isArray(jobs.value) ? jobs.value : []).find((job) => toId(job?.id) === toId(jobId)) || null
const canEditLogJob = (entry) => !!findJobById(entry?.jobId)
const editLogJob = (entry) => {
  const job = findJobById(entry?.jobId)
  if (!job) return false
  cron.editJob(job)
  return true
}

function isRunning(entry) {
  return !!getTimeMs(entry?.startedAt) && !getTimeMs(entry?.finishedAt)
}

function formatDuration(entry) {
  const startMs = getTimeMs(entry?.startedAt)
  if (!startMs) return ""
  const endMs = entry?.finishedAt ? getTimeMs(entry.finishedAt) : now.value
  return endMs ? `${Math.max(0, (endMs - startMs) / 1000).toFixed(1)}s` : ""
}

function getTriggerSourceLabel(entry) {
  const key = String(entry?.triggerSource || "").trim()
  return ["cron", "ui", "ipc", "hotkey", "preview"].includes(key) ? t(`main.logs.source.${key}`) : ""
}

function getContextMenuHeight(entry) {
  if (!entry) return 48
  return ((editorVisible.value ? 0 : 2) + (isRunning(entry) ? 1 : 2)) * 40
}

const canCopyLogOutput = (entry) => !isRunning(entry) && !!String(entry?.stdout || entry?.stderr || "").trim()

const closeContextMenu = () => {
  contextVisible.value = false
  contextLog.value = null
  panelClearConfirm.value = false
}

const clearFlashTimer = () => {
  if (!flashTimer) return
  clearTimeout(flashTimer)
  flashTimer = null
}

function flashLog(logId) {
  const id = toId(logId)
  if (!id) return
  flashLogId.value = id
  clearFlashTimer()
  flashTimer = setTimeout(() => {
    if (flashLogId.value === id) flashLogId.value = ""
    flashTimer = null
  }, 1800)
}

const findLogElement = (logId) =>
  [...(panelRef.value?.querySelectorAll("[data-log-id]") || [])].find((element) => element?.dataset?.logId === logId) || null

async function revealLogEntry(event) {
  const jobId = toId(event?.detail?.jobId)
  const logId = toId(event?.detail?.logId)
  if (!logId) return

  resetFilter()
  let steps = 0
  while (steps < MAX_REVEAL_STEPS) {
    await nextTick()
    const row = findLogElement(logId)
    if (row) {
      row.scrollIntoView({ behavior: "smooth", block: "center" })
      flashLog(logId)
      return
    }
    if (!needsMoreLogs.value || logsLoading.value || logsLoadingMore.value) {
      return
    }
    const beforeCount = loadedCount.value
    await cron.loadMoreLogs(jobId || logFocusJobId.value || "")
    if (loadedCount.value <= beforeCount) {
      return
    }
    steps += 1
  }
}

async function ensureFilteredLogsVisible() {
  if (filterPrefetching || activeFilter.value === "all") {
    return
  }
  if (filteredLogs.value.length || logsLoading.value || logsLoadingMore.value || !needsMoreLogs.value) {
    return
  }

  filterPrefetching = true
  try {
    let steps = 0
    while (
      steps < MAX_REVEAL_STEPS &&
      activeFilter.value !== "all" &&
      !filteredLogs.value.length &&
      !logsLoading.value &&
      !logsLoadingMore.value &&
      needsMoreLogs.value
    ) {
      const beforeCount = loadedCount.value
      await cron.loadMoreLogs(logFocusJobId.value || "")
      await nextTick()
      if (loadedCount.value <= beforeCount) {
        break
      }
      steps += 1
    }
  } finally {
    filterPrefetching = false
  }
}

function openMenu(e, entry = null) {
  e?.preventDefault?.()
  e?.stopPropagation?.()
  contextLog.value = entry
  panelClearConfirm.value = false
  const pos = getMenuPosition(e, {
    menuWidth: 220,
    menuHeight: getContextMenuHeight(entry),
    padding: 8,
  })
  contextX.value = pos.x
  contextY.value = pos.y
  contextVisible.value = true
}

function withContextLog(fn) {
  const entry = contextLog.value
  if (!entry) return
  fn(entry)
  closeContextMenu()
}

function onContextEditJob() {
  const entry = contextLog.value
  if (!entry || !editLogJob(entry)) return
  closeContextMenu()
}

const onContextShowJobLogs = () => withContextLog((entry) => cron.focusLogs(String(entry.jobId || "")))
const onContextCopyOutput = () => withContextLog((entry) => cron.copyLogOutput(entry))
const onContextTerminateTask = () => withContextLog((entry) => cron.terminateRunningLog(entry))
const onContextDeleteRecord = () => withContextLog((entry) => cron.deleteLogEntry(String(entry.id || "")))

async function onContextClearLogs() {
  if (!logs.value.length) return
  if (!panelClearConfirm.value) return void (panelClearConfirm.value = true)
  await cron.clearLogs()
  closeContextMenu()
}

const startNowTimer = () => {
  if (nowTimer) return
  nowTimer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
}

const stopNowTimer = () => {
  if (!nowTimer) return
  clearInterval(nowTimer)
  nowTimer = null
}

const windowEvents = [
  ["blur", closeContextMenu],
  ["wincron:reveal-log", revealLogEntry],
]

onMounted(() => {
  windowEvents.forEach(([name, handler]) => window.addEventListener(name, handler))
})

onBeforeUnmount(() => {
  windowEvents.forEach(([name, handler]) => window.removeEventListener(name, handler))
  stopNowTimer()
  clearFlashTimer()
})

watch(hasRunningLogs, (running) => {
  if (running) {
    now.value = Date.now()
    startNowTimer()
    return
  }
  stopNowTimer()
}, { immediate: true })

watch(logFocusJobId, () => {
  resetFilter()
})

watch([activeFilter, filteredLogs, logsHasMore, logsLoading, logsLoadingMore], () => {
  void ensureFilteredLogsVisible()
}, { flush: "post" })

</script>

<template>
  <section
    ref="panelRef"
    class="rounded-2xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.08)]"
    @contextmenu="openMenu($event)"
  >
    <div class="px-2.5 pt-2.5 pb-2 sm:px-3 sm:pt-3">
      <div class="flex flex-wrap items-center gap-2 sm:gap-3">
        <h2 class="min-w-0 flex-1 text-sm sm:text-base">{{ logFocusJobId ? $t("main.logs.job_title") : $t("main.logs.all_title") }}</h2>
        <div class="ml-auto flex flex-wrap justify-end gap-1.5">
          <button
            v-for="filter in LOG_FILTER_KEYS"
            :key="filter"
            type="button"
            class="rounded-full border px-2.5 py-1 text-xs transition sm:px-3"
            :class="activeFilter === filter ? 'border-blue-600/25 bg-blue-50 text-blue-800' : 'border-slate-200 bg-slate-50 text-slate-500 hover:bg-slate-100'"
            @click="activeFilter = filter"
          >
            {{ $t(`main.logs.filters.${filter}`) }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="showInitialLoading" class="p-2.5 text-xs text-slate-500 sm:p-3 sm:text-sm">{{ $t("main.logs.loading") }}</div>
    <div v-else-if="!sortedLogs.length" class="p-2.5 text-xs text-slate-500 sm:p-3 sm:text-sm">{{ $t("main.logs.empty") }}</div>
    <div v-else-if="showFilteredEmpty" class="p-2.5 text-xs text-slate-500 sm:p-3 sm:text-sm">{{ $t("main.logs.filtered_empty") }}</div>
    <template v-else>
      <div
        v-for="l in filteredLogs"
        :key="l.id"
        class="mx-2.5 mb-2.5 rounded-xl border border-slate-200 bg-white p-2.5 data-[flash=true]:border-amber-500/40 data-[flash=true]:ring-4 data-[flash=true]:ring-amber-500/20 sm:mx-3 sm:mb-3 sm:p-3"
        :data-log-id="l.id"
        :data-flash="flashLogId === String(l.id)"
        @contextmenu="openMenu($event, l)"
      >
        <div class="flex items-center justify-between gap-2">
          <div class="flex flex-wrap items-baseline gap-1.5 sm:gap-2.5">
            <button
              type="button"
              class="rounded-md text-left text-sm font-semibold text-slate-900 transition focus:outline-none focus:ring-4 focus:ring-blue-600/20 disabled:cursor-default disabled:text-slate-900 enabled:cursor-pointer enabled:hover:text-blue-700 sm:text-base"
              :disabled="!canEditLogJob(l)"
              :title="canEditLogJob(l) ? $t('main.context.edit_job') : undefined"
              @click.stop="editLogJob(l)"
            >
              {{ l.jobName }}
            </button>
            <span v-if="!isRunning(l)" class="mt-0.5 text-xs text-slate-500">exit={{ l.exitCode }}</span>
          </div>
          <div class="flex flex-wrap items-center justify-end gap-1.5">
            <span
              v-if="getTriggerSourceLabel(l)"
              class="h-fit rounded-full border border-slate-200 bg-slate-50 px-2 py-0.5 text-[10px] text-slate-500 sm:px-2.5 sm:py-1 sm:text-[11px]"
            >
              {{ getTriggerSourceLabel(l) }}
            </span>
            <span
              class="h-fit rounded-full border border-slate-200 bg-slate-50 px-2 py-0.5 text-[10px] text-slate-500 data-[kind=info]:border-blue-600/25 data-[kind=info]:bg-blue-50 data-[kind=info]:text-blue-800 data-[kind=success]:border-green-600/25 data-[kind=success]:bg-green-50 data-[kind=success]:text-green-800 data-[kind=danger]:border-red-600/25 data-[kind=danger]:bg-red-50 data-[kind=danger]:text-red-800 sm:px-2.5 sm:py-1 sm:text-[11px]"
              :data-kind="isRunning(l) ? 'info' : Number(l.exitCode) === 0 ? 'success' : 'danger'"
            >
              {{ isRunning(l) ? $t("common.running") : Number(l.exitCode) === 0 ? $t("common.ok") : $t("common.fail") }}
            </span>
          </div>
        </div>
        <div v-if="l.commandLine" class="mt-1 text-xs text-slate-500">{{ l.commandLine }}</div>
        <div class="mt-1 text-xs text-slate-500">
          {{ formatDateTime(l.startedAt) }} -> {{ isRunning(l) ? $t("common.running") : formatDateTime(l.finishedAt) }}
          <span v-if="formatDuration(l)">({{ formatDuration(l) }})</span>
        </div>
        <div v-if="l.error" class="mt-2 rounded-xl border border-red-600/25 bg-red-50 px-2.5 py-2 text-xs text-red-800 sm:mt-2.5 sm:px-3 sm:py-2.5 sm:text-sm">{{ l.error }}</div>
        <CollapsibleLog
          v-if="l.stdout"
          :text="l.stdout"
          :max-lines="30"
          pre-class="mt-2 whitespace-pre-wrap rounded-xl border border-slate-200 bg-slate-950 px-2 py-2 text-xs text-slate-200 sm:mt-2.5 sm:px-2.5 sm:py-2.5"
          overlay-class="bg-gradient-to-b from-transparent to-slate-950"
          button-class="text-slate-500 hover:text-slate-300"
        />
        <CollapsibleLog
          v-if="l.stderr"
          :text="l.stderr"
          :max-lines="30"
          pre-class="mt-2 whitespace-pre-wrap rounded-xl border border-red-600/25 bg-red-50 px-2 py-2 text-xs text-red-800 sm:mt-2.5 sm:px-2.5 sm:py-2.5"
          overlay-class="bg-gradient-to-b from-transparent to-red-50"
          button-class="text-red-700 hover:text-red-800"
        />
      </div>

      <div class="px-2.5 pb-3 text-center text-xs text-slate-400 sm:px-3">
        <div v-if="!logsLoadingMore && !needsMoreLogs">{{ $t("main.logs.no_more") }}</div>
        <div v-else class="h-4"></div>
      </div>
    </template>

    <teleport to="body">
      <div v-if="contextVisible" class="fixed inset-0 z-40" @click="closeContextMenu" @contextmenu.prevent="closeContextMenu" />
      <div
        v-if="contextVisible"
        class="fixed z-50 w-[220px] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.18)]"
        :style="{ left: contextX + 'px', top: contextY + 'px' }"
      >
        <template v-if="contextLog">
          <button v-if="!editorVisible" class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextEditJob">
            <span>{{ $t("main.context.edit_job") }}</span>
          </button>
          <button v-if="!editorVisible" class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextShowJobLogs">
            <span>{{ $t("main.context.show_job_logs") }}</span>
          </button>

          <div v-if="!editorVisible" class="h-px bg-slate-200/70" />

          <button
            v-if="isRunning(contextLog)"
            class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-rose-700 hover:bg-rose-50"
            @click="onContextTerminateTask"
          >
            <span>{{ $t("common.terminate_now") }}</span>
          </button>
          <template v-else>
            <button
              class="flex w-full items-center justify-between px-3 py-2 text-left text-xs transition disabled:cursor-not-allowed disabled:text-slate-400 disabled:hover:bg-white enabled:hover:bg-slate-50"
              :disabled="!canCopyLogOutput(contextLog)"
              @click="onContextCopyOutput"
            >
              <span>{{ $t("main.context.copy_output") }}</span>
            </button>
            <div class="h-px bg-slate-200/70" />
            <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-rose-700 hover:bg-rose-50" @click="onContextDeleteRecord">
              <span>{{ $t("main.context.delete_records") }}</span>
            </button>
          </template>
        </template>
        <template v-else>
          <button
            class="flex w-full items-center justify-between px-3 py-2 text-left text-xs transition disabled:cursor-not-allowed disabled:opacity-50"
            :class="panelClearConfirm ? 'bg-rose-600 text-white hover:bg-rose-700' : 'text-rose-700 hover:bg-rose-50'"
            :disabled="!logs.length"
            @click="onContextClearLogs"
          >
            <span>{{ panelClearConfirm ? $t("main.logs.clear_confirm_title") : $t("main.logs.clear_title") }}</span>
          </button>
        </template>
      </div>
    </teleport>
  </section>
</template>
