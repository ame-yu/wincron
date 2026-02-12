<script setup>
const show = defineModel({ type: Boolean, default: false })

defineProps({
  maxWidth: { type: Number, default: 560 },
})

const emit = defineEmits(["close"])

function close() {
  show.value = false
  emit("close")
}
</script>

<template>
  <teleport to="body">
    <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/20 p-4" @click.self="close">
      <div
        class="w-full rounded-2xl border border-slate-200 bg-white p-4 shadow-[0_10px_30px_rgba(2,6,23,0.16)]"
        :style="{ maxWidth: maxWidth + 'px' }"
      >
        <slot />
      </div>
    </div>
  </teleport>
</template>
