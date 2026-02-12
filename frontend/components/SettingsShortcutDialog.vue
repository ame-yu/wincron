<script setup>
import { ref } from "vue"
import ModalShell from "./ModalShell.vue"

defineProps({
  btn: { type: String, required: true },
  btnPrimary: { type: String, required: true },
})

const show = ref(false)
</script>

<template>
  <button :class="btn" @click="show = true">{{ $t("settings.shortcut_guide") }}</button>

  <ModalShell v-model="show" :max-width="560">
      <div>
        <h3>{{ $t("settings.shortcuts_title") }}</h3>
        <div class="mt-0.5 text-xs text-slate-500">{{ $t("settings.shortcuts_subtitle") }}</div>
      </div>

      <div class="mt-3 rounded-xl border border-slate-200 bg-slate-50 p-3">
        <div class="grid grid-cols-[140px_1fr] gap-x-3 gap-y-2 text-sm">
          <div class="font-mono text-xs text-slate-700">Ctrl/⌘ + N</div>
          <div class="text-slate-900">{{ $t("settings.shortcuts.new") }}</div>

          <div class="font-mono text-xs text-slate-700">Ctrl/⌘ + Shift + N</div>
          <div class="text-slate-900">{{ $t("settings.shortcuts.new_folder") }}</div>

          <div class="font-mono text-xs text-slate-700">Ctrl/⌘ + S</div>
          <div class="text-slate-900">{{ $t("settings.shortcuts.save") }}</div>
        </div>
      </div>

      <div class="mt-4 flex justify-end gap-2">
        <button :class="btnPrimary" @click="show = false">{{ $t("nav.back") }}</button>
      </div>
  </ModalShell>
</template>
