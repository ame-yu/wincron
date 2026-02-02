<script setup>
import { computed, ref } from "vue"
import { useCronStore } from "../stores/cron.js"

defineProps({
  btn: { type: String, required: true },
  btnPrimary: { type: String, required: true },
})

const cron = useCronStore()

const importInput = ref(null)

const show = ref(false)
const importText = ref("")
const importFileName = ref("")
const importConflicts = ref([])
const importStrategy = ref("coexist")

const hasConflicts = computed(() => importConflicts.value.length > 0)

function cancelImport() {
  show.value = false
  importText.value = importFileName.value = ""
  importConflicts.value = []
  importStrategy.value = "coexist"
}

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
      show.value = true
      return
    }

    await cron.importConfig(text, "coexist")
  } catch (e) {
    cron.error = String(e)
  }
}

async function confirmImport() {
  const text = importText.value
  const strategy = importStrategy.value
  cancelImport()

  try {
    await cron.importConfig(text, strategy)
  } catch (e) {
    cron.error = String(e)
  }
}
</script>

<template>
  <input ref="importInput" type="file" accept=".yml,.yaml,text/yaml,application/x-yaml,application/yaml" class="hidden" @change="onImportFile" />
  <button :class="btn" @click="triggerImport">{{ $t("settings.import_yaml") }}</button>

  <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/20 p-4" @click.self="cancelImport">
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

      <div v-if="hasConflicts" class="mt-3 rounded-xl border border-slate-200 bg-slate-50 p-3">
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
</template>
