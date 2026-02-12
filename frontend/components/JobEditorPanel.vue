<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue"
import { storeToRefs } from "pinia"
import { useCronStore } from "../stores/cron.js"
import ArgsEditor from "./ArgsEditor.vue"

const props = defineProps({
  btn: { type: String, required: true },
  btnPrimary: { type: String, required: true },
})

const cron = useCronStore()
const { error, editorVisible, editorPulse } = storeToRefs(cron)
const form = cron.form

const showAdvanced = ref(false)

const validProcessCreationFlags = new Set(["CREATE_NEW_CONSOLE", "CREATE_NO_WINDOW", "DETACHED_PROCESS"])

const normalizeProcessCreationFlag = (value) => {
  const v = String(value ?? "")
  return validProcessCreationFlags.has(v) ? v : ""
}

const flagProcessCreation = computed({
  get: () => {
    return normalizeProcessCreationFlag(form.flagProcessCreation)
  },
  set: (value) => {
    form.flagProcessCreation = normalizeProcessCreationFlag(value)
  },
})

const concurrencyPolicyIndex = computed(() => {
  const v = String(form.concurrencyPolicy || "").toLowerCase()
  if (v === "kill_old") return 1
  if (v === "allow") return 2
  return 0
})

const btnIcon = computed(() => props.btn + " text-base font-semibold")

const cronNextRun = ref("")
const cronNextRunError = ref("")
const cronNextRunPending = ref(false)
const pulseClass = ref("")
const savedHintVisible = ref(false)

let pulseTimer = null
let cronPreviewTimer = null
let cronPreviewSeq = 0

onBeforeUnmount(() => {
  if (cronPreviewTimer) {
    clearTimeout(cronPreviewTimer)
    cronPreviewTimer = null
  }
  if (pulseTimer) {
    clearTimeout(pulseTimer)
    pulseTimer = null
  }
})

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

watch(
  () => editorPulse.value,
  (v) => {
    const kind = v?.kind === "success" ? "success" : v?.kind === "error" ? "error" : ""
    if (!kind) return

    if (pulseTimer) clearTimeout(pulseTimer)
    pulseTimer = null

    pulseClass.value = ""
    savedHintVisible.value = false

    requestAnimationFrame(() => {
      pulseClass.value = kind === "success" ? "editor-pulse-success" : "editor-pulse-error"
      if (kind === "success") savedHintVisible.value = true
    })
    pulseTimer = setTimeout(() => ((pulseClass.value = ""), (savedHintVisible.value = false), (pulseTimer = null)), 2200)
  },
)
</script>

<template>
  <section v-if="editorVisible" class="relative rounded-2xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.08)]" :class="pulseClass">
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
        <ArgsEditor :btn-icon="btnIcon" />
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

        <label class="text-xs text-slate-500 md:pt-2.5">{{ $t("main.fields.flags") }}</label>
        <div class="pt-1.5">
          <div class="flex flex-wrap items-center gap-4">
            <label class="flex items-center gap-2.5">
              <input class="h-5 w-5" type="radio" name="flagProcessCreation" :value="''" v-model="flagProcessCreation" />
              <span class="mt-0.5 text-xs text-slate-500">NONE</span>
            </label>
            <label class="flex items-center gap-2.5">
              <input class="h-5 w-5" type="radio" name="flagProcessCreation" value="CREATE_NEW_CONSOLE" v-model="flagProcessCreation" />
              <span class="mt-0.5 text-xs text-slate-500">CREATE_NEW_CONSOLE</span>
              <span
                class="mt-0.5 inline-flex h-4 w-4 items-center justify-center rounded-full border border-slate-300 text-[10px] text-slate-500 cursor-help"
                :title="$t('main.flags.tooltip.CREATE_NEW_CONSOLE')"
                >?</span
              >
            </label>
            <label class="flex items-center gap-2.5">
              <input class="h-5 w-5" type="radio" name="flagProcessCreation" value="CREATE_NO_WINDOW" v-model="flagProcessCreation" />
              <span class="mt-0.5 text-xs text-slate-500">CREATE_NO_WINDOW</span>
              <span
                class="mt-0.5 inline-flex h-4 w-4 items-center justify-center rounded-full border border-slate-300 text-[10px] text-slate-500 cursor-help"
                :title="$t('main.flags.tooltip.CREATE_NO_WINDOW')"
                >?</span
              >
            </label>
            <label class="flex items-center gap-2.5">
              <input class="h-5 w-5" type="radio" name="flagProcessCreation" value="DETACHED_PROCESS" v-model="flagProcessCreation" />
              <span class="mt-0.5 text-xs text-slate-500">DETACHED_PROCESS</span>
              <span
                class="mt-0.5 inline-flex h-4 w-4 items-center justify-center rounded-full border border-slate-300 text-[10px] text-slate-500 cursor-help"
                :title="$t('main.flags.tooltip.DETACHED_PROCESS')"
                >?</span
              >
            </label>
          </div>
        </div>

        <label class="text-xs text-slate-500 md:pt-2.5">{{ $t("main.fields.timeout") }}</label>
        <div class="flex flex-wrap items-center gap-2.5 pt-1.5">
          <input
            v-model.number="form.timeout"
            type="number"
            min="0"
            class="w-[120px] rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            :placeholder="$t('main.placeholders.timeout')"
          />
          <span class="mt-0.5 text-xs text-slate-500">{{ $t("main.timeout_help") }}</span>
        </div>

        <label class="text-xs text-slate-500 md:pt-2.5">{{ $t("main.fields.concurrency_policy") }}</label>
        <div class="pt-1.5">
          <div class="relative grid grid-cols-3 rounded-full border border-slate-200 bg-slate-50 p-0.5">
            <div
              class="pointer-events-none absolute left-0 top-0 bottom-0 m-0.5 rounded-full bg-white shadow transition-transform duration-200"
              :style="{ width: '33.333333%', transform: `translateX(${concurrencyPolicyIndex * 100}%)` }"
            />
            <button
              type="button"
              class="relative z-10 rounded-full px-3 py-2 text-xs transition"
              :class="concurrencyPolicyIndex === 0 ? 'text-slate-900' : 'text-slate-500 hover:text-slate-700'"
              @click="form.concurrencyPolicy = 'skip'"
            >
              {{ $t("main.concurrency_policy.skip") }}
            </button>
            <button
              type="button"
              class="relative z-10 rounded-full px-3 py-2 text-xs transition"
              :class="concurrencyPolicyIndex === 1 ? 'text-slate-900' : 'text-slate-500 hover:text-slate-700'"
              @click="form.concurrencyPolicy = 'kill_old'"
            >
              {{ $t("main.concurrency_policy.terminate_old") }}
            </button>
            <button
              type="button"
              class="relative z-10 rounded-full px-3 py-2 text-xs transition"
              :class="concurrencyPolicyIndex === 2 ? 'text-slate-900' : 'text-slate-500 hover:text-slate-700'"
              @click="form.concurrencyPolicy = 'allow'"
            >
              {{ $t("main.concurrency_policy.allow") }}
            </button>
          </div>
        </div>

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

        <label class="text-xs text-slate-500 md:pt-2.5">{{ $t("main.fields.enabled") }}</label>
        <div class="flex items-center gap-2.5 pt-1.5">
          <input class="h-5 w-5" type="checkbox" v-model="form.enabled" />
          <span class="mt-0.5 text-xs text-slate-500">{{ form.id ? $t("main.enabled_help") : $t("main.enabled_help_create") }}</span>
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

    <div v-if="savedHintVisible" class="editor-saved-hint pointer-events-none absolute right-4 bottom-3 text-xs font-semibold">
      {{ $t("toast.saved") }}
    </div>
  </section>
</template>

<style scoped>
 .editor-pulse-success,
 .editor-pulse-error {
  animation-fill-mode: both;
  animation: editorPulseShadow 1.9s ease-out 0s 1;
 }

 .editor-pulse-success {
  --pulse-rgb: 34, 197, 94;
 }

 .editor-pulse-error {
  --pulse-rgb: 239, 68, 68;
 }

 @keyframes editorPulseShadow {
  0% {
    box-shadow:
      0 10px 30px rgba(2, 6, 23, 0.08),
      0 0 0 0 rgba(var(--pulse-rgb), 0),
      0 0 0 rgba(var(--pulse-rgb), 0);
  }
  30% {
    box-shadow:
      0 10px 30px rgba(2, 6, 23, 0.08),
      0 0 0 2px rgba(var(--pulse-rgb), 0.2),
      0 0 28px rgba(var(--pulse-rgb), 0.28);
  }
  75% {
    box-shadow:
      0 10px 30px rgba(2, 6, 23, 0.08),
      0 0 0 2px rgba(var(--pulse-rgb), 0.06),
      0 0 20px rgba(var(--pulse-rgb), 0.12);
  }
  100% {
    box-shadow:
      0 10px 30px rgba(2, 6, 23, 0.08),
      0 0 0 0 rgba(var(--pulse-rgb), 0),
      0 0 0 rgba(var(--pulse-rgb), 0);
  }
 }

 .editor-saved-hint {
  color: rgb(var(--pulse-rgb, 34, 197, 94));
  opacity: 0;
  text-shadow: 0 1px 10px rgba(var(--pulse-rgb, 34, 197, 94), 0.25);
  animation: editorSavedHint 1.9s ease-out 0s 1;
 }

 @keyframes editorSavedHint {
  0% {
    opacity: 0;
    transform: translateY(4px);
  }
  22% {
    opacity: 1;
    transform: translateY(0);
  }
  75% {
    opacity: 0.7;
    transform: translateY(-1px);
  }
  100% {
    opacity: 0;
    transform: translateY(-2px);
  }
 }
</style>
