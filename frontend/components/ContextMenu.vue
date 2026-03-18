<script setup>
import { computed, nextTick, ref } from "vue"
import { useCronStore } from "../stores/cron.js"
import { getMenuPosition } from "../ui/menuPosition.js"
import { getSelectedDirectJobIds, getSelectedFolderNames, getSelectedJobIdsExpanded } from "../ui/selectionTokens.js"

const props = defineProps({
  btn: { type: String, required: true },
  btnPrimary: { type: String, required: true },
})

const emit = defineEmits(["folder-rename", "folder-delete"])

const cron = useCronStore()

// State
const visible = ref(false)
const kind = ref("job") // 'job' | 'folder' | 'bulk'
const job = ref(null)
const folder = ref("")
const x = ref(0)
const y = ref(0)

// Hotkey dialog state
const hotkeyDialogVisible = ref(false)
const hotkeyDialogJob = ref(null)
const hotkeyDialogValue = ref("")
const hotkeyCaptureRef = ref(null)

const hotkeyDialogValueTrimmed = computed(() => String(hotkeyDialogValue.value || "").trim())
const hotkeyDialogTokens = computed(() =>
  hotkeyDialogValueTrimmed.value
    ? hotkeyDialogValueTrimmed.value.split("+").map((part) => part.trim()).filter(Boolean)
    : []
)
// Selected tokens (injected from parent via setSelection)
let selectedTokens = ref([])
let getFolderJobIdsFn = () => []

function setSelection(tokens, getFolderJobIds) {
  selectedTokens = tokens
  getFolderJobIdsFn = getFolderJobIds
}

const isBulk = computed(() => selectedTokens.value?.length > 1)

// Open menu at position
function openMenu(e, options = {}) {
  kind.value = options.kind || "job"
  job.value = options.job || null
  folder.value = options.folder || ""

  const menuHeight = isBulk.value ? 160 : kind.value === "folder" ? 200 : 240
  const p = getMenuPosition(e, { menuWidth: 220, menuHeight, padding: 8 })
  x.value = p.x
  y.value = p.y
  visible.value = true
}

function close() {
  visible.value = false
  job.value = null
  kind.value = "job"
  folder.value = ""
}

// Job actions
const onEdit = () => {
  if (!job.value) return
  cron.editJob(job.value)
  close()
}

const onToggle = () => {
  if (!job.value) return
  cron.toggleJob(job.value)
  close()
}

const onCopy = () => {
  if (!job.value) return
  cron.copyJob(job.value)
  close()
}

const onRun = () => {
  if (!job.value) return
  cron.runNow(job.value.id)
  close()
}

const onDelete = () => {
  if (!job.value) return
  cron.deleteJob(job.value.id)
  close()
}

// Hotkey binding
function focusHotkeyCapture() {
  hotkeyCaptureRef.value?.focus?.()
  requestAnimationFrame(() => hotkeyCaptureRef.value?.focus?.())
}

async function onBindHotkey() {
  if (!job.value) return
  const j = job.value
  const existingHotkey = String(j?.hotkey || "")
  const shouldAutoRecord = !existingHotkey.trim()
  hotkeyDialogJob.value = j
  hotkeyDialogValue.value = existingHotkey
  close()
  await cron.pauseHotkeys()
  hotkeyDialogVisible.value = true
  await nextTick()
  if (shouldAutoRecord) {
    focusHotkeyCapture()
  }
}

function closeHotkeyDialog() {
  hotkeyDialogVisible.value = false
  hotkeyDialogJob.value = null
  hotkeyDialogValue.value = ""
  cron.resumeHotkeys()
}

function clearHotkeyDialogValue() {
  hotkeyDialogValue.value = ""
  nextTick(() => {
    focusHotkeyCapture()
  })
}

function onHotkeyCaptureClick() {
  if (hotkeyDialogValueTrimmed.value) {
    hotkeyDialogValue.value = ""
  }
  nextTick(() => {
    focusHotkeyCapture()
  })
}

const HOTKEY_MAP = {
  Escape: "Esc",
  Delete: "Del",
  Insert: "Ins",
  PageUp: "PgUp",
  PageDown: "PgDn",
  ArrowUp: "Up",
  ArrowDown: "Down",
  ArrowLeft: "Left",
  ArrowRight: "Right",
}
const normalizeHotkeyKey = (k) => {
  k = String(k || "")
  if (!k) return ""
  if (k === " ") return "Space"
  if (HOTKEY_MAP[k]) return HOTKEY_MAP[k]
  if (/^F\d{1,2}$/i.test(k)) return k.toUpperCase()
  if (k.length === 1) return k.toUpperCase()
  return ""
}
const normalizeHotkeyEventKey = (e) => {
  const code = String(e?.code || "")
  if (code === "Space") return "Space"
  if (/^Key[A-Z]$/.test(code)) return code.slice(3)
  if (/^Digit[0-9]$/.test(code)) return code.slice(5)
  if (/^Numpad[0-9]$/.test(code)) return code.slice(6)
  return normalizeHotkeyKey(e?.key)
}

function onHotkeyKeyDown(e) {
  if (!e) return
  e.preventDefault()
  e.stopPropagation()
  const key = String(e.key || "")
  if (["Control", "Shift", "Alt", "Meta"].includes(key)) return
  const mods = [e.ctrlKey && "Ctrl", e.altKey && "Alt", e.shiftKey && "Shift", e.metaKey && "Win"].filter(Boolean)
  if (!mods.length) return
  const k = normalizeHotkeyEventKey(e)
  if (!k) return
  hotkeyDialogValue.value = [...mods, k].join("+")
}

async function saveHotkey() {
  const j = hotkeyDialogJob.value
  if (!j) return
  const raw = String(hotkeyDialogValue.value || "")
  const normalized = raw ? await cron.validateJobHotkey(raw) : ""
  await cron.setJobHotkey(j.id, normalized)
  closeHotkeyDialog()
}

// Folder actions
const onRenameFolder = () => {
  const name = folder.value
  close()
  emit("folder-rename", name)
}

const onEnableFolder = async () => {
  const ids = getFolderJobIdsFn(folder.value)
  close()
  await cron.setJobsEnabled(ids, true)
}

const onDisableFolder = async () => {
  const ids = getFolderJobIdsFn(folder.value)
  close()
  await cron.setJobsEnabled(ids, false)
}

const onDeleteFolder = () => {
  const name = folder.value
  close()
  emit("folder-delete", name)
}

// Bulk actions
const onEnableSelected = async () => {
  close()
  await cron.setJobsEnabled(getSelectedJobIdsExpanded(selectedTokens.value, getFolderJobIdsFn), true)
}

const onDisableSelected = async () => {
  close()
  await cron.setJobsEnabled(getSelectedJobIdsExpanded(selectedTokens.value, getFolderJobIdsFn), false)
}

const onDeleteSelected = async () => {
  const jobIds = getSelectedDirectJobIds(selectedTokens.value)
  const folders = getSelectedFolderNames(selectedTokens.value)
  close()
  selectedTokens.value = []
  jobIds.forEach((id) => cron.deleteJob(id))
  for (const n of folders) {
    await emit("folder-delete", n)
  }
}

defineExpose({
  openMenu,
  close,
  setSelection,
})
</script>

<template>
  <teleport to="body">
    <!-- Backdrop -->
    <div v-if="visible" class="fixed inset-0 z-40" @click="close" @contextmenu.prevent="close" />

    <!-- Menu -->
    <div
      v-if="visible"
      class="fixed z-50 w-[220px] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.18)]"
      :style="{ left: x + 'px', top: y + 'px' }"
    >
      <!-- Bulk context -->
      <template v-if="isBulk">
        <button
          v-for="a in [
            { click: onEnableSelected, label: 'main.folders.enable_all', color: 'text-green-800 hover:bg-green-50' },
            { click: onDisableSelected, label: 'main.folders.disable_all', color: 'text-slate-600 hover:bg-slate-50' },
          ]"
          :key="a.label"
          class="flex w-full items-center justify-between px-3 py-2 text-left text-xs"
          :class="a.color"
          @click="a.click"
        >
          <span>{{ $t(a.label) }}</span>
        </button>
        <div class="h-px bg-slate-200/70" />
        <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-rose-700 hover:bg-rose-50" @click="onDeleteSelected">
          <span>{{ $t("common.delete") }}</span>
        </button>
      </template>

      <!-- Folder context -->
      <template v-else-if="kind === 'folder'">
        <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onRenameFolder">
          <span>{{ $t("main.folders.rename") }}</span>
        </button>
        <button
          v-for="a in [
            { click: onEnableFolder, label: 'main.folders.enable_all', color: 'text-green-800 hover:bg-green-50' },
            { click: onDisableFolder, label: 'main.folders.disable_all', color: 'text-slate-600 hover:bg-slate-50' },
          ]"
          :key="a.label"
          class="flex w-full items-center justify-between px-3 py-2 text-left text-xs"
          :class="a.color"
          @click="a.click"
        >
          <span>{{ $t(a.label) }}</span>
        </button>
        <div class="h-px bg-slate-200/70" />
        <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-rose-700 hover:bg-rose-50" @click="onDeleteFolder">
          <span>{{ $t("main.folders.delete_folder") }}</span>
        </button>
      </template>

      <!-- Job context -->
      <template v-else>
        <button
          v-for="a in [
            { click: onEdit, label: 'common.edit' },
            { click: onToggle, label: job?.enabled ? 'common.disable' : 'common.enable' },
            { click: onCopy, label: 'common.copy' },
            { click: onRun, label: 'common.run_now' },
          ]"
          :key="a.label"
          class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50"
          @click="a.click"
        >
          <span>{{ $t(a.label) }}</span>
        </button>
        <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50" @click="onBindHotkey">
          <span>{{ $t("main.context.bind_hotkey") }}</span>
          <span v-if="job?.hotkey" class="text-slate-500">{{ job.hotkey }}</span>
        </button>
        <div class="h-px bg-slate-200/70" />
        <button class="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-rose-700 hover:bg-rose-50" @click="onDelete">
          <span>{{ $t("common.delete") }}</span>
        </button>
      </template>
    </div>

    <!-- Hotkey dialog -->
    <div v-if="hotkeyDialogVisible" class="fixed inset-0 z-50 flex items-center justify-center bg-black/20" @click.self="closeHotkeyDialog">
      <div class="w-[520px] max-w-[90vw] rounded-2xl border border-slate-200 bg-white p-5 shadow-[0_10px_30px_rgba(2,6,23,0.18)]">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <h3 class="text-base font-semibold text-slate-900">{{ $t("main.context.bind_hotkey") }}</h3>
            <div class="mt-1 truncate text-xs text-slate-500">{{ hotkeyDialogJob?.name || hotkeyDialogJob?.command || hotkeyDialogJob?.id }}</div>
          </div>
          <button
            type="button"
            class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-400 transition hover:border-slate-300 hover:text-slate-700 focus:outline-none focus:ring-4 focus:ring-blue-600/15"
            @click="closeHotkeyDialog"
          >
            &times;
          </button>
        </div>

        <div class="mt-3 hotkey-input-wrap">
          <div
            ref="hotkeyCaptureRef"
            class="hotkey-record-surface mt-2 flex min-h-24 w-full cursor-pointer select-none items-center justify-center px-1 py-3 text-center outline-none transition"
            tabindex="0"
            @click="onHotkeyCaptureClick"
            @keydown="onHotkeyKeyDown"
          >
            <div v-if="hotkeyDialogTokens.length" class="hotkey-token-list">
              <template v-for="(token, index) in hotkeyDialogTokens" :key="`${token}-${index}`">
                <div class="hotkey-token">{{ token }}</div>
                <span v-if="index < hotkeyDialogTokens.length - 1" class="hotkey-token-separator">+</span>
              </template>
            </div>
            <span v-else class="hotkey-record-placeholder">{{ $t("main.placeholders.hotkey") }}</span>
          </div>
        </div>

        <div class="mt-4 flex justify-end gap-2">
          <button :class="btn" type="button" @click="clearHotkeyDialogValue">{{ $t("common.clear") }}</button>
          <button :class="btnPrimary" type="button" @click="saveHotkey">{{ $t("common.save") }}</button>
        </div>
      </div>
    </div>
  </teleport>
</template>

<style scoped>
.hotkey-token-list {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  max-width: 100%;
}

.hotkey-token {
  min-width: 3.25rem;
  padding: 0.625rem 0.875rem;
  border: 1px solid rgb(203 213 225);
  border-radius: 0.9rem;
  background: white;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 1rem;
  font-weight: 700;
  line-height: 1;
  color: rgb(15 23 42);
}

.hotkey-token-separator {
  font-size: 0.95rem;
  font-weight: 700;
  color: rgb(100 116 139);
}

.hotkey-record-placeholder {
  font-size: 0.875rem;
  font-weight: 500;
  color: rgb(148 163 184);
}
</style>
