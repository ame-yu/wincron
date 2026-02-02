<script setup>
import { computed, ref } from "vue"

const props = defineProps({
  text: {
    type: String,
    default: "",
  },
  maxLines: {
    type: Number,
    default: 10,
  },
  preClass: {
    type: String,
    default: "",
  },
  overlayClass: {
    type: String,
    default: "",
  },
  buttonClass: {
    type: String,
    default: "",
  },
})

const expanded = ref(false)

const normalized = computed(() => {
  const v = typeof props.text === "string" ? props.text : ""
  return v.replace(/\r\n/g, "\n")
})

const lines = computed(() => {
  const arr = normalized.value.split("\n")
  while (arr.length && arr[arr.length - 1] === "") {
    arr.pop()
  }
  return arr
})

const isTruncated = computed(() => lines.value.length > props.maxLines)

const displayText = computed(() => {
  if (!isTruncated.value || expanded.value) {
    return normalized.value
  }
  return lines.value.slice(0, props.maxLines).join("\n")
})

function toggle() {
  expanded.value = !expanded.value
}
</script>

<template>
  <div class="relative">
    <pre :class="[preClass, isTruncated ? 'pb-12' : '']">{{ displayText }}</pre>
    <div v-if="isTruncated" class="absolute inset-x-0 bottom-0 flex justify-center pb-3">
      <div v-if="!expanded" class="pointer-events-none absolute inset-x-0 bottom-0 h-24 rounded-b-xl" :class="overlayClass" />
      <button type="button" class="relative text-center text-xs" :class="buttonClass" @click="toggle">
        {{ expanded ? $t("main.logs.collapse_entry") : $t("main.logs.expand_entry") }}
      </button>
    </div>
  </div>
</template>
