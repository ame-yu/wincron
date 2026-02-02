<script setup>
import { onMounted } from "vue"
import { storeToRefs } from "pinia"
import { useCronStore } from "../stores/cron.js"
import { useI18n } from "vue-i18n"
import { btn, btnDanger, btnPrimary } from "../ui/buttonClasses.js"

import SettingsExportDialog from "../components/SettingsExportDialog.vue"
import SettingsImportDialog from "../components/SettingsImportDialog.vue"
import SettingsShortcutDialog from "../components/SettingsShortcutDialog.vue"

const cron = useCronStore()
const { error, closeBehavior, silentStart, lightweightMode, autoStart } = storeToRefs(cron)

const { t } = useI18n()

onMounted(async () => {
  await cron.loadSettings()
})

async function resetAllData() {
  if (!window.confirm(t("settings.reset_confirm"))) return
  await cron.resetAll()
}

async function openDataDir() {
  await cron.openDataDir()
}

async function openEnvironmentVariables() {
  await cron.openEnvironmentVariables()
}

const onCloseBehaviorChange = (ev) => cron.setCloseBehavior(ev?.target?.value)
const onSilentStartChange = (ev) => cron.setSilentStart(!!ev?.target?.checked)
const onLightweightModeChange = (ev) => cron.setLightweightMode(!!ev?.target?.checked)
const onAutoStartChange = (ev) => cron.setAutoStart(!!ev?.target?.checked)
</script>

<template>
  <div class="mx-auto max-w-[920px] p-5">
    <section class="rounded-2xl border border-slate-200 bg-white p-3.5 shadow-[0_10px_30px_rgba(2,6,23,0.08)]">
      <div class="flex items-start justify-between gap-3 px-3 pt-3 pb-2">
        <div>
          <h2>{{ $t("settings.title") }}</h2>
          <div class="mt-0.5 text-xs text-slate-500">{{ $t("settings.subtitle") }}</div>
        </div>
      </div>

      <div v-if="error" class="mx-3 mb-3 rounded-xl border border-red-600/25 bg-red-50 px-3 py-2.5 text-sm text-red-800">
        {{ error }}
      </div>

      <div class="flex flex-wrap gap-2.5 px-3 pb-3.5">
        <button :class="btnDanger" @click="resetAllData">{{ $t("settings.reset_all") }}</button>
        <SettingsExportDialog :btn="btn" :btnPrimary="btnPrimary" />
        <SettingsImportDialog :btn="btn" :btnPrimary="btnPrimary" />
        <SettingsShortcutDialog :btn="btn" :btnPrimary="btnPrimary" />
      </div>

      <div class="px-3 pb-3.5 space-y-3">
        <div class="rounded-2xl border border-slate-200 bg-slate-50 p-3">
          <div class="text-sm font-semibold text-slate-900">{{ $t("settings.startup") }}</div>
          <div class="mt-2 space-y-3">
            <div class="flex items-start justify-between gap-4">
              <div>
                <div class="text-sm text-slate-900">{{ $t("settings.run_on_boot") }}</div>
                <div class="mt-0.5 text-xs text-slate-500">{{ $t("settings.run_on_boot_help") }}</div>
              </div>
              <input type="checkbox" v-model="autoStart" class="mt-0.5" @change="onAutoStartChange" />
            </div>

            <div class="flex items-start justify-between gap-4">
              <div>
                <div class="text-sm text-slate-900">{{ $t("settings.silent_start") }}</div>
                <div class="mt-0.5 text-xs text-slate-500">{{ $t("settings.silent_start_help") }}</div>
              </div>
              <input type="checkbox" v-model="silentStart" class="mt-0.5" @change="onSilentStartChange" />
            </div>

            <div class="flex items-start justify-between gap-4">
              <div>
                <div class="text-sm text-slate-900">{{ $t("settings.lightweight_mode") }}</div>
                <div class="mt-0.5 text-xs text-slate-500">{{ $t("settings.lightweight_mode_help") }}</div>
              </div>
              <input type="checkbox" v-model="lightweightMode" class="mt-0.5" @change="onLightweightModeChange" />
            </div>
          </div>
        </div>

        <div class="rounded-2xl border border-slate-200 bg-slate-50 p-3">
          <div class="text-sm font-semibold text-slate-900">{{ $t("settings.window") }}</div>
          <div class="mt-2">
            <div class="text-sm text-slate-900">{{ $t("settings.close_behavior") }}</div>
            <div class="mt-1.5 flex flex-wrap items-center gap-2.5">
              <label class="flex items-center gap-2">
                <input type="radio" name="closeBehavior" value="exit" v-model="closeBehavior" @change="onCloseBehaviorChange" />
                <span>{{ $t("settings.exit_application") }}</span>
              </label>
              <label class="flex items-center gap-2">
                <input type="radio" name="closeBehavior" value="tray" v-model="closeBehavior" @change="onCloseBehaviorChange" />
                <span>{{ $t("settings.hide_to_tray") }}</span>
              </label>
            </div>
            <div class="mt-0.5 text-xs text-slate-500">{{ $t("settings.hide_to_tray_help") }}</div>
          </div>
        </div>

        <div class="rounded-2xl border border-slate-200 bg-slate-50 p-3">
          <div class="text-sm font-semibold text-slate-900">{{ $t("settings.quick_access") }}</div>
          <div class="mt-2 flex flex-wrap gap-2.5">
            <button :class="btn" @click="openDataDir">{{ $t("settings.open_data_dir") }}</button>
            <button :class="btn" @click="openEnvironmentVariables">{{ $t("settings.open_environment_variables") }}</button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>
