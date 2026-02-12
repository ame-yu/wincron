<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from "vue"

const props = defineProps({
  btnPrimary: { type: String, required: true },
  primaryLabel: { type: String, required: true },
  menuItems: { type: Array, required: true },
  menuWidthClass: { type: String, default: "w-40" },
})

const emit = defineEmits(["primary", "select"])

const rootEl = ref(null)
const open = ref(false)

let closeTimer = null

function clearCloseTimer() {
  if (!closeTimer) {
    return
  }
  clearTimeout(closeTimer)
  closeTimer = null
}

const leftClass = computed(() => String(props.btnPrimary).replace(/\brounded-xl\b/g, "rounded-l-xl") + " border-r-0")
const rightClass = computed(() => String(props.btnPrimary).replace(/\brounded-xl\b/g, "rounded-r-xl") + " px-2.5")

function openMenu() {
  clearCloseTimer()
  open.value = true
}

function closeMenu() {
  clearCloseTimer()
  open.value = false
}

function scheduleClose() {
  clearCloseTimer()
  closeTimer = setTimeout(() => {
    closeTimer = null
    open.value = false
  }, 500)
}

function cancelClose() {
  clearCloseTimer()
}

function toggleMenu() {
  open.value = !open.value
}

function onPrimary() {
  closeMenu()
  emit("primary")
}

function onSelect(item) {
  closeMenu()
  const key = typeof item?.key === "string" ? item.key : ""
  if (!key) return
  emit("select", key)
}

function onDocPointerDown(e) {
  if (!open.value) return
  const el = rootEl.value
  if (!el) return
  if (e?.target && el.contains(e.target)) return
  closeMenu()
}

onMounted(() => {
  document.addEventListener("pointerdown", onDocPointerDown)
})

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", onDocPointerDown)
  clearCloseTimer()
})
</script>

<template>
  <div ref="rootEl" class="relative" @mouseenter="cancelClose" @mouseleave="scheduleClose">
    <div class="flex overflow-hidden rounded-xl">
      <button :class="leftClass" type="button" @click="onPrimary">{{ primaryLabel }}</button>
      <button type="button" :class="rightClass" aria-haspopup="menu" @mouseenter="openMenu" @click.stop="toggleMenu">
        <svg viewBox="0 0 20 20" fill="currentColor" class="h-4 w-4">
          <path
            fill-rule="evenodd"
            d="M5.23 7.21a.75.75 0 0 1 1.06.02L10 11.17l3.71-3.94a.75.75 0 1 1 1.08 1.04l-4.25 4.5a.75.75 0 0 1-1.08 0l-4.25-4.5a.75.75 0 0 1 .02-1.06Z"
            clip-rule="evenodd"
          />
        </svg>
      </button>
    </div>

    <div
      v-if="open"
      class="absolute right-0 z-20 mt-2 overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_10px_30px_rgba(2,6,23,0.18)]"
      :class="menuWidthClass"
      role="menu"
      @click.stop
      @mouseenter="cancelClose"
      @mouseleave="scheduleClose"
    >
      <button
        v-for="item in menuItems"
        :key="item.key"
        class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-slate-50"
        type="button"
        role="menuitem"
        @click="onSelect(item)"
      >
        <span :class="item?.default ? 'font-semibold' : ''">{{ item.label }}</span>
      </button>
    </div>
  </div>
</template>
