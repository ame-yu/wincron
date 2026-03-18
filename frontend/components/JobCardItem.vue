<script setup>
import { computed } from "vue"
import { useI18n } from "vue-i18n"

const props = defineProps({
  job: { type: Object, required: true },
  selected: { type: Boolean, default: false },
  running: { type: Boolean, default: false },
  inFolder: { type: Boolean, default: false },
  btn: { type: String, required: true },
  btnPrimary: { type: String, required: true },
  btnDanger: { type: String, required: true },
  formatNextRun: { type: Function, required: true },
})

const emit = defineEmits(["select", "edit", "toggle", "run", "terminate", "delete", "contextmenu", "dragstart", "dragover", "drop", "dragend"])

const { t } = useI18n()

const baseButtonClass =
  "job-card__action-button flex w-full min-w-0 items-center justify-center overflow-hidden text-ellipsis whitespace-nowrap px-1.5 py-1 text-[11px] sm:px-2 sm:py-1.5 sm:text-xs disabled:cursor-not-allowed disabled:opacity-45"

const toggleBtnClass = computed(() =>
  `${props.btn} ${baseButtonClass} cursor-pointer data-[kind=success]:border-green-600/25 data-[kind=success]:bg-green-50 data-[kind=success]:text-green-800 data-[kind=success]:hover:bg-green-100 data-[kind=muted]:bg-slate-100 data-[kind=muted]:text-slate-600 data-[kind=muted]:hover:bg-slate-200`,
)

const secondaryButtonClass = computed(() => `${props.btn} ${baseButtonClass}`)
const dangerButtonClass = computed(() => `${props.btnDanger} ${baseButtonClass}`)
const primaryActionClass = computed(() =>
  `${props.running ? props.btnDanger : props.btnPrimary} ${baseButtonClass} ${props.running ? "job-card__terminate-button" : ""}`,
)

const cardClass = computed(() => [
  "job-card rounded-xl border border-slate-200 bg-white active:cursor-grabbing data-[selected=true]:border-blue-600/45 data-[selected=true]:ring-4 data-[selected=true]:ring-blue-600/10 data-[flash=true]:border-amber-500/40 data-[flash=true]:ring-4 data-[flash=true]:ring-amber-500/20",
  props.inFolder ? "ml-3 p-2 sm:ml-4 sm:p-2.5" : "p-2.5 sm:p-3",
  props.running ? "job-card--running" : "",
].join(" "))

const displayName = computed(() => String(props.job?.name || props.job?.command || "").trim())
const commandText = computed(() => String(props.job?.command || "").trim())
const statusKind = computed(() => (props.running ? "running" : props.job?.enabled ? "success" : "muted"))
const statusText = computed(() => {
  if (props.running) return t("common.running")
  return props.job?.enabled ? t("common.enabled") : t("common.disabled")
})

const hotkeyRaw = computed(() => String(props.job?.hotkey || "").trim())

const hotkeyCompact = computed(() => {
  if (!hotkeyRaw.value) return ""
  const mods = {
    ALT: "\u2325",
    CONTROL: "\u2303",
    CTRL: "\u2303",
    META: "\u2318",
    SHIFT: "\u21e7",
    WIN: "\u2318",
    WINDOWS: "\u2318",
  }
  const parts = hotkeyRaw.value.split("+").map((value) => value.trim()).filter(Boolean)
  const out = []
  let key = ""

  for (const part of parts) {
    const upper = part.toUpperCase()
    if (mods[upper]) out.push(mods[upper])
    else key = part
  }

  return key ? `${out.join("")}${key}` : ""
})

function onEdit() {
  if (props.running) return
  emit("edit", props.job)
}

function onPrimaryAction() {
  emit(props.running ? "terminate" : "run", props.job)
}

function onCardDoubleClick() {
  if (props.running) return
  emit("edit", props.job)
}
</script>

<template>
  <div
    :class="cardClass"
    :data-selected="selected"
    :data-running="running"
    draggable="true"
    @dragstart="emit('dragstart', $event, job.id)"
    @dragend="emit('dragend', $event, job.id)"
    @dragover.prevent.stop="emit('dragover', $event, job.id)"
    @drop.prevent.stop="emit('drop', $event, job.id)"
    @click="emit('select', $event, job.id)"
    @dblclick="onCardDoubleClick"
    @contextmenu.prevent.stop="emit('contextmenu', $event, job)"
  >
    <div class="job-card__frame" aria-hidden="true"></div>

    <div class="job-card__header flex items-start justify-between gap-2">
      <div class="min-w-0">
        <div class="overflow-hidden text-ellipsis whitespace-nowrap text-xs font-semibold text-slate-900">
          {{ displayName }}
        </div>
        <div class="mt-0.5 overflow-hidden text-ellipsis whitespace-nowrap text-xs text-slate-500">
          {{ commandText }}
        </div>
      </div>

      <span
        class="shrink-0 whitespace-nowrap rounded-full border border-slate-200 bg-slate-50 px-2 py-0.5 text-[10px] text-slate-500 data-[kind=running]:border-amber-500/35 data-[kind=running]:bg-amber-50 data-[kind=running]:text-amber-800 data-[kind=success]:border-green-600/25 data-[kind=success]:bg-green-50 data-[kind=success]:text-green-800 data-[kind=muted]:border-slate-200 data-[kind=muted]:bg-slate-100 data-[kind=muted]:text-slate-600 sm:px-2.5 sm:py-1 sm:text-[11px]"
        :data-kind="statusKind"
      >
        {{ statusText }}
      </span>
    </div>

    <div
      class="job-card__schedule mt-2 flex items-center justify-between gap-2 rounded-xl border border-slate-200 bg-slate-50 px-2 py-1.5 text-xs text-slate-700 sm:mt-2.5 sm:px-2.5 sm:py-2"
      :title="formatNextRun(job)"
    >
      <span
        class="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-xs"
        :class="job.enabled ? '' : 'text-slate-400 line-through decoration-slate-500'"
      >
        {{ job.cron || $t("common.cron_not_set") }}
      </span>

      <span
        v-if="hotkeyCompact"
        class="shrink-0 px-1 text-sm font-medium sm:text-base"
        :class="job.enabled ? 'text-slate-500' : 'text-slate-400 line-through decoration-slate-400'"
        :title="hotkeyRaw"
      >
        {{ hotkeyCompact }}
      </span>
    </div>

    <div class="job-card__actions mt-2 grid items-end gap-1.5 overflow-visible sm:mt-2.5">
      <button
        type="button"
        :class="secondaryButtonClass"
        :disabled="running"
        @click.stop="onEdit"
      >
        {{ $t("common.edit") }}
      </button>

      <button
        type="button"
        :class="toggleBtnClass"
        :data-kind="job.enabled ? 'muted' : 'success'"
        :disabled="running"
        @click.stop="emit('toggle', job)"
      >
        {{ job.enabled ? $t("common.disable") : $t("common.enable") }}
      </button>

      <div class="job-card__primary-slot">
        <div v-if="running" class="job-card__running-fire-glow" aria-hidden="true"></div>

        <div v-if="running" class="job-card__running-fire" aria-hidden="true">
          <div class="job-card__flames">
            <div class="job-card__flame"></div>
            <div class="job-card__flame"></div>
            <div class="job-card__flame"></div>
            <div class="job-card__flame"></div>
          </div>
          <div class="job-card__ember job-card__ember--one"></div>
          <div class="job-card__ember job-card__ember--two"></div>
        </div>

        <button
          type="button"
          :class="primaryActionClass"
          @click.stop="onPrimaryAction"
        >
          {{ running ? $t("common.terminate_now") : $t("common.run_now") }}
        </button>
      </div>

      <button
        type="button"
        :class="dangerButtonClass"
        :disabled="running"
        @click.stop="emit('delete', job.id)"
      >
        {{ $t("common.delete") }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.job-card {
  position: relative;
  isolation: isolate;
  overflow: visible;
  --job-card-fire-left: calc(50% - 3ch);
}

.job-card--running {
  border-color: rgba(251, 146, 60, 0.32);
  background:
    radial-gradient(circle at 78% 112%, rgba(251, 146, 60, 0.22), transparent 36%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.9), rgba(255, 247, 237, 0.72));
  box-shadow: 0 14px 34px rgba(251, 146, 60, 0.12);
}

.job-card__frame {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  pointer-events: none;
}

.job-card__header,
.job-card__schedule {
  position: relative;
  z-index: 2;
}

.job-card[data-running="true"] .job-card__frame {
  z-index: 1;
  border: 1px solid rgba(251, 146, 60, 0.34);
}

.job-card[data-running="true"] .job-card__schedule {
  border-color: rgba(251, 146, 60, 0.28);
  background: linear-gradient(180deg, rgba(255, 247, 237, 0.62), rgba(255, 255, 255, 0.42));
}

.job-card__action-button {
  position: relative;
  z-index: 3;
}

.job-card__actions {
  grid-template-columns: minmax(0, 0.88fr) minmax(0, 0.98fr) minmax(0, 1.4fr) minmax(0, 0.9fr);
}

.job-card__primary-slot {
  position: relative;
  display: flex;
  min-width: 0;
  width: 100%;
  align-items: flex-end;
  justify-content: center;
  overflow: visible;
}

.job-card__terminate-button {
  box-shadow: 0 10px 20px rgba(194, 65, 12, 0.18);
}

.job-card__running-fire-glow,
.job-card__running-fire {
  position: absolute;
  left: var(--job-card-fire-left);
  z-index: 0;
  pointer-events: none;
}

.job-card__running-fire-glow {
  bottom: -1.35rem;
  width: 6.8rem;
  height: 3.84rem;
  transform: translateX(-20%);
  border-radius: 9999px;
  background: radial-gradient(circle, rgba(251, 191, 36, 0.48) 0%, rgba(249, 115, 22, 0.32) 38%, rgba(249, 115, 22, 0) 76%);
  filter: blur(13px);
  opacity: 0.95;
}

.job-card__running-fire {
  bottom: -1.8rem;
  width: 5.44rem;
  height: 7.2rem;
  transform: translateX(-18%);
}

.job-card__flames {
  position: absolute;
  bottom: 0.88rem;
  left: 46%;
  width: 72%;
  height: 74%;
  transform: translateX(-50%) rotate(45deg);
}

.job-card__flame {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 0;
  height: 0;
  border-radius: 0.72rem;
  background-color: #ffdc01;
  filter: drop-shadow(0 0 10px rgba(249, 115, 22, 0.44));
}

.job-card__flame:nth-child(odd) {
  animation: job-card-flame-odd 1.45s ease-in infinite;
}

.job-card__flame:nth-child(even) {
  animation: job-card-flame-even 1.45s ease-in infinite;
}

.job-card__flame:nth-child(1) {
  animation-delay: 0s;
}

.job-card__flame:nth-child(2) {
  animation-delay: 0.36s;
}

.job-card__flame:nth-child(3) {
  animation-delay: 0.72s;
}

.job-card__flame:nth-child(4) {
  animation-delay: 1.08s;
}

.job-card__ember {
  position: absolute;
  bottom: 1.24rem;
  z-index: 2;
  width: 0.4rem;
  height: 0.4rem;
  border-radius: 9999px;
  background: rgba(255, 237, 160, 0.95);
  box-shadow: 0 0 13px rgba(249, 115, 22, 0.56);
  opacity: 0;
}

.job-card__ember--one {
  left: 1.36rem;
  animation: job-card-ember 1.9s linear infinite;
}

.job-card__ember--two {
  right: 1.08rem;
  animation: job-card-ember 1.9s linear 0.8s infinite;
}

@keyframes job-card-flame-odd {
  0%,
  100% {
    right: 0;
    bottom: 0;
    width: 0;
    height: 0;
    background-color: #ffdc01;
    z-index: 2;
  }

  25% {
    right: 1%;
    bottom: 2%;
    width: 100%;
    height: 100%;
  }

  40% {
    background-color: #fdac01;
  }

  100% {
    right: 150%;
    bottom: 170%;
    background-color: #f73b01;
    z-index: 0;
  }
}

@keyframes job-card-flame-even {
  0%,
  100% {
    right: 0;
    bottom: 0;
    width: 0;
    height: 0;
    background-color: #ffdc01;
    z-index: 2;
  }

  25% {
    right: 2%;
    bottom: 1%;
    width: 100%;
    height: 100%;
  }

  40% {
    background-color: #fdac01;
  }

  100% {
    right: 170%;
    bottom: 150%;
    background-color: #f73b01;
    z-index: 0;
  }
}

@keyframes job-card-ember {
  0% {
    transform: translateY(0) scale(0.8);
    opacity: 0;
  }

  18% {
    opacity: 1;
  }

  100% {
    transform: translateY(-4.6rem) scale(0.2);
    opacity: 0;
  }
}
</style>
