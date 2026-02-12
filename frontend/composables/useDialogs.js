import { ref, watch } from "vue"

export function useDialogs() {
  const textDialogVisible = ref(false)
  const textDialogTitle = ref("")
  const textDialogLabel = ref("")
  const textDialogValue = ref("")
  const textDialogInput = ref(null)

  let textDialogResolve = null

  const confirmDialogVisible = ref(false)
  const confirmDialogTitle = ref("")
  const confirmDialogMessage = ref("")
  const confirmDialogDanger = ref(false)

  let confirmDialogResolve = null

  function openTextDialog(options = {}) {
    const title = String(options?.title ?? "")
    const label = String(options?.label ?? "")
    const initial = String(options?.initial ?? "")
    return new Promise((resolve) => {
      textDialogTitle.value = title
      textDialogLabel.value = label
      textDialogValue.value = initial
      textDialogVisible.value = true
      textDialogResolve = resolve
    })
  }

  function closeTextDialog(result) {
    textDialogVisible.value = false
    const resolve = textDialogResolve
    textDialogResolve = null
    resolve?.(String(result ?? ""))
  }

  function openConfirmDialog(options = {}) {
    const title = String(options?.title ?? "")
    const message = String(options?.message ?? "")
    const danger = !!options?.danger
    return new Promise((resolve) => {
      confirmDialogTitle.value = title
      confirmDialogMessage.value = message
      confirmDialogDanger.value = danger
      confirmDialogVisible.value = true
      confirmDialogResolve = resolve
    })
  }

  function closeConfirmDialog(result) {
    confirmDialogVisible.value = false
    const resolve = confirmDialogResolve
    confirmDialogResolve = null
    resolve?.(!!result)
  }

  watch(
    textDialogVisible,
    (v) => {
      if (!v) {
        return
      }
      textDialogInput.value?.focus?.()
      textDialogInput.value?.select?.()
    },
    { flush: "post" },
  )

  return {
    textDialogVisible,
    textDialogTitle,
    textDialogLabel,
    textDialogValue,
    textDialogInput,
    openTextDialog,
    closeTextDialog,
    confirmDialogVisible,
    confirmDialogTitle,
    confirmDialogMessage,
    confirmDialogDanger,
    openConfirmDialog,
    closeConfirmDialog,
  }
}
