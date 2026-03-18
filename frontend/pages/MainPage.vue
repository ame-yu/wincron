<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { storeToRefs } from "pinia"
import { useCronStore } from "../stores/cron.js"
import { LOG_PAGE_SIZE } from "../stores/cron/logs.js"
import { btn, btnDanger, btnPrimary } from "../ui/buttonClasses.js"
import JobListPanel from "../components/JobListPanel.vue"
import JobEditorPanel from "../components/JobEditorPanel.vue"
import LogsPanel from "../components/LogsPanel.vue"
import AppScrollbar from "../components/AppScrollbar.vue"
import SearchPalette from "../components/SearchPalette.vue"

const LOAD_MORE_THRESHOLD = 160

const cron = useCronStore()
const {
  editorVisible,
  logs,
  logsLoading,
  logsLoadingMore,
  logsHasMore,
  logsTotalCount,
} = storeToRefs(cron)
const searchVisible = ref(false)
const rightScrollbarRef = ref(null)
const currentBottomLogIndex = ref(0)
let rightScrollEl = null
let rightScrollRafId = 0

const visibleLogCount = computed(() => (Array.isArray(logs.value) ? logs.value.length : 0))
const totalLogCount = computed(() => Math.max(Number(logsTotalCount.value) || 0, visibleLogCount.value))
const needsMoreLogs = computed(() => logsHasMore.value || visibleLogCount.value < totalLogCount.value)
const scrollBadgeText = computed(() => (totalLogCount.value > 0 && currentBottomLogIndex.value > 0 ? `${currentBottomLogIndex.value}/${totalLogCount.value}` : ""))

const getRightWrapEl = () => {
  const wrap = rightScrollbarRef.value?.wrapRef
  return wrap?.value || wrap || null
}

function onSearchOpen() {
  searchVisible.value = true
}

function updateRightScrollState() {
  const wrap = getRightWrapEl()
  if (!wrap) {
    currentBottomLogIndex.value = 0
    return
  }

  const wrapRect = wrap.getBoundingClientRect()
  let nextIndex = 0
  let rowIndex = 0

  for (const row of wrap.querySelectorAll("[data-log-id]")) {
    rowIndex += 1
    const { top, bottom } = row.getBoundingClientRect()
    if (bottom <= wrapRect.top) continue
    if (top >= wrapRect.bottom) break
    nextIndex = rowIndex
  }
  currentBottomLogIndex.value = nextIndex
}

const onGlobalKeydown = (e) => {
  if (e?.repeat) {
    return
  }
  const key = typeof e?.key === "string" ? e.key.toLowerCase() : ""
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && key === "f") {
    e.preventDefault()
    searchVisible.value = true
    return
  }
  if (searchVisible.value) {
    if (key === "escape") {
      e.preventDefault()
      searchVisible.value = false
    }
    return
  }
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

const detachRightScroll = () => {
  if (!rightScrollEl) return
  rightScrollEl.removeEventListener("scroll", onRightScroll)
  rightScrollEl = null
}

async function loadMoreFromRightScroll() {
  const wrap = getRightWrapEl()
  if (!wrap || logsLoading.value || logsLoadingMore.value || !needsMoreLogs.value) {
    return
  }
  const remaining = wrap.scrollHeight - wrap.scrollTop - wrap.clientHeight
  if (remaining > LOAD_MORE_THRESHOLD) {
    return
  }
  await cron.loadMoreLogs()
  await nextTick()
}

function onRightScroll() {
  if (!rightScrollRafId) {
    rightScrollRafId = requestAnimationFrame(() => {
      rightScrollRafId = 0
      updateRightScrollState()
    })
  }
  void loadMoreFromRightScroll()
}

async function scrollLogsToTop() {
  const wrap = getRightWrapEl()
  if (!wrap) {
    return
  }
  wrap.scrollTop = 0
  if (visibleLogCount.value > LOG_PAGE_SIZE || logsHasMore.value) {
    await cron.reloadFocusedLogs()
    await nextTick()
  }
  updateRightScrollState()
}

watch([logs, editorVisible], () => nextTick(updateRightScrollState), { flush: "post" })

onMounted(() => {
  window.addEventListener("keydown", onGlobalKeydown)
  window.addEventListener("wincron:open-search", onSearchOpen)
  void nextTick(() => {
    rightScrollEl = getRightWrapEl()
    rightScrollEl?.addEventListener("scroll", onRightScroll, { passive: true })
    updateRightScrollState()
  })
})

onBeforeUnmount(() => {
  window.removeEventListener("keydown", onGlobalKeydown)
  window.removeEventListener("wincron:open-search", onSearchOpen)
  if (rightScrollRafId) {
    cancelAnimationFrame(rightScrollRafId)
    rightScrollRafId = 0
  }
  detachRightScroll()
})
</script>

<template>
  <div class="mx-auto flex h-full min-h-0 max-w-[1240px] flex-1 flex-col md:flex-row md:items-stretch">
    <div class="pt-2 md:pl-3 md:pt-3 md:pb-2">
      <JobListPanel :btn="btn" :btn-primary="btnPrimary" :btn-danger="btnDanger" />
    </div>

    <AppScrollbar
      ref="rightScrollbarRef"
      root-class="min-w-0 min-h-0 flex flex-1"
      wrap-class="min-h-0"
      view-class="flex h-full min-h-0 flex-col gap-3 px-2 pt-2 md:gap-4 md:px-3 md:pt-3"
      :hide-bars="true"
      :scroll-badge-text="scrollBadgeText"
      :show-scroll-top-button="totalLogCount > 0"
      :scroll-top-button-title="$t('main.logs.back_to_top')"
      @scroll-top="scrollLogsToTop"
    >
      <div v-if="editorVisible">
        <JobEditorPanel :btn="btn" :btn-primary="btnPrimary" />
      </div>
      <LogsPanel />
    </AppScrollbar>
  </div>
  <SearchPalette :visible="searchVisible" @update:visible="searchVisible = $event" />
</template>
