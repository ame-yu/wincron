<script setup>
import { onBeforeUnmount, onMounted, ref } from "vue"
import { storeToRefs } from "pinia"
import { useCronStore } from "../stores/cron.js"
import { useI18n } from "vue-i18n"

const cron = useCronStore()
const { error, closeBehavior, silentStart, lightweightMode, autoStart } = storeToRefs(cron)

const { t } = useI18n()

const importInput = ref(null)

const btn =
  "appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
const btnPrimary =
  "appearance-none rounded-xl border border-blue-600/35 bg-blue-600 px-2.5 py-2 text-xs leading-none text-white transition hover:bg-blue-700 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
const btnDanger =
  "appearance-none rounded-xl border border-red-600/25 bg-red-50 px-2.5 py-2 text-xs leading-none text-red-600 transition hover:bg-red-100 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"

const setError = (e) => (cron.error = String(e))
const safeSetting = async (fn) => {
  try {
    await fn()
  } catch {
    await cron.loadSettings()
  }
}

const showExportDialog = ref(false)
const exportSettings = ref(true)
const exportOnlyEnabled = ref(false)

const showImportDialog = ref(false)
const importText = ref("")
const importFileName = ref("")
const importConflicts = ref([])
const importStrategy = ref("coexist")

const showShortcutDialog = ref(false)

function cancelImport() {
  showImportDialog.value = false
  importText.value = importFileName.value = ""
  importConflicts.value = []
  importStrategy.value = "coexist"
}

onMounted(async () => {
  await cron.loadSettings()
  window.addEventListener("keydown", onGlobalKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener("keydown", onGlobalKeydown)
})

const onGlobalKeydown = (e) => {
  if (e?.repeat) {
    return
  }
  const key = typeof e?.key === "string" ? e.key.toLowerCase() : ""
  if (key !== "escape") {
    return
  }

  if (showImportDialog.value) {
    cancelImport()
    return
  }
  if (showExportDialog.value) {
    showExportDialog.value = false
    return
  }
  if (showShortcutDialog.value) {
    showShortcutDialog.value = false
  }
}

async function resetAllData() {
  if (!window.confirm(t("settings.reset_confirm"))) return
  try {
    await cron.resetAll()
  } catch (e) {
    setError(e)
  }
}

async function confirmExport() {
  showExportDialog.value = false
  try {
    await cron.exportConfig({
      exportSettings: exportSettings.value,
      onlyEnabled: exportOnlyEnabled.value,
    })
  } catch (e) {
    setError(e)
  }
}

async function openDataDir() {
  try {
    await cron.openDataDir()
  } catch (e) {
    setError(e)
  }
}

function openShortcutGuide() {
  showShortcutDialog.value = true
}

const onCloseBehaviorChange = (ev) => safeSetting(() => cron.setCloseBehavior(ev?.target?.value))
const onSilentStartChange = (ev) => safeSetting(() => cron.setSilentStart(!!ev?.target?.checked))
const onLightweightModeChange = (ev) => safeSetting(() => cron.setLightweightMode(!!ev?.target?.checked))
const onAutoStartChange = (ev) => safeSetting(() => cron.setAutoStart(!!ev?.target?.checked))

const triggerImport = () => {
  importInput.value?.click()
}

async function onImportFile(ev) {
  const input = ev?.target
  const file = input?.files?.[0]
  if (input) {
    input.value = ""
  }
  if (!file) {
    return
  }

  try {
    const text = await file.text()
    const conflicts = await cron.checkImportConflicts(text)
    const list = Array.isArray(conflicts) ? conflicts : []
    if (list.length > 0) {
      importText.value = text
      importFileName.value = file?.name ?? ""
      importConflicts.value = list
      importStrategy.value = "coexist"
      showImportDialog.value = true
      return
    }

    await cron.importConfig(text, "coexist")
  } catch (e) {
    setError(e)
  }
}

async function confirmImport() {
  const text = importText.value
  const strategy = importStrategy.value
  cancelImport()

  try {
    await cron.importConfig(text, strategy)
  } catch (e) {
    setError(e)
  }
}
</script>

<template>
  <div class="mx-auto max-w-[920px] p-5">
    <input ref="importInput" type="file" accept=".yml,.yaml,text/yaml,application/x-yaml,application/yaml" class="hidden" @change="onImportFile" />

    <div v-if="showExportDialog" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/20 p-4" @click.self="showExportDialog = false">
      <div class="w-full max-w-[460px] rounded-2xl border border-slate-200 bg-white p-4 shadow-[0_10px_30px_rgba(2,6,23,0.16)]">
        <div>
          <h3>{{ $t("settings.export_options_title") }}</h3>
          <div class="mt-0.5 text-xs text-slate-500">{{ $t("settings.export_options_subtitle") }}</div>
        </div>

        <div class="mt-3 space-y-2">
          <label class="flex items-center gap-2">
            <input type="checkbox" v-model="exportSettings" />
            <span>{{ $t("settings.export_settings") }}</span>
          </label>
          <label class="flex items-center gap-2">
            <input type="checkbox" v-model="exportOnlyEnabled" />
            <span>{{ $t("settings.export_only_enabled") }}</span>
          </label>
        </div>

        <div class="mt-4 flex justify-end gap-2">
          <button :class="btn" @click="showExportDialog = false">{{ $t("common.cancel") }}</button>
          <button :class="btnPrimary" @click="confirmExport">{{ $t("common.export") }}</button>
        </div>
      </div>
    </div>

    <div v-if="showShortcutDialog" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/20 p-4" @click.self="showShortcutDialog = false">
      <div class="w-full max-w-[560px] rounded-2xl border border-slate-200 bg-white p-4 shadow-[0_10px_30px_rgba(2,6,23,0.16)]">
        <div>
          <h3>{{ $t("settings.shortcuts_title") }}</h3>
          <div class="mt-0.5 text-xs text-slate-500">{{ $t("settings.shortcuts_subtitle") }}</div>
        </div>

        <div class="mt-3 rounded-xl border border-slate-200 bg-slate-50 p-3">
          <div class="grid grid-cols-[140px_1fr] gap-x-3 gap-y-2 text-sm">
            <div class="font-mono text-xs text-slate-700">Ctrl/⌘ + S</div>
            <div class="text-slate-900">{{ $t("settings.shortcuts.save") }}</div>

            <div class="font-mono text-xs text-slate-700">Esc</div>
            <div class="text-slate-900">{{ $t("settings.shortcuts.close_dialog") }}</div>
          </div>
        </div>

        <div class="mt-4 flex justify-end gap-2">
          <button :class="btnPrimary" @click="showShortcutDialog = false">{{ $t("common.ok") }}</button>
        </div>
      </div>
    </div>

    <div v-if="showImportDialog" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/20 p-4" @click.self="cancelImport">
      <div class="w-full max-w-[560px] rounded-2xl border border-slate-200 bg-white p-4 shadow-[0_10px_30px_rgba(2,6,23,0.16)]">
        <div>
          <h3>{{ $t("settings.import_title") }}</h3>
          <div class="mt-0.5 text-xs text-slate-500">{{ $t("settings.import_subtitle") }}</div>
          <div v-if="importFileName" class="mt-0.5 text-xs text-slate-500">{{ $t("settings.import_file", { name: importFileName }) }}</div>
        </div>

        <div class="mt-3 space-y-2">
          <label class="flex items-start gap-2">
            <input type="radio" name="importStrategy" value="coexist" v-model="importStrategy" />
            <div>
              <div>{{ $t("settings.import_strategy.coexist") }}</div>
              <div class="text-xs text-slate-500">{{ $t("settings.import_strategy_help.coexist") }}</div>
            </div>
          </label>
          <label class="flex items-start gap-2">
            <input type="radio" name="importStrategy" value="overwrite" v-model="importStrategy" />
            <div>
              <div>{{ $t("settings.import_strategy.overwrite") }}</div>
              <div class="text-xs text-slate-500">{{ $t("settings.import_strategy_help.overwrite") }}</div>
            </div>
          </label>
        </div>

        <div v-if="importConflicts.length" class="mt-3 rounded-xl border border-slate-200 bg-slate-50 p-3">
          <div class="text-xs font-semibold text-slate-700">{{ $t("settings.conflicts", { count: importConflicts.length }) }}</div>
          <div class="mt-2 max-h-[180px] overflow-auto">
            <div v-for="name in importConflicts" :key="name" class="font-mono text-xs text-slate-700">{{ name }}</div>
          </div>
        </div>
        <div v-else class="mt-3 text-xs text-slate-500">{{ $t("settings.no_conflicts") }}</div>

        <div class="mt-4 flex justify-end gap-2">
          <button :class="btn" @click="cancelImport">{{ $t("common.cancel") }}</button>
          <button :class="btnPrimary" @click="confirmImport">{{ $t("common.import") }}</button>
        </div>
      </div>
    </div>

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
        <button :class="btn" @click="showExportDialog = true">{{ $t("settings.export_yaml") }}</button>
        <button :class="btn" @click="triggerImport">{{ $t("settings.import_yaml") }}</button>
        <button :class="btn" @click="openDataDir">{{ $t("settings.open_data_dir") }}</button>
        <button :class="btn" @click="openShortcutGuide">{{ $t("settings.shortcut_guide") }}</button>
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
              <input type="checkbox" v-model="lightweightMode" class="mt-0.5" :disabled="!silentStart" @change="onLightweightModeChange" />
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
      </div>

      <div class="px-3 pb-3.5 text-xs text-slate-500">{{ $t("settings.import_note") }}</div>
    </section>
  </div>
</template>
