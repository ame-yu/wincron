<script setup>
import { onMounted, ref } from "vue"
import { storeToRefs } from "pinia"
import { useCronStore } from "../stores/cron.js"

const cron = useCronStore()
const { error, closeBehavior, silentStart, autoStart } = storeToRefs(cron)

const importInput = ref(null)

const showExportDialog = ref(false)
const exportSettings = ref(true)
const exportOnlyEnabled = ref(false)

const showImportDialog = ref(false)
const importText = ref("")
const importFileName = ref("")
const importConflicts = ref([])
const importStrategy = ref("coexist")

function cancelImport() {
  showImportDialog.value = false
  importText.value = ""
  importFileName.value = ""
  importConflicts.value = []
  importStrategy.value = "coexist"
}

onMounted(async () => {
  await cron.loadSettings()
})

async function resetAllData() {
  const ok = window.confirm("Are you sure you want to clear all data? This action cannot be undone.")
  if (!ok) {
    return
  }

  try {
    await cron.resetAll()
  } catch (e) {
    cron.error = String(e)
  }
}

async function exportConfig() {
  showExportDialog.value = true
}

async function confirmExport() {
  showExportDialog.value = false
  try {
    await cron.exportConfig({
      exportSettings: exportSettings.value,
      onlyEnabled: exportOnlyEnabled.value,
    })
  } catch (e) {
    cron.error = String(e)
  }
}

async function openDataDir() {
  try {
    await cron.openDataDir()
  } catch (e) {
    cron.error = String(e)
  }
}

async function onCloseBehaviorChange(ev) {
  const v = ev?.target?.value
  try {
    await cron.setCloseBehavior(v)
  } catch {
    await cron.loadSettings()
  }
}

async function onSilentStartChange(ev) {
  const v = !!ev?.target?.checked
  try {
    await cron.setSilentStart(v)
  } catch {
    await cron.loadSettings()
  }
}

async function onAutoStartChange(ev) {
  const v = !!ev?.target?.checked
  try {
    await cron.setAutoStart(v)
  } catch {
    await cron.loadSettings()
  }
}

function triggerImport() {
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
    cron.error = String(e)
  }
}

async function confirmImport() {
  const text = importText.value
  cancelImport()

  try {
    await cron.importConfig(text, importStrategy.value)
  } catch (e) {
    cron.error = String(e)
  }
}
</script>

<template>
  <div class="mx-auto max-w-[920px] p-5">
    <input ref="importInput" type="file" accept=".yml,.yaml,text/yaml,application/x-yaml,application/yaml" class="hidden" @change="onImportFile" />

    <div v-if="showExportDialog" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/20 p-4" @click.self="() => (showExportDialog = false)">
      <div class="w-full max-w-[460px] rounded-2xl border border-slate-200 bg-white p-4 shadow-[0_10px_30px_rgba(2,6,23,0.16)]">
        <div>
          <h3>Export YAML Config</h3>
          <div class="mt-0.5 text-xs text-slate-500">Choose export options</div>
        </div>

        <div class="mt-3 space-y-2">
          <label class="flex items-center gap-2">
            <input type="checkbox" v-model="exportSettings" />
            <span>Export settings</span>
          </label>
          <label class="flex items-center gap-2">
            <input type="checkbox" v-model="exportOnlyEnabled" />
            <span>Only enabled jobs</span>
          </label>
        </div>

        <div class="mt-4 flex justify-end gap-2">
          <button
            class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            @click="() => (showExportDialog = false)"
          >
            Cancel
          </button>
          <button
            class="appearance-none rounded-xl border border-blue-600/35 bg-blue-600 px-2.5 py-2 text-xs leading-none text-white transition hover:bg-blue-700 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            @click="confirmExport"
          >
            Export
          </button>
        </div>
      </div>
    </div>

    <div v-if="showImportDialog" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/20 p-4" @click.self="cancelImport">
      <div class="w-full max-w-[560px] rounded-2xl border border-slate-200 bg-white p-4 shadow-[0_10px_30px_rgba(2,6,23,0.16)]">
        <div>
          <h3>Import YAML Config</h3>
          <div class="mt-0.5 text-xs text-slate-500">If job names conflict, choose a strategy</div>
          <div v-if="importFileName" class="mt-0.5 text-xs text-slate-500">File: {{ importFileName }}</div>
        </div>

        <div class="mt-3 space-y-2">
          <label class="flex items-start gap-2">
            <input type="radio" name="importStrategy" value="coexist" v-model="importStrategy" />
            <div>
              <div>Coexist</div>
              <div class="text-xs text-slate-500">Keep existing jobs. Imported jobs with the same name will be renamed (e.g. "(imported)").</div>
            </div>
          </label>
          <label class="flex items-start gap-2">
            <input type="radio" name="importStrategy" value="overwrite" v-model="importStrategy" />
            <div>
              <div>Overwrite</div>
              <div class="text-xs text-slate-500">Replace existing jobs that have the same name with the imported ones.</div>
            </div>
          </label>
        </div>

        <div v-if="importConflicts.length" class="mt-3 rounded-xl border border-slate-200 bg-slate-50 p-3">
          <div class="text-xs font-semibold text-slate-700">Conflicts ({{ importConflicts.length }})</div>
          <div class="mt-2 max-h-[180px] overflow-auto">
            <div v-for="name in importConflicts" :key="name" class="font-mono text-xs text-slate-700">{{ name }}</div>
          </div>
        </div>
        <div v-else class="mt-3 text-xs text-slate-500">No conflicts detected.</div>

        <div class="mt-4 flex justify-end gap-2">
          <button
            class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            @click="cancelImport"
          >
            Cancel
          </button>
          <button
            class="appearance-none rounded-xl border border-blue-600/35 bg-blue-600 px-2.5 py-2 text-xs leading-none text-white transition hover:bg-blue-700 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
            @click="confirmImport"
          >
            Import
          </button>
        </div>
      </div>
    </div>

    <section class="rounded-2xl border border-slate-200 bg-white p-3.5 shadow-[0_10px_30px_rgba(2,6,23,0.08)]">
      <div class="flex items-start justify-between gap-3 px-3 pt-3 pb-2">
        <div>
          <h2>Settings</h2>
          <div class="mt-0.5 text-xs text-slate-500">Import/Export configuration & reset</div>
        </div>
      </div>

      <div v-if="error" class="mx-3 mb-3 rounded-xl border border-red-600/25 bg-red-50 px-3 py-2.5 text-sm text-red-800">
        {{ error }}
      </div>

      <div class="flex flex-wrap gap-2.5 px-3 pb-3.5">
        <button
          class="appearance-none rounded-xl border border-red-600/25 bg-red-50 px-2.5 py-2 text-xs leading-none text-red-600 transition hover:bg-red-100 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
          @click="resetAllData"
        >
          Clear All Data
        </button>
        <button
          class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
          @click="exportConfig"
        >
          Export YAML Config
        </button>
        <button
          class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
          @click="triggerImport"
        >
          Import YAML Config
        </button>
        <button
          class="appearance-none rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs leading-none text-slate-900 transition hover:bg-slate-50 active:translate-y-px focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
          @click="openDataDir"
        >
          Open Data Directory
        </button>
      </div>

      <div class="px-3 pb-3.5 space-y-3">
        <div class="rounded-2xl border border-slate-200 bg-slate-50 p-3">
          <div class="text-sm font-semibold text-slate-900">Startup</div>
          <div class="mt-2 space-y-3">
            <div class="flex items-start justify-between gap-4">
              <div>
                <div class="text-sm text-slate-900">Run on boot</div>
                <div class="mt-0.5 text-xs text-slate-500">Create a shortcut in the Windows Startup folder.</div>
              </div>
              <input type="checkbox" v-model="autoStart" class="mt-0.5" @change="onAutoStartChange" />
            </div>

            <div class="flex items-start justify-between gap-4">
              <div>
                <div class="text-sm text-slate-900">Silent start</div>
                <div class="mt-0.5 text-xs text-slate-500">Start in tray without showing the main window.</div>
              </div>
              <input type="checkbox" v-model="silentStart" class="mt-0.5" @change="onSilentStartChange" />
            </div>
          </div>
        </div>

        <div class="rounded-2xl border border-slate-200 bg-slate-50 p-3">
          <div class="text-sm font-semibold text-slate-900">Window</div>
          <div class="mt-2">
            <div class="text-sm text-slate-900">Close button behavior</div>
            <div class="mt-1.5 flex flex-wrap items-center gap-2.5">
              <label class="flex items-center gap-2">
                <input type="radio" name="closeBehavior" value="exit" v-model="closeBehavior" @change="onCloseBehaviorChange" />
                <span>Exit application</span>
              </label>
              <label class="flex items-center gap-2">
                <input type="radio" name="closeBehavior" value="tray" v-model="closeBehavior" @change="onCloseBehaviorChange" />
                <span>Hide to tray</span>
              </label>
            </div>
            <div class="mt-0.5 text-xs text-slate-500">If set to “Hide to tray”, the app continues running in the background.</div>
          </div>
        </div>
      </div>

      <div class="px-3 pb-3.5 text-xs text-slate-500">Import will only prompt when job name conflicts are detected.</div>
    </section>
  </div>
</template>
