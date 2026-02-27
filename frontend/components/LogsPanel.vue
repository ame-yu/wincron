<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from "vue"
import { storeToRefs } from "pinia"
import { useCronStore } from "../stores/cron.js"
import { getMenuPosition } from "../ui/menuPosition.js"
import { formatDateTime } from "../ui/datetime.js"
import CollapsibleLog from "./CollapsibleLog.vue"

defineProps({
  btnDanger: { type: String, required: true },
})

const cron = useCronStore()
const { logs, editorVisible, jobs } = storeToRefs(cron)

const contextVisible = ref(false)
const contextLog = ref(null)
const contextX = ref(0)
const contextY = ref(0)

function closeContextMenu() {
  contextVisible.value = false
  contextLog.value = null
}

function openMenuAt(e, menuHeight) {
  const pos = getMenuPosition(e, { menuWidth: 220, menuHeight, padding: 8 })
  contextX.value = pos.x
  contextY.value = pos.y
  contextVisible.value = true
}

function openContextMenu(e, log) {
  if (!log) return
  e?.preventDefault?.()
  e?.stopPropagation?.()
  contextLog.value = log
  openMenuAt(e, editorVisible.value ? 80 : 160)
}

function findJobById(jobId) {
  const id = String(jobId || "")
  if (!id) return null
  const list = Array.isArray(jobs.value) ? jobs.value : []
  return list.find((j) => String(j?.id || "") === id) || null
}

function onContextEditJob() {
  const entry = contextLog.value
  if (!entry) return
  const job = findJobById(entry.jobId)
  if (!job) return
  cron.editJob(job)
  closeContextMenu()
}

function onContextShowJobLogs() {
  const entry = contextLog.value
  if (!entry) return
  cron.focusLogs(String(entry.jobId || ""))
  closeContextMenu()
}

function onContextCopyOutput() {
  const entry = contextLog.value
  if (!entry) return
  cron.copyLogOutput(entry)
  closeContextMenu()
}

function onContextDeleteRecord() {
  const entry = contextLog.value
  if (!entry) return
  cron.deleteLogEntry(String(entry.id || ""))
  closeContextMenu()
}

onMounted(() => {
  window.addEventListener("blur", closeContextMenu)
})

onBeforeUnmount(() => {
  window.removeEventListener("blur", closeContextMenu)
})

const getLogMs = (l) => {
  const raw = l?.finishedAt || l?.startedAt || ""
  const ms = Date.parse(raw)
  return Number.isFinite(ms) ? ms : 0
}

const formatLocalTime = (raw) => formatDateTime(raw)

const formatDuration = (l) => {
  const startMs = Date.parse(l?.startedAt || "")
  if (!Number.isFinite(startMs)) return ""
  const endMs = l?.finishedAt ? Date.parse(l.finishedAt) : Date.now()
  if (!Number.isFinite(endMs)) return ""
  return `${Math.max(0, (endMs - startMs) / 1000).toFixed(1)}s`
}

const sortedLogs = computed(() =>
  [...(Array.isArray(logs.value) ? logs.value : [])].sort((a, b) => getLogMs(b) - getLogMs(a)),
)
</script>

<template>
  <section data-wincron-keep-selection="1" class="rounded-2xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.08)]">
    <div class="flex items-start justify-between gap-3 px-3 pt-3 pb-2">
      <div>
        <h2>{{ $t("main.logs.title") }}</h2>
        <div class="mt-0.5 text-xs text-slate-500">{{ $t("main.logs.subtitle") }}</div>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button
          :class="btnDanger + ' disabled:opacity-60 disabled:cursor-not-allowed'"
          type="button"
          :title="$t('main.logs.clear_title')"
          :disabled="!logs.length"
          @click="cron.clearLogs"
        >
          🗑️ {{ $t("main.logs.clear_title") }}
        </button>
      </div>
    </div>

    <div v-if="!sortedLogs.length" class="p-2.5 text-sm text-slate-500">{{ $t("main.logs.empty") }}</div>
    <div v-for="l in sortedLogs" :key="l.id" class="mx-3 mb-3 rounded-xl border border-slate-200 bg-white p-3" @contextmenu="openContextMenu($event, l)">
      <div class="flex items-center justify-between gap-2.5">
        <div class="flex flex-wrap items-baseline gap-2.5">
          <strong>{{ l.jobName }}</strong>
          <span class="mt-0.5 text-xs text-slate-500">exit={{ l.exitCode }}</span>
        </div>
        <span
          class="h-fit rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 text-[11px] text-slate-500 data-[kind=success]:border-green-600/25 data-[kind=success]:bg-green-50 data-[kind=success]:text-green-800 data-[kind=danger]:border-red-600/25 data-[kind=danger]:bg-red-50 data-[kind=danger]:text-red-800"
          :data-kind="l.exitCode === 0 ? 'success' : 'danger'"
        >
          {{ l.exitCode === 0 ? $t("common.ok") : $t("common.fail") }}
        </span>
      </div>
      <div v-if="l.commandLine" class="mt-1.5 text-xs text-slate-500">{{ l.commandLine }}</div>
      <div class="mt-1.5 text-xs text-slate-500">
        {{ formatLocalTime(l.startedAt) }} -> {{ formatLocalTime(l.finishedAt) }}
        <span v-if="formatDuration(l)">({{ formatDuration(l) }})</span>
      </div>
      <div v-if="l.error" class="mt-2.5 rounded-xl border border-red-600/25 bg-red-50 px-3 py-2.5 text-sm text-red-800">{{ l.error }}</div>
      <CollapsibleLog
        v-if="l.stdout"
        :text="l.stdout"
        :max-lines="30"
        pre-class="mt-2.5 whitespace-pre-wrap rounded-xl border border-slate-200 bg-slate-950 px-2.5 py-2.5 text-slate-200"
        overlay-class="bg-gradient-to-b from-transparent to-slate-950"
        button-class="text-slate-500 hover:text-slate-300"
      />
      <CollapsibleLog
        v-if="l.stderr"
        :text="l.stderr"
        :max-lines="30"
        pre-class="mt-2.5 whitespace-pre-wrap rounded-xl border border-red-600/25 bg-red-50 px-2.5 py-2.5 text-red-800"
        overlay-class="bg-gradient-to-b from-transparent to-red-50"
        button-class="text-red-700 hover:text-red-800"
      />
    </div>

    <teleport to="body">
      <div v-if="contextVisible" data-wincron-keep-selection="1" class="fixed inset-0 z-40" @click="closeContextMenu" @contextmenu.prevent="closeContextMenu" />
      <div
        v-if="contextVisible"
        data-wincron-keep-selection="1"
        class="fixed z-50 w-[220px] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.18)]"
        :style="{ left: contextX + 'px', top: contextY + 'px' }"
      >
        <button v-if="!editorVisible" class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextEditJob">
          <span>{{ $t("main.context.edit_job") }}</span>
        </button>
        <button v-if="!editorVisible" class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextShowJobLogs">
          <span>{{ $t("main.context.show_job_logs") }}</span>
        </button>

        <div v-if="!editorVisible" class="h-px bg-slate-200/70" />

        <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextCopyOutput">
          <span>{{ $t("main.context.copy_output") }}</span>
        </button>
        <div class="h-px bg-slate-200/70" />
        <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-rose-700 hover:bg-rose-50" @click="onContextDeleteRecord">
          <span>{{ $t("main.context.delete_records") }}</span>
        </button>
      </div>
    </teleport>
  </section>
</template>
