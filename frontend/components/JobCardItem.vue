<script setup>
import { computed } from "vue"

const props = defineProps({
  job: { type: Object, required: true },
  selected: { type: Boolean, default: false },
  inFolder: { type: Boolean, default: false },
  btn: { type: String, required: true },
  btnPrimary: { type: String, required: true },
  btnDanger: { type: String, required: true },
  formatNextRun: { type: Function, required: true },
})

const emit = defineEmits(["select", "edit", "toggle", "run", "delete", "contextmenu", "dragstart", "dragover", "drop", "dragend"])

const toggleBtnClass = computed(() => props.btn + " cursor-pointer data-[kind=success]:border-green-600/25 data-[kind=success]:bg-green-50 data-[kind=success]:text-green-800 data-[kind=success]:hover:bg-green-100 data-[kind=muted]:bg-slate-100 data-[kind=muted]:text-slate-600 data-[kind=muted]:hover:bg-slate-200")

const cardClass = computed(() => "rounded-xl border border-slate-200 bg-white active:cursor-grabbing data-[selected=true]:border-blue-600/45 data-[selected=true]:ring-4 data-[selected=true]:ring-blue-600/10 " + (props.inFolder ? "p-2.5 ml-4" : "p-3"))

const hotkeyRaw = computed(() => String(props.job?.hotkey || "").trim())

const hotkeyCompact = computed(() => {
  if (!hotkeyRaw.value) return ""
  const mods = { CTRL: "⌃", ALT: "⌥", SHIFT: "⇧", WIN: "⌘", WINDOWS: "⌘", META: "⌘", CONTROL: "⌃" }
  const parts = hotkeyRaw.value.split("+").map(s => s.trim()).filter(Boolean)
  const out = []
  let key = ""
  for (const p of parts) {
    const u = p.toUpperCase()
    if (mods[u]) out.push(mods[u])
    else key = p
  }
  return key ? out.join("") + key : ""
})

const onEdit = () => emit("edit", props.job)
</script>

<template>
  <div :class="cardClass" :data-selected="selected" draggable="true" @dragstart="emit('dragstart', $event, job.id)" @dragend="emit('dragend', $event, job.id)" @dragover.prevent.stop="emit('dragover', $event, job.id)" @drop.prevent.stop="emit('drop', $event, job.id)" @click="emit('select', $event, job.id)" @dblclick="onEdit" @contextmenu.prevent.stop="emit('contextmenu', $event, job)">
    <div class="flex justify-between gap-2.5" draggable="true">
      <div class="min-w-0">
        <div class="overflow-hidden text-ellipsis whitespace-nowrap text-xs font-semibold">{{ job.name || job.command }}</div>
        <div class="mt-0.5 overflow-hidden text-ellipsis whitespace-nowrap text-xs text-slate-500">{{ job.command }}</div>
      </div>
      <span class="h-fit shrink-0 whitespace-nowrap rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 text-[11px] text-slate-500 data-[kind=success]:border-green-600/25 data-[kind=success]:bg-green-50 data-[kind=success]:text-green-800 data-[kind=muted]:border-slate-200 data-[kind=muted]:bg-slate-100 data-[kind=muted]:text-slate-600" :data-kind="job.enabled ? 'success' : 'muted'">{{ job.enabled ? $t("common.enabled") : $t("common.disabled") }}</span>
    </div>
    <div class="mt-2.5 flex items-center justify-between gap-2 rounded-xl border border-slate-200 bg-slate-50 px-2.5 py-2 text-xs text-slate-700" :title="formatNextRun(job)" draggable="true">
      <span class="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap" :class="job.enabled ? '' : 'line-through decoration-slate-500 text-slate-400'">{{ job.cron || $t("common.cron_not_set") }}</span>
      <span v-if="hotkeyCompact" class="shrink-0 px-1 text-base font-medium" :class="job.enabled ? 'text-slate-500' : 'text-slate-400 line-through decoration-slate-400'" :title="hotkeyRaw">{{ hotkeyCompact }}</span>
    </div>
    <div class="mt-2.5 flex flex-wrap gap-2">
      <button :class="btn" @click.stop="onEdit">{{ $t("common.edit") }}</button>
      <button :class="toggleBtnClass" :data-kind="job.enabled ? 'muted' : 'success'" @click.stop="emit('toggle', job)">{{ job.enabled ? $t("common.disable") : $t("common.enable") }}</button>
      <button :class="btnPrimary" @click.stop="emit('run', job)">{{ $t("common.run_now") }}</button>
      <button :class="btnDanger" @click.stop="emit('delete', job.id)">{{ $t("common.delete") }}</button>
    </div>
  </div>
</template>
