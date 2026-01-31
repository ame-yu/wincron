<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { storeToRefs } from "pinia"
import { useCronStore } from "../stores/cron.js"
import JobSortControls from "../components/JobSortControls.vue"

const cron = useCronStore()
const { error, jobs, selectedJobId, logs, editorVisible } = storeToRefs(cron)
const form = cron.form

const showAdvanced = ref(false)

const jobSortKey = ref("name")
const jobSortAsc = ref(true)

const cronNextRun = ref("")
const cronNextRunError = ref("")
const cronNextRunPending = ref(false)

let cronPreviewTimer = null
let cronPreviewSeq = 0

const btn =
  "appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
const btnPrimary =
  "appearance-none rounded-xl border border-blue-600/35 bg-blue-600 px-2.5 py-2 text-xs leading-none text-white transition hover:bg-blue-700 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
const btnDanger =
  "appearance-none rounded-xl border border-red-600/25 bg-red-50 px-2.5 py-2 text-xs leading-none text-red-600 transition hover:bg-red-100 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"

const onGlobalKeydown = (e) => {
  if (e?.repeat) {
    return
  }
  const key = typeof e?.key === "string" ? e.key.toLowerCase() : ""
  if ((e.ctrlKey || e.metaKey) && key === "s") {
    e.preventDefault()
    cron.saveJob()
  }
}

onMounted(() => {
  window.addEventListener("keydown", onGlobalKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener("keydown", onGlobalKeydown)
  if (cronPreviewTimer) {
    clearTimeout(cronPreviewTimer)
    cronPreviewTimer = null
  }
})

const getLogMs = (l) => {
  const raw = l?.finishedAt || l?.startedAt || ""
  const ms = Date.parse(raw)
  return Number.isFinite(ms) ? ms : 0
}

const sortedLogs = computed(() =>
  [...(Array.isArray(logs.value) ? logs.value : [])].sort((a, b) => getLogMs(b) - getLogMs(a)),
)

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

const argRefs = []
const setArgRef = (el, index) => (argRefs[index] = el)
const focusArg = (index) => nextTick(() => argRefs[index]?.focus?.())
const ensureArgs = () => (Array.isArray(form.args) ? form.args : (form.args = [""]))

function addArg(afterIndex) {
  const args = ensureArgs()
  const insertAt = Math.min(Math.max(afterIndex + 1, 0), args.length)
  args.splice(insertAt, 0, "")
  focusArg(insertAt)
}

function removeArg(index) {
  const args = ensureArgs()
  if (args.length === 1) {
    args[0] = ""
    focusArg(0)
    return
  }
  args.splice(index, 1)
  focusArg(Math.min(Math.max(index - 1, 0), args.length - 1))
}

function onArgBackspace(e, index) {
  const args = ensureArgs()
  if (args.length <= 1 || args[index] !== "") {
    return
  }
  e.preventDefault()
  removeArg(index)
}

function onArgEnter(e, index) {
  const args = ensureArgs()
  if (args[index] === "" && index === args.length - 1) {
    e.preventDefault()
    return
  }
  e.preventDefault()
  addArg(index)
}

function editJob(job) {
  cron.loadJobToForm(job)
  cron.loadLogs(job.id)
}

const commandPreview = computed(() => {
  const cmd = form.command ?? ""
  if (cmd === "") {
    return ""
  }
  const args = Array.isArray(form.args) ? form.args.filter((s) => s !== "") : []
  return [cmd, ...args].filter((s) => s !== "").join(" ")
})

const cronNextRunDisplay = computed(() => {
  const raw = cronNextRun.value
  if (!raw) {
    return ""
  }
  const ms = Date.parse(raw)
  if (!Number.isFinite(ms)) {
    return raw
  }
  return new Date(ms).toLocaleString()
})

watch(
  () => form.cron,
  (value) => {
    const seq = ++cronPreviewSeq
    cronNextRun.value = ""
    cronNextRunError.value = ""

    if (cronPreviewTimer) {
      clearTimeout(cronPreviewTimer)
      cronPreviewTimer = null
    }

    const expr = typeof value === "string" ? value.trim() : ""
    if (!expr) {
      cronNextRunPending.value = false
      return
    }

    cronNextRunPending.value = true
    cronPreviewTimer = setTimeout(async () => {
      try {
        const result = await cron.previewNextRun(expr)
        if (seq !== cronPreviewSeq) {
          return
        }
        cronNextRun.value = result
        cronNextRunError.value = ""
      } catch (e) {
        if (seq !== cronPreviewSeq) {
          return
        }
        cronNextRun.value = ""
        cronNextRunError.value = String(e)
      } finally {
        if (seq === cronPreviewSeq) {
          cronNextRunPending.value = false
        }
      }
    }, 350)
  },
  { immediate: true },
)
</script>

<template>
  <div class="mx-auto flex max-w-[1240px] flex-col gap-4 p-3 sm:p-5 lg:flex-row">
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
    </aside>

    <main class="min-w-0 flex flex-1 flex-col gap-4">
      <section
        v-if="editorVisible"
        class="rounded-2xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.08)]"
      >
        <div class="flex items-start justify-between gap-3 px-3 pt-3 pb-2">
          <div>
            <h2>{{ $t("main.editor.title") }}</h2>
            <div class="mt-0.5 text-xs text-slate-500">{{ $t("main.editor.subtitle") }}</div>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button :class="btnPrimary" @click="cron.saveJob">{{ $t("common.save") }}</button>
          </div>
        </div>

        <div v-if="error" class="mx-3 mb-3 rounded-xl border border-red-600/25 bg-red-50 px-3 py-2.5 text-sm text-red-800">
          {{ error }}
        </div>

        <div class="grid grid-cols-1 gap-x-3 gap-y-2.5 px-3 pb-2.5 md:grid-cols-[160px_1fr]">
          <label class="text-xs text-slate-500 md:pt-2.5">{{ $t("main.fields.name") }}</label>
          <input
            v-model="form.name"
            class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            :placeholder="$t('main.placeholders.name')"
          />

          <label class="text-xs text-slate-500 md:pt-2.5">{{ $t("main.fields.cron") }}</label>
          <div>
            <input
              v-model="form.cron"
              class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
              :placeholder="$t('main.placeholders.cron')"
            />
            <div v-if="cronNextRunError" class="mt-1 text-xs text-red-700">{{ cronNextRunError }}</div>
            <div v-else-if="cronNextRunPending" class="mt-1 text-xs text-slate-500">{{ $t("main.next_run.calculating") }}</div>
            <div v-else class="mt-1 text-xs text-slate-500">{{ $t("main.next_run.display", { value: cronNextRunDisplay || "-" }) }}</div>
          </div>

          <label class="text-xs text-slate-500 md:pt-2.5">{{ $t("main.fields.command") }}</label>
          <input
            v-model="form.command"
            class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            :placeholder="$t('main.placeholders.command')"
          />

          <label class="text-xs text-slate-500 md:pt-2.5">{{ $t("main.fields.args") }}</label>
          <div>
            <div v-for="(a, i) in form.args" :key="i" class="mb-2 flex flex-wrap items-center gap-2">
              <input
                :ref="(el) => setArgRef(el, i)"
                v-model="form.args[i]"
                class="w-full flex-1 rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
                :placeholder="$t('main.placeholders.arg')"
                @keydown.enter="onArgEnter($event, i)"
                @keydown.backspace="onArgBackspace($event, i)"
              />
              <button :class="btn" type="button" @click="addArg(i)">+</button>
              <button :class="btn" type="button" @click="removeArg(i)">-</button>
            </div>
          </div>

          <label v-if="commandPreview" class="text-xs text-slate-500 md:pt-2.5">{{ $t("main.fields.preview") }}</label>
          <div v-if="commandPreview" class="flex flex-col items-stretch gap-2 sm:flex-row">
            <pre class="m-0 flex-1 whitespace-pre-wrap rounded-xl border border-slate-200 bg-slate-100 px-2.5 py-2.5 font-mono text-xs text-slate-900">{{ commandPreview }}</pre>
            <button
              class="w-full appearance-none rounded-xl border border-blue-600/35 bg-blue-600 px-2.5 py-2 text-xs leading-none text-white transition hover:bg-blue-700 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50 sm:w-auto"
              type="button"
              @click="cron.runPreviewFromForm"
            >
              {{ $t("common.run") }}
            </button>
          </div>

          <template v-if="showAdvanced">
            <label class="text-xs text-slate-500 md:pt-2.5">{{ $t("main.fields.workdir") }}</label>
            <input
              v-model="form.workDir"
              class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
              :placeholder="$t('main.placeholders.workdir')"
            />
          </template>

          <label class="text-xs text-slate-500 md:pt-2.5">{{ $t("main.fields.enabled") }}</label>
          <div class="flex items-center gap-2.5 pt-1.5">
            <input class="h-5 w-5" type="checkbox" v-model="form.enabled" />
            <span class="mt-0.5 text-xs text-slate-500">{{ form.id ? $t("main.enabled_help") : $t("main.enabled_help_create") }}</span>
          </div>

          <template v-if="showAdvanced">
            <label class="text-xs text-slate-500 md:pt-2.5">{{ $t("main.fields.max_failures") }}</label>
            <div class="flex flex-wrap items-center gap-2.5 pt-1.5">
              <input
                v-model.number="form.maxConsecutiveFailures"
                type="number"
                min="1"
                class="w-full max-w-[220px] rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
                :placeholder="$t('main.placeholders.max_failures')"
              />
              <span class="mt-0.5 text-xs text-slate-500">{{ $t("main.max_failures_help") }}</span>
            </div>
          </template>
        </div>

        <div class="px-3 pb-3.5">
          <div class="flex justify-center">
            <button :class="btn" type="button" @click="showAdvanced = !showAdvanced">
              {{ showAdvanced ? $t("main.advanced.hide") : $t("main.advanced.show") }}
            </button>
          </div>
        </div>
      </section>

      <section class="rounded-2xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.08)]">
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
        <div v-for="l in sortedLogs" :key="l.id" class="mx-3 mb-3 rounded-xl border border-slate-200 bg-white p-3">
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
          <div class="mt-1.5 text-xs text-slate-500">{{ l.startedAt }} -> {{ l.finishedAt }}</div>
          <div v-if="l.error" class="mt-2.5 rounded-xl border border-red-600/25 bg-red-50 px-3 py-2.5 text-sm text-red-800">{{ l.error }}</div>
          <pre v-if="l.stdout" class="mt-2.5 whitespace-pre-wrap rounded-xl border border-slate-200 bg-slate-950 px-2.5 py-2.5 text-slate-200">{{ l.stdout }}</pre>
          <pre v-if="l.stderr" class="mt-2.5 whitespace-pre-wrap rounded-xl border border-red-600/25 bg-red-50 px-2.5 py-2.5 text-red-800">{{ l.stderr }}</pre>
        </div>
      </section>
    </main>
  </div>
</template>
