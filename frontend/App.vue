<script setup>
import { computed, onMounted, onUnmounted, watch } from "vue"
import { storeToRefs } from "pinia"
import { useRoute, useRouter } from "vue-router"
import { Events, Window } from "@wailsio/runtime"
import { useCronStore } from "./stores/cron.js"

const cron = useCronStore()
const { toast, toastKind, globalEnabled } = storeToRefs(cron)

const router = useRouter()
const route = useRoute()
const isSettings = computed(() => route.name === "settings")

watch(
  () => route.name,
  (name) => {
    const base = "WinCron"
    const suffix = name ? String(name) : ""
    const title = suffix ? `${base} - ${suffix}` : base
    document.title = title
    Window.SetTitle(title).catch(() => {})
  },
  { immediate: true },
)

function goSettings() {
  router.push({ name: "settings" })
}

function goBack() {
  router.push({ name: "main" })
}

async function toggleGlobalEnabled() {
  try {
    await cron.setGlobalEnabled(!globalEnabled.value)
  } catch {
    // noop
  }
}

let offNavigate = null
let offGlobalEnabledChanged = null

onMounted(async () => {
  await cron.init()

  offNavigate = Events.On("navigate", async (event) => {
    const target = String(event?.data || "")
    if (target === "settings") {
      await router.push({ name: "settings" })
      return
    }
    await router.push({ name: "main" })
  })

  offGlobalEnabledChanged = Events.On("globalEnabledChanged", (event) => {
    globalEnabled.value = !!event?.data
  })
})

onUnmounted(() => {
  if (offNavigate) {
    offNavigate()
    offNavigate = null
  }
  if (offGlobalEnabledChanged) {
    offGlobalEnabledChanged()
    offGlobalEnabledChanged = null
  }
  cron.dispose()
})
</script>

<template>
  <div class="relative min-h-screen bg-slate-50 text-slate-900 font-sans">
    <div
      v-if="toast"
      class="fixed top-3 right-3 z-[9999] max-w-[380px] rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm shadow-[0_10px_30px_rgba(2,6,23,0.08)] data-[kind=success]:border-green-600/25 data-[kind=success]:bg-green-50 data-[kind=danger]:border-red-600/25 data-[kind=danger]:bg-red-50 sm:top-4 sm:right-4"
      :data-kind="toastKind"
    >
      {{ toast }}
    </div>

    <header class="sticky top-0 z-[9998] border-b border-slate-200 bg-slate-50">
      <div class="mx-auto flex max-w-[1240px] items-center justify-between gap-3 px-3 py-2 sm:px-5 sm:py-3">
        <div class="flex items-center gap-2.5">
          <div class="flex items-center gap-2">
            <span class="text-xs text-slate-600">Global</span>
            <button
              type="button"
              class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-4 focus:ring-blue-600/20"
              :class="globalEnabled ? 'bg-green-600' : 'bg-slate-300'"
              :title="globalEnabled ? 'Global enabled' : 'Global disabled'"
              @click="toggleGlobalEnabled"
            >
              <span
                class="inline-block h-5 w-5 transform rounded-full bg-white transition"
                :class="globalEnabled ? 'translate-x-5' : 'translate-x-1'"
              />
            </button>
          </div>

          <button
            v-if="!isSettings"
            class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            @click="goSettings"
          >
            Settings
          </button>
          <button
            v-else
            class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            @click="goBack"
          >
            Back
          </button>
        </div>
        <div class="flex items-center gap-2.5">
          <div class="h-8 min-w-[120px]"></div>
        </div>
      </div>
    </header>

    <router-view />
  </div>
</template>
