<script setup>
import { nextTick } from "vue"
import { useCronStore } from "../stores/cron.js"

defineProps({
  btnIcon: { type: String, required: true },
})

const cron = useCronStore()
const form = cron.form

const argRefs = []
const setArgRef = (el, index) => (argRefs[index] = el)
const focusArg = (index) => nextTick(() => argRefs[index]?.focus?.())
const ensureArgs = () => (Array.isArray(form.args) ? form.args : (form.args = [""]))

function addArg(afterIndex) {
  const args = ensureArgs()
  const insertAt = Math.min(Math.max(afterIndex + 1, 0), args.length)
  args.splice(insertAt, 0, "")
  focusArg(insertAt)
}

function removeArg(index) {
  const args = ensureArgs()
  if (args.length === 1) {
    args[0] = ""
    focusArg(0)
    return
  }
  args.splice(index, 1)
  focusArg(Math.min(Math.max(index - 1, 0), args.length - 1))
}

function onArgBackspace(e, index) {
  const args = ensureArgs()
  if (args.length <= 1 || args[index] !== "") {
    return
  }
  e.preventDefault()
  removeArg(index)
}

function onArgEnter(e, index) {
  const args = ensureArgs()
  if (args[index] === "" && index === args.length - 1) {
    e.preventDefault()
    return
  }
  e.preventDefault()
  addArg(index)
}
</script>

<template>
  <div>
    <div v-for="(a, i) in form.args" :key="i" class="mb-2 flex flex-wrap items-center gap-2">
      <input
        :ref="(el) => setArgRef(el, i)"
        v-model="form.args[i]"
        class="w-full flex-1 rounded-xl border border-slate-200 bg-white px-2.5 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-4 focus:ring-blue-600/20 focus:border-blue-600/50"
        :placeholder="$t('main.placeholders.arg')"
        @keydown.enter="onArgEnter($event, i)"
        @keydown.backspace="onArgBackspace($event, i)"
      />
      <button :class="btnIcon" type="button" @click="addArg(i)">+</button>
      <button :class="btnIcon" type="button" @click="removeArg(i)">-</button>
    </div>
  </div>
</template>
