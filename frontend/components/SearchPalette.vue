<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue"
import { storeToRefs } from "pinia"
import { useI18n } from "vue-i18n"
import { ListLogs } from "../bindings/wincron/cronservice.js"
import { useCronStore } from "../stores/cron.js"
import { formatDateTime } from "../ui/datetime.js"

const MODE_STORAGE_KEY = "wincron.searchPalette.mode"
const MAX_RESULTS = 8
const SEARCH_DEBOUNCE_MS = 180

const props = defineProps({ visible: { type: Boolean, default: false } })
const emit = defineEmits(["update:visible"])

const cron = useCronStore()
const { jobs } = storeToRefs(cron)
const { t } = useI18n()

const inputRef = ref(null)
const query = ref("")
const debouncedQuery = ref("")
const selectedIndex = ref(0)
const logsLoading = ref(false)
const logsError = ref("")
const allLogs = ref([])
const mode = ref(loadMode())
let queryTimer = null

const text = (value) => String(value || "").trim()
const lower = (value) => text(value).toLowerCase()
const joinArgs = (args) => (Array.isArray(args) ? args.map(text).filter(Boolean).join(" ") : "")
const event = (name, detail) => window.dispatchEvent(new CustomEvent(name, { detail }))
const queryText = computed(() => lower(debouncedQuery.value))
const placeholder = computed(() => t(mode.value === "logs" ? "main.search.placeholder_logs" : "main.search.placeholder_jobs"))
const modeThumbStyle = computed(() => ({ width: "50%", transform: `translateX(${mode.value === "logs" ? "100%" : "0%"})` }))
const showLogLoadingState = computed(() => mode.value === "logs" && logsLoading.value && !allLogs.value.length)

function loadMode() {
  try {
    return localStorage.getItem(MODE_STORAGE_KEY) === "logs" ? "logs" : "jobs"
  } catch {
    return "jobs"
  }
}

function persistMode() {
  try {
    localStorage.setItem(MODE_STORAGE_KEY, mode.value)
  } catch {
  }
}

function stamp(value) {
  const ms = Date.parse(value || "")
  return Number.isFinite(ms) ? ms : 0
}

function shorten(value, max = 140) {
  const current = text(value)
  return current.length > max ? `${current.slice(0, max - 1)}...` : current
}

function scoreField(queryValue, value, weight) {
  const source = lower(value)
  if (!queryValue || !source) return 0
  if (source === queryValue) return weight + 60
  if (source.startsWith(queryValue)) return weight + 40
  if ([" ", "/", "\\"].some((prefix) => source.includes(`${prefix}${queryValue}`))) return weight + 20
  return source.includes(queryValue) ? weight : 0
}

function scoreFields(fields) {
  return fields.reduce((sum, [value, weight]) => sum + scoreField(queryText.value, value, weight), 0)
}

function buildSnippet(fields) {
  if (!queryText.value) {
    return shorten(fields.find((value) => text(value)) || "")
  }
  for (const value of fields.map(text)) {
    if (!value) continue
    const index = value.toLowerCase().indexOf(queryText.value)
    if (index < 0) continue
    const start = Math.max(0, index - 36)
    const end = Math.min(value.length, index + queryText.value.length + 72)
    return `${start > 0 ? "..." : ""}${value.slice(start, end)}${end < value.length ? "..." : ""}`
  }
  return ""
}

async function focusInput() {
  await nextTick()
  inputRef.value?.focus?.()
  inputRef.value?.select?.()
}

function clearQueryTimer() {
  if (!queryTimer) return
  clearTimeout(queryTimer)
  queryTimer = null
}

function syncQuery(immediate = false) {
  clearQueryTimer()
  if (immediate) {
    debouncedQuery.value = query.value
    return
  }
  queryTimer = setTimeout(() => {
    debouncedQuery.value = query.value
    queryTimer = null
  }, SEARCH_DEBOUNCE_MS)
}

function resetSearch() {
  clearQueryTimer()
  query.value = ""
  debouncedQuery.value = ""
  selectedIndex.value = 0
}

async function ensureLogsLoaded(force = false) {
  if (mode.value !== "logs" || logsLoading.value || (!force && allLogs.value.length)) return
  logsLoading.value = true
  logsError.value = ""
  try {
    const list = await ListLogs("", 100)
    allLogs.value = Array.isArray(list) ? list : []
  } catch (error) {
    allLogs.value = []
    logsError.value = String(error)
  } finally {
    logsLoading.value = false
  }
}

const closePalette = () => emit("update:visible", false)
const setMode = (nextMode) => { mode.value = nextMode === "logs" ? "logs" : "jobs" }
const moveSelection = (offset) => { selectedIndex.value = results.value.length ? (selectedIndex.value + offset + results.value.length) % results.value.length : 0 }

function buildJobResult(job) {
  const id = text(job?.id)
  if (!id) return null
  const command = text(job?.command)
  const folder = text(job?.folder)
  const args = joinArgs(job?.args)
  const title = text(job?.name) || command || id
  const score = queryText.value ? scoreFields([[title, 340], [command, 220], [folder, 120], [args, 90]]) : 1
  if (!score) return null
  return {
    key: `job:${id}`,
    kind: "job",
    title,
    subtitle: shorten(command),
    meta: folder,
    snippet: queryText.value ? buildSnippet([command, args]) : "",
    statusLabel: job?.enabled ? t("common.enabled") : t("common.disabled"),
    statusKind: job?.enabled ? "success" : "muted",
    score,
    time: stamp(job?.lastExecutedAt),
    job,
  }
}

function buildLogResult(entry) {
  const id = text(entry?.id)
  const jobId = text(entry?.jobId)
  if (!id || !jobId) return null
  const commandLine = text(entry?.commandLine)
  const errorText = text(entry?.error)
  const stdout = text(entry?.stdout)
  const stderr = text(entry?.stderr)
  const timeValue = text(entry?.finishedAt) || text(entry?.startedAt)
  const title = text(entry?.jobName) || commandLine || t("main.search.untitled_log")
  const score = queryText.value ? scoreFields([[title, 300], [commandLine, 220], [errorText, 170], [stdout, 140], [stderr, 140]]) : 1
  if (!score) return null
  return {
    key: `log:${id}`,
    kind: "log",
    title,
    subtitle: shorten(commandLine),
    meta: formatDateTime(timeValue),
    snippet: buildSnippet([errorText, stderr, stdout]),
    statusLabel: Number(entry?.exitCode) === 0 ? t("common.ok") : t("common.fail"),
    statusKind: Number(entry?.exitCode) === 0 ? "success" : "danger",
    score,
    time: stamp(timeValue),
    entry,
  }
}

const taskResults = computed(() =>
  (Array.isArray(jobs.value) ? jobs.value : [])
    .map(buildJobResult)
    .filter(Boolean)
    .sort((a, b) => (queryText.value ? b.score - a.score : 0) || b.time - a.time || a.title.localeCompare(b.title))
    .slice(0, MAX_RESULTS),
)

const logResults = computed(() =>
  (Array.isArray(allLogs.value) ? allLogs.value : [])
    .map(buildLogResult)
    .filter(Boolean)
    .sort((a, b) => (queryText.value ? b.score - a.score : 0) || b.time - a.time)
    .slice(0, MAX_RESULTS),
)

const results = computed(() => (mode.value === "logs" ? logResults.value : taskResults.value))
const canJump = computed(() => !!results.value.length && !showLogLoadingState.value)

watch(mode, async (value) => {
  persistMode()
  selectedIndex.value = 0
  if (props.visible && value === "logs") await ensureLogsLoaded(true)
  if (props.visible) await focusInput()
})

watch(query, () => {
  selectedIndex.value = 0
  syncQuery()
})

watch(results, (list) => {
  selectedIndex.value = list.length ? Math.min(selectedIndex.value, list.length - 1) : 0
})

watch(
  () => props.visible,
  async (visible) => {
    resetSearch()
    if (!visible) return
    if (mode.value === "logs") await ensureLogsLoaded(true)
    await focusInput()
  },
)

onBeforeUnmount(clearQueryTimer)

async function jumpTo(result = null) {
  syncQuery(true)
  const target = result || results.value[selectedIndex.value] || results.value[0]
  if (!target || !canJump.value) return
  if (target.kind === "job") {
    await cron.editJob(target.job)
    event("wincron:reveal-job", { jobId: text(target.job?.id) })
    closePalette()
    return
  }
  const jobId = text(target.entry?.jobId)
  const logId = text(target.entry?.id)
  if (!jobId || !logId) return
  await cron.selectJob(jobId)
  event("wincron:reveal-log", { jobId, logId })
  closePalette()
}

async function runHighlightedTask() {
  syncQuery(true)
  const target = results.value[selectedIndex.value] || results.value[0]
  if (mode.value !== "jobs" || target?.kind !== "job") return
  const jobId = text(target.job?.id)
  if (!jobId) return
  await cron.selectJob(jobId)
  event("wincron:reveal-job", { jobId })
  closePalette()
  void cron.runNow(jobId)
}
</script>

<template>
  <teleport to="body">
    <Transition name="search-palette">
      <div
        v-if="visible"
        class="fixed inset-0 z-[10000] bg-slate-950/30 px-4 pt-[11vh] backdrop-blur-[3px]"
        @click.self="closePalette"
      >
        <div class="search-palette__panel mx-auto w-full max-w-[760px] overflow-hidden rounded-[28px] border border-slate-200/80 bg-white shadow-[0_32px_90px_rgba(15,23,42,0.28)]" @click.stop>
          <div class="border-b border-slate-200/80 bg-[linear-gradient(135deg,rgba(248,250,252,1),rgba(255,255,255,0.98))] px-4 pt-4 pb-3 sm:px-5 sm:pt-5">
            <div class="text-[11px] font-semibold uppercase tracking-[0.26em] text-slate-400">{{ $t("main.search.title") }}</div>
            <div class="mt-3 flex items-center gap-3">
              <input
                ref="inputRef"
                v-model="query"
                class="h-12 min-w-0 flex-1 rounded-2xl border border-slate-200 bg-slate-50 px-4 text-sm text-slate-900 outline-none transition placeholder:text-slate-400 focus:border-blue-600/40 focus:bg-white focus:ring-4 focus:ring-blue-600/15"
                :placeholder="placeholder"
                @keydown.down.prevent="moveSelection(1)"
                @keydown.up.prevent="moveSelection(-1)"
                @keydown.ctrl.enter.prevent="runHighlightedTask()"
                @keydown.enter.exact.prevent="jumpTo()"
                @keydown.esc.prevent="closePalette"
              />

              <div class="relative grid h-12 shrink-0 grid-cols-2 rounded-full border border-slate-200 bg-slate-50 p-0.5 text-sm">
                <div class="pointer-events-none absolute left-0 top-0 bottom-0 m-0.5 rounded-full bg-white shadow transition-transform duration-200" :style="modeThumbStyle" />
                <button
                  type="button"
                  class="relative z-10 min-w-[88px] rounded-full px-3 py-2 text-xs transition focus:outline-none"
                  :class="mode === 'jobs' ? 'text-slate-900' : 'text-slate-500 hover:text-slate-700'"
                  @click="setMode('jobs')"
                >
                  {{ $t("main.search.scope_jobs") }}
                </button>
                <button
                  type="button"
                  class="relative z-10 min-w-[88px] rounded-full px-3 py-2 text-xs transition focus:outline-none"
                  :class="mode === 'logs' ? 'text-slate-900' : 'text-slate-500 hover:text-slate-700'"
                  @click="setMode('logs')"
                >
                  {{ $t("main.search.scope_logs") }}
                </button>
              </div>
            </div>
          </div>

          <div class="max-h-[420px] overflow-auto p-2.5 sm:p-3">
            <div v-if="showLogLoadingState" class="px-3 py-10 text-center text-sm text-slate-500">{{ $t("main.search.loading_logs") }}</div>
            <div v-else-if="mode === 'logs' && logsError" class="rounded-2xl border border-red-600/20 bg-red-50 px-3 py-4 text-sm text-red-700">{{ logsError }}</div>
            <div v-else-if="!results.length" class="px-3 py-10 text-center text-sm text-slate-500">{{ $t("main.search.empty") }}</div>

            <button
              v-for="(result, index) in results"
              :key="result.key"
              type="button"
              class="mb-2 flex w-full items-start justify-between gap-3 rounded-2xl border border-transparent px-3 py-3 text-left transition last:mb-0 data-[active=true]:border-blue-600/30 data-[active=true]:bg-blue-50/70 hover:border-slate-200 hover:bg-slate-50"
              :data-active="selectedIndex === index"
              @mouseenter="selectedIndex = index"
              @click="jumpTo(result)"
            >
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm font-semibold text-slate-900">{{ result.title }}</div>
                <div v-if="result.subtitle" class="mt-1 truncate text-xs text-slate-500">{{ result.subtitle }}</div>
                <div v-if="result.snippet" class="search-palette__snippet mt-2 text-xs text-slate-600">{{ result.snippet }}</div>
              </div>

              <div class="flex shrink-0 flex-col items-end gap-2">
                <div
                  class="rounded-full px-2.5 py-1 text-[11px] font-medium"
                  :class="{
                    'bg-green-50 text-green-800': result.statusKind === 'success',
                    'bg-rose-50 text-rose-700': result.statusKind === 'danger',
                    'bg-slate-100 text-slate-600': result.statusKind === 'muted',
                  }"
                >
                  {{ result.statusLabel }}
                </div>
                <div v-if="result.meta" class="max-w-[180px] truncate text-[11px] text-slate-400">{{ result.meta }}</div>
              </div>
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </teleport>
</template>

<style scoped>
.search-palette__snippet { display: -webkit-box; overflow: hidden; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.search-palette-enter-active, .search-palette-leave-active { transition: opacity 180ms ease; }
.search-palette-enter-active .search-palette__panel, .search-palette-leave-active .search-palette__panel { transition: transform 180ms ease, opacity 180ms ease; }
.search-palette-enter-from, .search-palette-leave-to { opacity: 0; }
.search-palette-enter-from .search-palette__panel, .search-palette-leave-to .search-palette__panel { opacity: 0; transform: translateY(-10px) scale(0.985); }
</style>
