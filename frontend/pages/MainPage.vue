<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue"
import { storeToRefs } from "pinia"
import { useCronStore } from "../stores/cron.js"

const cron = useCronStore()
const { error, jobs, selectedJobId, logs } = storeToRefs(cron)
const form = cron.form

const showAdvanced = ref(false)

function onGlobalKeydown(e) {
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
})

const sortedLogs = computed(() => {
  const arr = Array.isArray(logs.value) ? [...logs.value] : []
  const getMs = (l) => {
    const raw = l?.finishedAt || l?.startedAt || ""
    const ms = Date.parse(raw)
    return Number.isFinite(ms) ? ms : 0
  }
  arr.sort((a, b) => getMs(b) - getMs(a))
  return arr
})

const argRefs = ref([])

function setArgRef(el, index) {
  if (!el) {
    return
  }
  argRefs.value[index] = el
}

function focusArg(index) {
  nextTick(() => {
    const el = argRefs.value[index]
    if (el && typeof el.focus === "function") {
      el.focus()
    }
  })
}

function addArg(afterIndex) {
  if (!Array.isArray(form.args)) {
    form.args = [""]
  }
  const insertAt = Math.min(Math.max(afterIndex + 1, 0), form.args.length)
  form.args.splice(insertAt, 0, "")
  focusArg(insertAt)
}

function removeArg(index) {
  if (!Array.isArray(form.args) || form.args.length === 0) {
    form.args = [""]
    focusArg(0)
    return
  }
  if (form.args.length === 1) {
    form.args[0] = ""
    focusArg(0)
    return
  }
  const nextIndex = Math.max(index - 1, 0)
  form.args.splice(index, 1)
  focusArg(Math.min(nextIndex, form.args.length - 1))
}

function onArgBackspace(e, index) {
  if (!Array.isArray(form.args)) {
    return
  }
  if (form.args[index] !== "") {
    return
  }
  if (form.args.length <= 1) {
    return
  }
  e.preventDefault()
  removeArg(index)
}

function onArgEnter(e, index) {
  if (!Array.isArray(form.args)) {
    form.args = [""]
  }
  if (form.args[index] === "" && index === form.args.length - 1) {
    e.preventDefault()
    return
  }
  e.preventDefault()
  addArg(index)
}

const commandPreview = computed(() => {
  const cmd = form.command ?? ""
  if (cmd === "") {
    return ""
  }
  const args = Array.isArray(form.args) ? form.args.filter((s) => s !== "") : []
  return [cmd, ...args].filter((s) => s !== "").join(" ")
})
</script>

<template>
  <div class="mx-auto flex max-w-[1240px] flex-col gap-4 p-3 sm:p-5 lg:flex-row">
    <aside class="w-full rounded-2xl border border-slate-200 bg-white p-3 shadow-[0_10px_30px_rgba(2,6,23,0.08)] sm:p-3.5 lg:w-[380px]">
      <div class="flex items-start justify-between gap-3 px-3 pt-3 pb-2">
        <div>
          <h2>Jobs</h2>
          <div class="mt-0.5 text-xs text-slate-500">Schedule & run commands</div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            @click="cron.refreshJobs"
          >
            Refresh
          </button>
          <button
            class="appearance-none rounded-xl border border-blue-600/35 bg-blue-600 px-2.5 py-2 text-xs leading-none text-white transition hover:bg-blue-700 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            @click="cron.resetForm"
          >
            New
          </button>
        </div>
      </div>

      <div class="flex flex-col gap-2.5 px-2.5 pb-2.5">
        <div
          v-for="job in jobs"
          :key="job.id"
          class="rounded-xl border border-slate-200 bg-white p-3 data-[selected=true]:border-blue-600/45 data-[selected=true]:ring-4 data-[selected=true]:ring-blue-600/10"
          :data-selected="selectedJobId === job.id"
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
              {{ job.enabled ? "Enabled" : "Disabled" }}
            </span>
          </div>

          <div class="mt-2.5 rounded-xl border border-slate-200 bg-slate-50 px-2.5 py-2 font-mono text-xs text-slate-700">
            {{ job.cron }}
          </div>

          <div class="mt-2.5 flex flex-wrap gap-2">
            <button
              class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
              @click="() => { cron.loadJobToForm(job); cron.loadLogs(job.id) }"
            >
              Edit
            </button>
            <button
              class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50 data-[kind=success]:border-green-600/25 data-[kind=success]:bg-green-50 data-[kind=success]:text-green-800 data-[kind=success]:hover:bg-green-100 data-[kind=muted]:border-slate-200 data-[kind=muted]:bg-slate-100 data-[kind=muted]:text-slate-600 data-[kind=muted]:hover:bg-slate-200"
              :data-kind="job.enabled ? 'muted' : 'success'"
              @click="() => cron.toggleJob(job)"
            >
              {{ job.enabled ? "Disable" : "Enable" }}
            </button>
            <button
              class="appearance-none rounded-xl border border-blue-600/35 bg-blue-600 px-2.5 py-2 text-xs leading-none text-white transition hover:bg-blue-700 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
              @click="() => cron.runNow(job.id)"
            >
              Run Now
            </button>
            <button
              class="appearance-none rounded-xl border border-red-600/25 bg-red-50 px-2.5 py-2 text-xs leading-none text-red-600 transition hover:bg-red-100 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
              @click="() => cron.deleteJob(job.id)"
            >
              Delete
            </button>
          </div>
        </div>

        <div v-if="!jobs.length" class="p-2.5 text-sm text-slate-500">No jobs yet</div>
      </div>
    </aside>

    <main class="min-w-0 flex flex-1 flex-col gap-4">
      <section class="rounded-2xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.08)]">
        <div class="flex items-start justify-between gap-3 px-3 pt-3 pb-2">
          <div>
            <h2>Editor</h2>
            <div class="mt-0.5 text-xs text-slate-500">Create or edit a job</div>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              class="appearance-none rounded-xl border border-blue-600/35 bg-blue-600 px-2.5 py-2 text-xs leading-none text-white transition hover:bg-blue-700 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
              @click="cron.saveJob"
            >
              Save
            </button>
            <button
              v-if="form.id"
              class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
              @click="() => cron.loadLogs(form.id)"
            >
              Load Logs
            </button>
          </div>
        </div>

        <div v-if="error" class="mx-3 mb-3 rounded-xl border border-red-600/25 bg-red-50 px-3 py-2.5 text-sm text-red-800">
          {{ error }}
        </div>

        <div class="grid grid-cols-1 gap-x-3 gap-y-2.5 px-3 pb-2.5 md:grid-cols-[160px_1fr]">
          <label class="text-xs text-slate-500 md:pt-2.5">Name</label>
          <input
            v-model="form.name"
            class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            placeholder="Friendly name"
          />

          <label class="text-xs text-slate-500 md:pt-2.5">Cron</label>
          <input
            v-model="form.cron"
            class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            placeholder="*/1 * * * *"
          />

          <label class="text-xs text-slate-500 md:pt-2.5">Command</label>
          <input
            v-model="form.command"
            class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            placeholder="C:\\Windows\\System32\\notepad.exe"
          />

          <label class="text-xs text-slate-500 md:pt-2.5">Args</label>
          <div>
            <div v-for="(a, i) in form.args" :key="i" class="mb-2 flex flex-wrap items-center gap-2">
              <input
                :ref="(el) => setArgRef(el, i)"
                v-model="form.args[i]"
                class="w-full flex-1 rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
                placeholder="arg"
                @keydown.enter="(e) => onArgEnter(e, i)"
                @keydown.backspace="(e) => onArgBackspace(e, i)"
              />
              <button
                class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
                type="button"
                @click="() => addArg(i)"
              >
                +
              </button>
              <button
                class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
                type="button"
                @click="() => removeArg(i)"
              >
                -
              </button>
            </div>
          </div>

          <label v-if="commandPreview" class="text-xs text-slate-500 md:pt-2.5">Preview</label>
          <div v-if="commandPreview" class="flex flex-col items-stretch gap-2 sm:flex-row">
            <pre class="m-0 flex-1 whitespace-pre-wrap rounded-xl border border-slate-200 bg-slate-100 px-2.5 py-2.5 font-mono text-xs text-slate-900">{{ commandPreview }}</pre>
            <button
              class="w-full appearance-none rounded-xl border border-blue-600/35 bg-blue-600 px-2.5 py-2 text-xs leading-none text-white transition hover:bg-blue-700 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50 sm:w-auto"
              type="button"
              @click="cron.runPreviewFromForm"
            >
              Run
            </button>
          </div>

          <template v-if="showAdvanced">
            <label class="text-xs text-slate-500 md:pt-2.5">WorkDir</label>
            <input
              v-model="form.workDir"
              class="w-full rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
              placeholder="C:\\"
            />
          </template>

          <label class="text-xs text-slate-500 md:pt-2.5">Enabled</label>
          <div class="flex items-center gap-2.5 pt-1.5">
            <input type="checkbox" v-model="form.enabled" />
            <span class="mt-0.5 text-xs text-slate-500">Run on schedule</span>
          </div>

          <template v-if="showAdvanced">
            <label class="text-xs text-slate-500 md:pt-2.5">Disable after consecutive failures</label>
            <div class="flex flex-wrap items-center gap-2.5 pt-1.5">
              <input
                v-model.number="form.maxConsecutiveFailures"
                type="number"
                min="1"
                class="w-full max-w-[220px] rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
                placeholder="3"
              />
              <span class="mt-0.5 text-xs text-slate-500">Auto-disable when reached</span>
            </div>
          </template>
        </div>

        <div class="px-3 pb-3.5">
          <div class="flex justify-center">
            <button
              class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
              type="button"
              @click="() => (showAdvanced = !showAdvanced)"
            >
              {{ showAdvanced ? "Hide advanced options" : "Show advanced options" }}
            </button>
          </div>
        </div>
      </section>

      <section class="rounded-2xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.08)]">
        <div class="flex items-start justify-between gap-3 px-3 pt-3 pb-2">
          <div>
            <h2>Logs</h2>
            <div class="mt-0.5 text-xs text-slate-500">Latest executions (max 100)</div>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              class="appearance-none rounded-xl border border-red-600/25 bg-red-50 px-2.5 py-2 text-xs leading-none text-red-600 transition hover:bg-red-100 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50 disabled:opacity-60 disabled:cursor-not-allowed"
              type="button"
              title="Clear logs"
              :disabled="!logs.length"
              @click="cron.clearLogs"
            >
              🗑️
            </button>
          </div>
        </div>

        <div v-if="!sortedLogs.length" class="p-2.5 text-sm text-slate-500">No logs</div>
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
              {{ l.exitCode === 0 ? "OK" : "FAIL" }}
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
