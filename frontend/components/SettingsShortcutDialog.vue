<script setup>
import { ref } from "vue"

defineProps({
  btn: { type: String, required: true },
  btnPrimary: { type: String, required: true },
})

const show = ref(false)
</script>

<template>
  <button :class="btn" @click="show = true">{{ $t("settings.shortcut_guide") }}</button>

  <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/20 p-4" @click.self="show = false">
    <div class="w-full max-w-[560px] rounded-2xl border border-slate-200 bg-white p-4 shadow-[0_10px_30px_rgba(2,6,23,0.16)]">
      <div>
        <h3>{{ $t("settings.shortcuts_title") }}</h3>
        <div class="mt-0.5 text-xs text-slate-500">{{ $t("settings.shortcuts_subtitle") }}</div>
      </div>

      <div class="mt-3 rounded-xl border border-slate-200 bg-slate-50 p-3">
        <div class="grid grid-cols-[140px_1fr] gap-x-3 gap-y-2 text-sm">
          <div class="font-mono text-xs text-slate-700">Ctrl/⌘ + N</div>
          <div class="text-slate-900">{{ $t("settings.shortcuts.new") }}</div>

          <div class="font-mono text-xs text-slate-700">Ctrl/⌘ + S</div>
          <div class="text-slate-900">{{ $t("settings.shortcuts.save") }}</div>
        </div>
      </div>

      <div class="mt-4 flex justify-end gap-2">
        <button :class="btnPrimary" @click="show = false">{{ $t("common.ok") }}</button>
      </div>
    </div>
  </div>
</template>
