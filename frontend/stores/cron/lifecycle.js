import { Events } from "@wailsio/runtime"

export function createLifecycleActions(ctx) {
  const bindJobEvent = (name, handler) => Events.On(name, ({ data } = {}) => data && handler(data))

  const disposeListener = (getDisposer, setDisposer) => {
    const off = getDisposer()
    if (off) {
      off()
      setDisposer(null)
    }
  }

  async function init() {
    if (ctx.getOffJobExecuted() || ctx.getOffJobStarted()) {
      return
    }

    await ctx.loadSettings()
    await ctx.loadGlobalEnabled()
    await ctx.refreshJobs()
    await ctx.focusLogs("")

    ctx.setOffJobStarted(bindJobEvent("jobStarted", (entry) => ctx.syncLiveLog(entry)))

    ctx.setOffJobExecuted(
      bindJobEvent("jobExecuted", async (entry) => {
        ctx.syncLiveLog(entry)
        const ok = entry.exitCode === 0
        ctx.showToast(
          `${entry.jobName}: ${ok ? ctx.t("common.ok") : `${ctx.t("common.fail")} (exit=${entry.exitCode})`}`,
          ok ? "success" : "danger",
        )

        await ctx.refreshJobs()
        if (ctx.selectedJobId.value) {
          const job = Array.isArray(ctx.jobs.value) ? ctx.jobs.value.find((j) => j?.id === ctx.selectedJobId.value) : null
          if (job && !ctx.isFormDirty()) {
            ctx.loadJobToForm(job)
          }
        }

        await ctx.reloadFocusedLogs()
      }),
    )
  }

  function dispose() {
    disposeListener(ctx.getOffJobStarted, ctx.setOffJobStarted)
    disposeListener(ctx.getOffJobExecuted, ctx.setOffJobExecuted)
    ctx.dismissToast()
    if (ctx.pendingDeleteJobs.size) {
      for (const pending of ctx.pendingDeleteJobs.values()) {
        if (pending?.timer) {
          clearTimeout(pending.timer)
        }
      }
      ctx.pendingDeleteJobs.clear()
    }
  }

  return {
    init,
    dispose,
  }
}
