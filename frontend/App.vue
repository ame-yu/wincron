<script setup>
import { computed, onMounted, onUnmounted, watch } from "vue"
import { storeToRefs } from "pinia"
import { useRoute, useRouter } from "vue-router"
import { useI18n } from "vue-i18n"
import { Events, Window } from "@wailsio/runtime"
import { useCronStore } from "./stores/cron.js"
import { setAppLocale } from "./i18n.js"

const cron = useCronStore()
const { toast, toastKind, toastActionLabel, globalEnabled } = storeToRefs(cron)

const { t, locale } = useI18n()

const appLocale = computed({
  get: () => locale.value,
  set: (value) => {
    setAppLocale(value)
  },
})

const router = useRouter()
const route = useRoute()
const isSettings = computed(() => route.name === "Settings")
const nav = computed(() => {
  const _ = locale.value
  return isSettings.value
    ? { label: t("nav.back"), name: "Home" }
    : { label: t("nav.settings"), name: "Settings" }
})

watch(
  [() => route.name, () => locale.value, () => globalEnabled.value],
  ([name, _locale, enabled]) => {
    const base = enabled ? "WinCron" : t("global.disabled_title")
    const routeName = typeof name === "string" ? name : ""
    const suffix =
      routeName === "Settings"
        ? t("route.settings")
        : routeName === "Home"
          ? t("route.home")
          : routeName
    const title = suffix ? `${base} - ${suffix}` : base
    document.title = title
    Window.SetTitle(title).catch(() => {})
  },
  { immediate: true },
)

function goNav() {
  router.push({ name: nav.value.name }).catch(() => {})
}

async function toggleGlobalEnabled(value) {
  const next = typeof value === "boolean" ? value : !globalEnabled.value
  try {
    await cron.setGlobalEnabled(next)
  } catch {
    // noop
  }
}

const offHandlers = []

onMounted(async () => {
  await cron.init()

  const flushDraft = () => {
    cron.flushDraft()
  }

  const promptDraft = () => {
    cron.promptDraftRecovery()
  }

  offHandlers.push(
    Events.On("navigate", async (event) => {
      flushDraft()
      const target = String(event?.data || "")
      await router.push({ name: target === "Settings" ? "Settings" : "Home" }).catch(() => {})
    }),
  )

  ;["common:WindowClosing", "common:WindowHide", "common:WindowMinimise"].forEach((name) => {
    offHandlers.push(Events.On(name, flushDraft))
  })

  ;["common:WindowShow", "common:WindowRestore", "common:WindowUnMinimise"].forEach((name) => {
    offHandlers.push(Events.On(name, promptDraft))
  })

  offHandlers.push(
    Events.On("globalEnabledChanged", (event) => {
      globalEnabled.value = !!event?.data
    }),
  )
})

onUnmounted(() => {
  offHandlers.splice(0).forEach((off) => off?.())
  cron.dispose()
})
</script>

<template>
  <div class="relative h-screen overflow-hidden bg-slate-50 text-slate-900 font-sans flex flex-col" >
    <Transition name="toast" appear>
      <div
        v-if="toast"
        class="fixed right-3 bottom-3 z-[9999] max-w-[380px] rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm shadow-[0_10px_30px_rgba(2,6,23,0.08)] data-[kind=success]:border-green-600/25 data-[kind=success]:bg-green-50 data-[kind=danger]:border-red-600/25 data-[kind=danger]:bg-red-50 sm:right-4 sm:bottom-4"
        :data-kind="toastKind"
      >
        <div class="flex items-center gap-3">
          <div class="min-w-0 flex-1">{{ toast }}</div>
          <button
            v-if="toastActionLabel"
            type="button"
            class="shrink-0 rounded-lg bg-slate-900 px-2.5 py-1.5 text-xs font-medium text-white transition hover:bg-slate-800 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20"
            @click.stop="cron.triggerToastAction"
          >
            {{ toastActionLabel }}
          </button>
        </div>
      </div>
    </Transition>

    <header class="sticky top-0 z-[9998] border-b border-slate-200 bg-slate-50">
      <div class="mx-auto flex max-w-[1240px] items-center justify-between gap-3 px-3 py-2 sm:px-5 sm:py-3">
        <div class="flex items-center gap-2.5">
          <div class="flex items-center gap-2">
            <span class="text-xs text-slate-600">{{ $t("global.label") }}</span>
            <div
              class="relative inline-flex h-8 w-[240px] items-stretch rounded-xl border border-slate-200 bg-white p-0.5 shadow-sm"
              :title="globalEnabled ? $t('global.enabled') : $t('global.disabled')"
            >
              <div
                class="pointer-events-none absolute inset-y-0 left-0 w-1/2 rounded-lg transition-transform"
                :class="[
                  globalEnabled ? 'translate-x-0 bg-green-600' : 'translate-x-full bg-slate-600',
                ]"
              ></div>

              <button
                type="button"
                class="relative z-10 flex flex-1 items-center justify-center rounded-lg px-2 text-[11px] font-medium leading-tight whitespace-nowrap transition-colors focus:outline-none focus:ring-4 focus:ring-blue-600/20"
                :class="globalEnabled ? 'text-white' : 'text-slate-500'"
                @click="toggleGlobalEnabled(true)"
              >
                {{ $t("global.enable_wincorn") }}
              </button>
              <button
                type="button"
                class="relative z-10 flex flex-1 items-center justify-center rounded-lg px-2 text-[11px] font-medium leading-tight whitespace-nowrap transition-colors focus:outline-none focus:ring-4 focus:ring-blue-600/20"
                :class="globalEnabled ? 'text-slate-500' : 'text-white'"
                @click="toggleGlobalEnabled(false)"
              >
                {{ $t("global.disable_wincorn") }}
              </button>
            </div>
          </div>
        </div>
        <div class="flex items-center gap-2.5">
          <select
            v-model="appLocale"
            class="h-8 w-auto appearance-none rounded-xl border border-slate-200 bg-white px-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            :title="$t('app.language')"
          >
            <option value="en">🇺🇸 EN</option>
            <option value="zh">🇨🇳 中文</option>
            <option value="ja">🇯🇵 日本語</option>
          </select>

          <button
            class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            :title="nav.label"
            @click="goNav"
          >
            {{ nav.label }}
          </button>
        </div>
      </div>
    </header>

    <div class="flex-1 min-h-0">
      <router-view />
    </div>
  </div>
</template>
