<script setup>
import { computed, ref, watch } from "vue"
import { storeToRefs } from "pinia"
import { useCronStore } from "../stores/cron.js"
import ModalShell from "./ModalShell.vue"

const props = defineProps({
  btn: { type: String, required: true },
  btnPrimary: { type: String, required: true },
  btnDanger: { type: String, required: true },
})

const show = ref(false)

const cron = useCronStore()
const { jobs } = storeToRefs(cron)

const hotkeyJobs = computed(() => {
  const list = Array.isArray(jobs.value) ? jobs.value : []
  return list
    .filter((j) => String(j?.hotkey || "").trim())
    .slice()
    .sort((a, b) => String(a?.hotkey || "").localeCompare(String(b?.hotkey || "")))
})

watch(show, async (v) => {
  if (!v) return
  await cron.refreshJobs()
})

const toggleEnabled = (job) => job?.id && cron.toggleJob(job)
const unbindHotkey = (job) => job?.id && cron.setJobHotkey(String(job.id), "")

const toggleBtnClass = props.btn + " cursor-pointer data-[kind=success]:border-green-600/25 data-[kind=success]:bg-green-50 data-[kind=success]:text-green-800 data-[kind=success]:hover:bg-green-100 data-[kind=muted]:bg-slate-100 data-[kind=muted]:text-slate-600 data-[kind=muted]:hover:bg-slate-200"
</script>

<template>
  <button :class="btn" @click="show = true">{{ $t("settings.shortcut_guide") }}</button>

  <ModalShell v-model="show" :max-width="560">
    <div class="mt-3 rounded-xl border border-slate-200 bg-slate-50 p-3">
      <div class="text-sm font-semibold text-slate-900 pb-2">{{ $t("settings.shortcuts_subtitle") }}</div>
      <div class="grid grid-cols-[140px_1fr] gap-x-3 gap-y-2 text-sm px-3">
        <div class="text-xs text-slate-700">Ctrl + N</div>
        <div class="text-slate-900">{{ $t("settings.shortcuts.new") }}</div>
        <div class="text-xs text-slate-700">Ctrl + Shift + N</div>
        <div class="text-slate-900">{{ $t("settings.shortcuts.new_folder") }}</div>
        <div class="text-xs text-slate-700">Ctrl + S</div>
        <div class="text-slate-900">{{ $t("settings.shortcuts.save") }}</div>
        <div class="text-xs text-slate-700">Ctrl + F</div>
        <div class="text-slate-900">{{ $t("settings.shortcuts.search") }}</div>
      </div>
    </div>

    <div class="mt-3 rounded-xl border border-slate-200 bg-slate-50 p-3">
      <div class="text-sm font-semibold text-slate-900">{{ $t("settings.hotkeys_title") }}</div>

      <div v-if="!hotkeyJobs.length" class="mt-2 text-xs text-slate-500">{{ $t("settings.hotkeys_empty") }}</div>
      <div v-else class="mt-2 flex flex-col gap-2">
        <div
          v-for="j in hotkeyJobs"
          :key="j.id"
          class="flex items-center justify-between gap-3 rounded-xl border border-slate-200 bg-white px-3 py-2"
        >
          <div class="min-w-0">
            <div class="text-xs text-slate-700">{{ j.hotkey }}</div>
            <div class="truncate text-sm text-slate-900">{{ j.name }}</div>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <button :class="toggleBtnClass" :data-kind="j.enabled ? 'muted' : 'success'" type="button" @click="toggleEnabled(j)">{{ j.enabled ? $t("common.disable") : $t("common.enable") }}</button>
            <button :class="btnDanger" type="button" @click="unbindHotkey(j)">{{ $t("common.unbind") }}</button>
          </div>
        </div>
      </div>
    </div>

    <div class="mt-4 flex justify-end">
      <button :class="btnPrimary" @click="show = false">{{ $t("nav.back") }}</button>
    </div>
  </ModalShell>
</template>
