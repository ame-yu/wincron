<script setup>
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useCronStore } from "../stores/cron.js"
import ModalShell from "./ModalShell.vue"

defineProps({
  btn: { type: String, required: true },
  btnPrimary: { type: String, required: true },
})

const cron = useCronStore()
const { t } = useI18n()

const show = ref(false)

const exportJobs = ref(true)
const exportJobScope = ref("all")
const exportSettings = ref(true)

const exportOnlyEnabled = computed(() => exportJobs.value && exportJobScope.value === "onlyEnabled")
const exportNothingSelected = computed(() => !exportJobs.value && !exportSettings.value)

async function confirmExport() {
  if (exportNothingSelected.value) {
    return
  }
  try {
    await cron.exportConfig({
      exportJobs: exportJobs.value,
      exportSettings: exportSettings.value,
      onlyEnabled: exportOnlyEnabled.value,
    })
    show.value = false
  } catch (e) {
    cron.error = String(e)
  }
}
</script>

<template>
  <button :class="btn" @click="show = true">{{ $t("settings.export_yaml") }}</button>

  <ModalShell v-model="show" :max-width="460">
      <div>
        <h3>{{ $t("settings.export_options_title") }}</h3>
        <div class="mt-0.5 text-xs text-slate-500">{{ $t("settings.export_options_subtitle") }}</div>
      </div>

      <div class="mt-3 space-y-2">
        <label class="flex items-start gap-2">
          <input type="checkbox" v-model="exportJobs" class="mt-0.5" />
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-slate-900">{{ $t("settings.export_jobs") }}</span>
              <div class="flex items-center gap-1 rounded-lg border border-slate-200 bg-white p-0.5" :class="!exportJobs ? 'opacity-50 pointer-events-none' : ''">
                <button
                  type="button"
                  class="rounded-md px-2 py-1 text-xs leading-none transition"
                  :class="exportJobScope === 'all' ? 'bg-slate-900 text-white' : 'text-slate-700 hover:bg-slate-50'"
                  :disabled="!exportJobs"
                  @click.stop.prevent="exportJobScope = 'all'"
                >
                  {{ $t("settings.export_jobs_all") }}
                </button>
                <button
                  type="button"
                  class="rounded-md px-2 py-1 text-xs leading-none transition"
                  :class="exportJobScope === 'onlyEnabled' ? 'bg-slate-900 text-white' : 'text-slate-700 hover:bg-slate-50'"
                  :disabled="!exportJobs"
                  @click.stop.prevent="exportJobScope = 'onlyEnabled'"
                >
                  {{ $t("settings.export_jobs_only_enabled") }}
                </button>
              </div>
            </div>
          </div>
        </label>

        <label class="flex items-center gap-2">
          <input type="checkbox" v-model="exportSettings" />
          <span>{{ $t("settings.export_settings") }}</span>
        </label>
      </div>

      <div class="mt-4 flex justify-end gap-2">
        <button :class="btn" @click="show = false">{{ t("common.cancel") }}</button>
        <button
          :class="[btnPrimary, exportNothingSelected ? 'opacity-50 pointer-events-none' : '']"
          :disabled="exportNothingSelected"
          @click="confirmExport"
        >
          {{ t("common.export") }}
        </button>
      </div>
  </ModalShell>
</template>
