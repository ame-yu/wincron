<script setup>
import { computed, onBeforeUnmount, onMounted, ref, useAttrs } from "vue"

defineOptions({ inheritAttrs: false })

const props = defineProps({
  always: { type: Boolean, default: false },
  minThumbSize: { type: Number, default: 20 },
  rootClass: { type: String, default: "" },
  wrapClass: { type: String, default: "" },
  viewClass: { type: String, default: "" },
})

const attrs = useAttrs()

const rootRef = ref(null)
const wrapRef = ref(null)
const viewRef = ref(null)
const vBarRef = ref(null)
const hBarRef = ref(null)

const vThumbSize = ref(0)
const vThumbMove = ref(0)
const hThumbSize = ref(0)
const hThumbMove = ref(0)

const scrolling = ref(false)
const hovering = ref(false)
const dragging = ref(false)

let scrollHideTimer = null
let rafId = 0
let prevBodyUserSelect = ""

const visible = computed(() => props.always || hovering.value || scrolling.value || dragging.value)
const hasVertical = computed(() => vThumbSize.value > 0)
const hasHorizontal = computed(() => hThumbSize.value > 0)

function clamp(n, min, max) {
  return Math.min(max, Math.max(min, n))
}

function scheduleUpdate() {
  if (rafId) return
  rafId = requestAnimationFrame(() => {
    rafId = 0
    update()
  })
}

function update() {
  const wrap = wrapRef.value
  if (!wrap) return

  const clientHeight = wrap.clientHeight
  const scrollHeight = wrap.scrollHeight
  const clientWidth = wrap.clientWidth
  const scrollWidth = wrap.scrollWidth

  const vNeeded = scrollHeight > clientHeight + 1
  const hNeeded = scrollWidth > clientWidth + 1

  if (vNeeded) {
    const barSize = Math.max(0, clientHeight - 4)
    const ratio = clientHeight / scrollHeight
    const size = clamp(Math.floor(barSize * ratio), props.minThumbSize, barSize)
    const maxMove = Math.max(0, barSize - size)
    const maxScroll = Math.max(1, scrollHeight - clientHeight)
    vThumbSize.value = size
    vThumbMove.value = clamp(Math.floor((wrap.scrollTop / maxScroll) * maxMove), 0, maxMove)
  } else {
    vThumbSize.value = 0
    vThumbMove.value = 0
  }

  if (hNeeded) {
    const barSize = Math.max(0, clientWidth - 4)
    const ratio = clientWidth / scrollWidth
    const size = clamp(Math.floor(barSize * ratio), props.minThumbSize, barSize)
    const maxMove = Math.max(0, barSize - size)
    const maxScroll = Math.max(1, scrollWidth - clientWidth)
    hThumbSize.value = size
    hThumbMove.value = clamp(Math.floor((wrap.scrollLeft / maxScroll) * maxMove), 0, maxMove)
  } else {
    hThumbSize.value = 0
    hThumbMove.value = 0
  }
}

function showOnScroll() {
  scrolling.value = true
  if (scrollHideTimer) {
    clearTimeout(scrollHideTimer)
  }
  scrollHideTimer = setTimeout(() => {
    scrolling.value = false
    scrollHideTimer = null
  }, 800)
}

function onWrapScroll() {
  showOnScroll()
  scheduleUpdate()
}

function onMouseEnter() {
  hovering.value = true
}

function onMouseLeave() {
  hovering.value = false
}

const dragState = {
  axis: "",
  startClient: 0,
  startScroll: 0,
  barSize: 0,
  thumbSize: 0,
  scrollSize: 0,
  clientSize: 0,
}

function readAxisMetrics(axis) {
  const wrap = wrapRef.value
  if (!wrap) return null

  if (axis === "vertical") {
    return {
      barSize: Math.max(0, wrap.clientHeight - 4),
      thumbSize: vThumbSize.value,
      scrollSize: wrap.scrollHeight,
      clientSize: wrap.clientHeight,
      scroll: () => wrap.scrollTop,
      setScroll: (v) => (wrap.scrollTop = v),
      client: (e) => e.clientY,
    }
  }

  return {
    barSize: Math.max(0, wrap.clientWidth - 4),
    thumbSize: hThumbSize.value,
    scrollSize: wrap.scrollWidth,
    clientSize: wrap.clientWidth,
    scroll: () => wrap.scrollLeft,
    setScroll: (v) => (wrap.scrollLeft = v),
    client: (e) => e.clientX,
  }
}

function onThumbMousedown(axis, e) {
  const metrics = readAxisMetrics(axis)
  if (!metrics) return

  dragging.value = true
  try {
    prevBodyUserSelect = document?.body?.style?.userSelect ?? ""
    if (document?.body?.style) {
      document.body.style.userSelect = "none"
    }
  } catch {}
  dragState.axis = axis
  dragState.startClient = metrics.client(e)
  dragState.startScroll = metrics.scroll()
  dragState.barSize = metrics.barSize
  dragState.thumbSize = metrics.thumbSize
  dragState.scrollSize = metrics.scrollSize
  dragState.clientSize = metrics.clientSize

  document.addEventListener("mousemove", onDocumentMousemove)
  document.addEventListener("mouseup", onDocumentMouseup)

  showOnScroll()
  e.preventDefault()
  e.stopPropagation()
}

function onTrackMousedown(axis, e) {
  const wrap = wrapRef.value
  if (!wrap) return

  const metrics = readAxisMetrics(axis)
  if (!metrics) return

  const barEl = axis === "vertical" ? vBarRef.value : hBarRef.value
  if (!barEl) return

  const rect = barEl.getBoundingClientRect()
  const clickPos = axis === "vertical" ? e.clientY - rect.top - 2 : e.clientX - rect.left - 2

  const maxThumbMove = Math.max(0, metrics.barSize - metrics.thumbSize)
  const maxScroll = Math.max(0, metrics.scrollSize - metrics.clientSize)
  if (maxThumbMove <= 0 || maxScroll <= 0) return

  const targetThumbMove = clamp(clickPos - metrics.thumbSize / 2, 0, maxThumbMove)
  metrics.setScroll((targetThumbMove / maxThumbMove) * maxScroll)

  showOnScroll()
  scheduleUpdate()
  e.preventDefault()
}

function onDocumentMousemove(e) {
  if (!dragging.value) return

  const metrics = readAxisMetrics(dragState.axis)
  if (!metrics) return

  const maxThumbMove = Math.max(0, dragState.barSize - dragState.thumbSize)
  const maxScroll = Math.max(0, dragState.scrollSize - dragState.clientSize)
  if (maxThumbMove <= 0 || maxScroll <= 0) return

  const delta = metrics.client(e) - dragState.startClient
  const scrollDelta = (delta / maxThumbMove) * maxScroll
  metrics.setScroll(dragState.startScroll + scrollDelta)

  showOnScroll()
  scheduleUpdate()
}

function onDocumentMouseup() {
  dragging.value = false
  dragState.axis = ""
  try {
    if (document?.body?.style) {
      document.body.style.userSelect = prevBodyUserSelect
    }
  } catch {}
  document.removeEventListener("mousemove", onDocumentMousemove)
  document.removeEventListener("mouseup", onDocumentMouseup)
}

let wrapResizeObserver = null
let viewResizeObserver = null

onMounted(() => {
  update()

  const wrap = wrapRef.value
  const view = viewRef.value

  if (typeof ResizeObserver !== "undefined") {
    wrapResizeObserver = new ResizeObserver(() => scheduleUpdate())
    viewResizeObserver = new ResizeObserver(() => scheduleUpdate())
    if (wrap) wrapResizeObserver.observe(wrap)
    if (view) viewResizeObserver.observe(view)
  }
})

onBeforeUnmount(() => {
  if (scrollHideTimer) {
    clearTimeout(scrollHideTimer)
    scrollHideTimer = null
  }
  try {
    if (document?.body?.style) {
      document.body.style.userSelect = prevBodyUserSelect
    }
  } catch {}
  if (rafId) {
    cancelAnimationFrame(rafId)
    rafId = 0
  }
  if (wrapResizeObserver) {
    wrapResizeObserver.disconnect()
    wrapResizeObserver = null
  }
  if (viewResizeObserver) {
    viewResizeObserver.disconnect()
    viewResizeObserver = null
  }
  document.removeEventListener("mousemove", onDocumentMousemove)
  document.removeEventListener("mouseup", onDocumentMouseup)
})

const vThumbStyle = computed(() => {
  if (!vThumbSize.value) return {}
  return {
    height: vThumbSize.value + "px",
    transform: `translateY(${vThumbMove.value}px)`,
  }
})

const hThumbStyle = computed(() => {
  if (!hThumbSize.value) return {}
  return {
    width: hThumbSize.value + "px",
    transform: `translateX(${hThumbMove.value}px)`,
  }
})
</script>

<template>
  <div
    ref="rootRef"
    class="app-scrollbar"
    :class="[props.rootClass, { 'is-visible': visible }]"
    @mouseenter="onMouseEnter"
    @mouseleave="onMouseLeave"
  >
    <div ref="wrapRef" class="app-scrollbar__wrap" :class="props.wrapClass" v-bind="attrs" @scroll.passive="onWrapScroll">
      <div ref="viewRef" class="app-scrollbar__view" :class="props.viewClass">
        <slot />
      </div>
    </div>

    <div v-show="hasVertical" ref="vBarRef" class="app-scrollbar__bar is-vertical" @mousedown="onTrackMousedown('vertical', $event)">
      <div class="app-scrollbar__thumb" :style="vThumbStyle" @mousedown="onThumbMousedown('vertical', $event)"></div>
    </div>

    <div v-show="hasHorizontal" ref="hBarRef" class="app-scrollbar__bar is-horizontal" @mousedown="onTrackMousedown('horizontal', $event)">
      <div class="app-scrollbar__thumb" :style="hThumbStyle" @mousedown="onThumbMousedown('horizontal', $event)"></div>
    </div>
  </div>
</template>

<style>
.app-scrollbar {
  position: relative;
  overflow: hidden;
  height: 100%;
  width: 100%;
}

.app-scrollbar__wrap {
  height: 100%;
  width: 100%;
  overflow: auto;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.app-scrollbar__wrap::-webkit-scrollbar {
  width: 0;
  height: 0;
}

.app-scrollbar__bar {
  position: absolute;
  right: 2px;
  bottom: 2px;
  z-index: 10;
  border-radius: 4px;
  background-color: transparent;
  opacity: 0;
  pointer-events: none;
  transition: opacity 120ms ease-out;
}

.app-scrollbar.is-visible > .app-scrollbar__bar {
  opacity: 1;
  pointer-events: auto;
  background-color: rgba(0, 0, 0, 0.06);
}

.app-scrollbar__bar.is-vertical {
  top: 2px;
  width: 6px;
}

.app-scrollbar__bar.is-horizontal {
  left: 2px;
  height: 6px;
}

.app-scrollbar__thumb {
  position: relative;
  display: block;
  width: 100%;
  height: 100%;
  cursor: pointer;
  border-radius: inherit;
  background-color: rgba(144, 147, 153, 0.3);
  transition: background-color 0.3s;
}

.app-scrollbar__thumb::before {
  content: "";
  position: absolute;
  inset: -2px;
  border-radius: inherit;
}

.app-scrollbar__thumb:hover {
  background-color: rgba(144, 147, 153, 0.5);
}

.app-scrollbar__thumb:active {
  background-color: rgba(144, 147, 153, 0.7);
}
</style>
