<script setup>
const props = defineProps({
  sortKey: { type: String, default: "name" },
  sortAsc: { type: Boolean, default: true },
  btnClass: { type: String, default: "" },
})

const emit = defineEmits(["update:sortKey", "update:sortAsc"])

function toggleSortDir() {
  emit("update:sortAsc", !props.sortAsc)
}

function onSortKeyChange(e) {
  const v = e?.target?.value
  emit("update:sortKey", typeof v === "string" && v ? v : "name")
}
</script>

<template>
  <div class="group inline-flex min-w-0 items-stretch">
    <select
      :title="$t('main.jobs.sort.title')"
      :value="sortKey"
      :class="
        btnClass +
        ' w-auto max-w-[140px] rounded-r-none pr-4 overflow-hidden text-ellipsis whitespace-nowrap focus:relative focus:z-10 group-hover:shadow-[inset_-1px_0_0_rgba(15,23,42,0.10)]'
      "
      @change="onSortKeyChange"
    >
      <option value="name">{{ $t("main.jobs.sort.name") }}</option>
      <option value="executedCount">{{ $t("main.jobs.sort.executed_count") }}</option>
      <option value="lastExecutedAt">{{ $t("main.jobs.sort.last_executed") }}</option>
      <option value="nextRunAt">{{ $t("main.jobs.sort.next_run") }}</option>
    </select>

    <button
      :title="$t(sortAsc ? 'main.jobs.sort.asc' : 'main.jobs.sort.desc')"
      type="button"
      :class="btnClass + ' -ml-px rounded-l-none px-2 text-base font-semibold focus:relative focus:z-10'"
      @click="toggleSortDir"
    >
      {{ sortAsc ? "↑" : "↓" }}
    </button>
  </div>
</template>
