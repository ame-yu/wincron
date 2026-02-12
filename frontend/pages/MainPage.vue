<script setup>
import { onBeforeUnmount, onMounted } from "vue"
import { useCronStore } from "../stores/cron.js"
import { btn, btnDanger, btnPrimary } from "../ui/buttonClasses.js"
import JobListPanel from "../components/JobListPanel.vue"
import JobEditorPanel from "../components/JobEditorPanel.vue"
import LogsPanel from "../components/LogsPanel.vue"
import AppScrollbar from "../components/AppScrollbar.vue"

const cron = useCronStore()

const onGlobalKeydown = (e) => {
  if (e?.repeat) {
    return
  }
  const key = typeof e?.key === "string" ? e.key.toLowerCase() : ""
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && key === "n") {
    e.preventDefault()
    window.dispatchEvent(new CustomEvent("wincron:new-folder"))
    return
  }
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && key === "n") {
    e.preventDefault()
    cron.resetForm()
  }
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
</script>

<template>
  <div class="mx-auto flex flex-1 h-full max-w-[1240px] flex-col gap-4 p-3 sm:p-5 lg:flex-row lg:items-stretch min-h-0">
    <JobListPanel :btn="btn" :btn-primary="btnPrimary" :btn-danger="btnDanger" />

    <AppScrollbar root-class="min-w-0 min-h-0 flex flex-1" view-class="flex flex-col gap-4 p-2.5">
      <JobEditorPanel :btn="btn" :btn-primary="btnPrimary" />
      <LogsPanel :btn-danger="btnDanger" />
    </AppScrollbar>
  </div>
</template>
