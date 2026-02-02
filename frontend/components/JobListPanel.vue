<script setup>
import { computed, ref } from "vue"
import { storeToRefs } from "pinia"
import { useCronStore } from "../stores/cron.js"
import JobSortControls from "./JobSortControls.vue"

defineProps({
  btn: { type: String, required: true },
  btnPrimary: { type: String, required: true },
  btnDanger: { type: String, required: true },
})

const cron = useCronStore()
const { jobs, selectedJobId } = storeToRefs(cron)

const contextVisible = ref(false)
const contextJob = ref(null)
const contextX = ref(0)
const contextY = ref(0)

const jobSortKey = ref("name")
const jobSortAsc = ref(true)

const sortedJobs = computed(() => {
  const list = Array.isArray(jobs.value) ? [...jobs.value] : []
  const dir = jobSortAsc.value ? 1 : -1

  const getName = (j) => String(j?.name || j?.command || "").trim().toLowerCase()
  const getExecutedCount = (j) => {
    const n = Number(j?.executedCount)
    return Number.isFinite(n) ? n : 0
  }
  const getLastExecutedMs = (j) => {
    const raw = String(j?.lastExecutedAt || "")
    const ms = Date.parse(raw)
    return Number.isFinite(ms) ? ms : 0
  }
  const getNextRunMs = (j) => {
    const raw = String(j?.nextRunAt || "")
    const ms = Date.parse(raw)
    return Number.isFinite(ms) ? ms : NaN
  }

  list.sort((a, b) => {
    const key = jobSortKey.value
    if (key === "executedCount") {
      const av = getExecutedCount(a)
      const bv = getExecutedCount(b)
      if (av !== bv) return (av - bv) * dir
    } else if (key === "lastExecutedAt") {
      const av = getLastExecutedMs(a)
      const bv = getLastExecutedMs(b)
      if (av !== bv) return (av - bv) * dir
    } else if (key === "nextRunAt") {
      const av = getNextRunMs(a)
      const bv = getNextRunMs(b)
      const aHas = Number.isFinite(av)
      const bHas = Number.isFinite(bv)
      if (aHas && bHas && av !== bv) return (av - bv) * dir
      if (aHas !== bHas) return aHas ? -1 : 1
    } else {
      const av = getName(a)
      const bv = getName(b)
      if (av !== bv) return (av < bv ? -1 : 1) * dir
    }

    const ai = String(a?.id || "")
    const bi = String(b?.id || "")
    return ai.localeCompare(bi) * dir
  })
  return list
})

const formatJobNextRun = (job) => {
  const raw = String(job?.nextRunAt || "")
  if (!raw) {
    return ""
  }
  const ms = Date.parse(raw)
  if (!Number.isFinite(ms)) {
    return raw
  }
  return new Date(ms).toLocaleString()
}

function editJob(job) {
  cron.loadJobToForm(job)
  cron.loadLogs(job.id)
}

function closeContextMenu() {
  contextVisible.value = false
  contextJob.value = null
}

function openContextMenu(e, job) {
  selectedJobId.value = job.id
  contextJob.value = job

  const menuWidth = 220
  const menuHeight = 200
  const padding = 8

  const maxX = Math.max(padding, window.innerWidth - menuWidth - padding)
  const maxY = Math.max(padding, window.innerHeight - menuHeight - padding)

  contextX.value = Math.min(Math.max(padding, e.clientX), maxX)
  contextY.value = Math.min(Math.max(padding, e.clientY), maxY)
  contextVisible.value = true
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

function onContextDelete() {
  if (!contextJob.value) return
  cron.deleteJob(contextJob.value.id)
  closeContextMenu()
}
</script>

<template>
  <aside class="w-full rounded-2xl border border-slate-200 bg-white p-3 shadow-[0_10px_30px_rgba(2,6,23,0.08)] sm:p-3.5 lg:w-[380px]">
    <div class="flex items-start justify-between gap-3 px-3 pt-3 pb-2">
      <div>
        <h2>{{ $t("main.jobs.title") }}</h2>
        <div class="mt-0.5 text-xs text-slate-500">{{ $t("main.jobs.subtitle") }}</div>
      </div>
      <div class="flex shrink-0 items-center gap-2 whitespace-nowrap">
        <JobSortControls v-model:sort-key="jobSortKey" v-model:sort-asc="jobSortAsc" :btn-class="btn" />
        <button :class="btnPrimary" @click="cron.resetForm">{{ $t("common.new") }}</button>
      </div>
    </div>

    <div class="flex flex-col gap-2.5 px-2.5 pb-2.5">
      <div
        v-for="job in sortedJobs"
        :key="job.id"
        class="rounded-xl border border-slate-200 bg-white p-3 data-[selected=true]:border-blue-600/45 data-[selected=true]:ring-4 data-[selected=true]:ring-blue-600/10"
        :data-selected="selectedJobId === job.id"
        @click="cron.selectJob(job.id)"
        @dblclick="editJob(job)"
        @contextmenu.prevent.stop="openContextMenu($event, job)"
      >
        <div class="flex justify-between gap-2.5">
          <div class="min-w-0">
            <div class="overflow-hidden text-ellipsis whitespace-nowrap text-xs font-semibold">{{ job.name || job.command }}</div>
            <div class="mt-0.5 overflow-hidden text-ellipsis whitespace-nowrap text-xs text-slate-500">{{ job.command }}</div>
          </div>
          <span
            class="h-fit rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 text-[11px] text-slate-500 data-[kind=success]:border-green-600/25 data-[kind=success]:bg-green-50 data-[kind=success]:text-green-800 data-[kind=muted]:border-slate-200 data-[kind=muted]:bg-slate-100 data-[kind=muted]:text-slate-600"
            :data-kind="job.enabled ? 'success' : 'muted'"
          >
            {{ job.enabled ? $t("common.enabled") : $t("common.disabled") }}
          </span>
        </div>

        <div
          class="mt-2.5 rounded-xl border border-slate-200 bg-slate-50 px-2.5 py-2 font-mono text-xs text-slate-700"
          :title="formatJobNextRun(job)"
        >
          {{ job.cron }}
        </div>

        <div class="mt-2.5 flex flex-wrap gap-2">
          <button :class="btn" @click.stop="editJob(job)">{{ $t("common.edit") }}</button>
          <button
            class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50 data-[kind=success]:border-green-600/25 data-[kind=success]:bg-green-50 data-[kind=success]:text-green-800 data-[kind=success]:hover:bg-green-100 data-[kind=muted]:border-slate-200 data-[kind=muted]:bg-slate-100 data-[kind=muted]:text-slate-600 data-[kind=muted]:hover:bg-slate-200"
            :data-kind="job.enabled ? 'muted' : 'success'"
            @click.stop="cron.toggleJob(job)"
          >
            {{ job.enabled ? $t("common.disable") : $t("common.enable") }}
          </button>
          <button :class="btnPrimary" @click.stop="cron.runNow(job.id)">{{ $t("common.run_now") }}</button>
          <button :class="btnDanger" @click.stop="cron.deleteJob(job.id)">{{ $t("common.delete") }}</button>
        </div>
      </div>

      <div v-if="!jobs.length" class="p-2.5 text-sm text-slate-500">{{ $t("main.jobs.empty") }}</div>
    </div>

    <teleport to="body">
      <div v-if="contextVisible" class="fixed inset-0 z-40" @click="closeContextMenu" @contextmenu.prevent="closeContextMenu" />
      <div
        v-if="contextVisible"
        class="fixed z-50 w-[220px] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.18)]"
        :style="{ left: contextX + 'px', top: contextY + 'px' }"
      >
        <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextEdit">
          <span>{{ $t("common.edit") }}</span>
        </button>
        <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextToggle">
          <span>{{ contextJob?.enabled ? $t("common.disable") : $t("common.enable") }}</span>
        </button>
        <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onContextCopy">
          <span>{{ $t("common.copy") }}</span>
        </button>
        <div class="h-px bg-slate-200/70" />
        <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-rose-700 hover:bg-rose-50" @click="onContextDelete">
          <span>{{ $t("common.delete") }}</span>
        </button>
      </div>
    </teleport>
  </aside>
</template>
