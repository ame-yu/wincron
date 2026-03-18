import { normalizeJobIds } from "../../ui/selectionTokens.js"
import { withJobsNextRunAt } from "../../ui/cron.js"
import { refreshJobsAndSyncSelectedJob, requireUpdatedJob } from "./jobUpdate.js"

export function createJobActions(ctx) {
  const normalizeArgs = (value) => Array.isArray(value) ? value.filter((s) => s !== "") : []
  const findJobById = (id) =>
    Array.isArray(ctx.jobs.value) ? ctx.jobs.value.find((j) => String(j?.id || "") === String(id || "")) : null

  async function callSetJobFolder(id, folder) {
    const updatedRaw = await ctx.callCronT(5000, "SetJobFolder", id, folder)
    return requireUpdatedJob(ctx, updatedRaw)
  }

  const resolveRunEntry = (entryRaw, errorKey) => {
    const entry = ctx.normalizeObjectResult(entryRaw)
    if (!entry) {
      throw new Error(ctx.t(errorKey))
    }
    ctx.syncLiveLog(entry)
  }

  const getRunTimeoutMs = (value) => {
    const timeout = Number(value)
    return Number.isFinite(timeout) && timeout > 0 ? Math.max(60000, timeout * 1000 + 5000) : 0
  }

  async function refreshJobs() {
    try {
      const result = await ctx.callCronT(5000, "ListJobs")
      const listed = ctx.normalizeArrayResult(result)
      const pending = ctx.pendingDeleteJobs.size ? new Set(ctx.pendingDeleteJobs.keys()) : null
      const visibleJobs = pending ? listed.filter((j) => !pending.has(String(j?.id || ""))) : listed
      ctx.jobs.value = withJobsNextRunAt(visibleJobs)
      ctx.jobsLoaded.value = true
    } catch (e) {
      ctx.jobs.value = []
      ctx.reportError(e)
    }
  }

  function loadJobToForm(job) {
    const args = normalizeArgs(job.args)
    ctx.selectedJobId.value = job.id
    ctx.logFocusJobId.value = String(job.id || "")
    ctx.loadLogs(ctx.logFocusJobId.value)
    ctx.editorVisible.value = true
    ctx.form.id = job.id
    ctx.form.name = job.name ?? ""
    ctx.form.folder = job.folder ?? ""
    ctx.form.cron = job.cron ?? "0 * * * *"
    ctx.form.command = job.command ?? ""
    ctx.form.args = args.length ? args : [""]
    ctx.form.workDir = job.workDir ?? ""
    ctx.form.inheritEnv = job.inheritEnv !== false
    ctx.form.flagProcessCreation = String(job.flagProcessCreation ?? "")

    const timeout = Number(job.timeout)
    ctx.form.timeout = Number.isFinite(timeout) && timeout > 0 ? timeout : 0
    ctx.form.concurrencyPolicy = job.concurrencyPolicy ? String(job.concurrencyPolicy) : "skip"
    ctx.form.enabled = !!job.enabled
    ctx.form.maxConsecutiveFailures = ctx.normalizeMaxConsecutiveFailures(job.maxConsecutiveFailures)
    ctx.markFormClean()
    return true
  }

  function resetForm() {
    return resetFormWithEditor(true)
  }

  function resetFormWithEditor(showEditor = true) {
    ctx.selectedJobId.value = ""
    ctx.logFocusJobId.value = ""
    ctx.loadLogs("")
    ctx.editorVisible.value = !!showEditor
    ctx.form.id = ""
    ctx.form.name = ""
    ctx.form.folder = ""
    ctx.form.cron = "0 * * * *"
    ctx.form.command = ""
    ctx.form.args = [""]
    ctx.form.workDir = ""
    ctx.form.inheritEnv = false
    ctx.form.flagProcessCreation = ""
    ctx.form.timeout = 0
    ctx.form.concurrencyPolicy = "skip"
    ctx.form.enabled = true
    ctx.form.maxConsecutiveFailures = 3
    ctx.markFormClean()
    return true
  }

  async function editJob(job) {
    if (!loadJobToForm(job)) {
      return
    }
  }

  async function selectJob(jobId) {
    const id = typeof jobId === "string" ? jobId : ""
    ctx.selectedJobId.value = id
    ctx.editorVisible.value = false
    await ctx.focusLogs(id)
    return true
  }

  async function saveJob() {
    ctx.error.value = ""
    try {
      const args = normalizeArgs(ctx.form.args)
      const existing = ctx.form.id ? findJobById(ctx.form.id) : null
      const existingHotkey = String(existing?.hotkey || "")

      const savedRaw = await ctx.callCronT(5000, "UpsertJob", {
        id: ctx.form.id,
        name: ctx.form.name,
        folder: ctx.form.folder,
        cron: ctx.form.cron,
        command: ctx.form.command,
        args,
        workDir: ctx.form.workDir,
        inheritEnv: ctx.form.inheritEnv !== false,
        hotkey: existingHotkey,
        flagProcessCreation: String(ctx.form.flagProcessCreation ?? ""),
        timeout: Number(ctx.form.timeout) || 0,
        concurrencyPolicy: ctx.form.concurrencyPolicy,
        enabled: ctx.form.enabled,
        maxConsecutiveFailures: ctx.normalizeMaxConsecutiveFailures(ctx.form.maxConsecutiveFailures),
      })

      const saved = ctx.normalizeObjectResult(savedRaw)
      if (!saved?.id) {
        throw new Error(ctx.t("errors.failed_to_save_job"))
      }

      await refreshJobs()
      loadJobToForm(saved)
      await ctx.focusLogs(String(saved.id || ""))
      ctx.dismissToast()
      ctx.triggerEditorPulse("success")
    } catch (e) {
      ctx.error.value = String(e)
      ctx.dismissToast()
      ctx.triggerEditorPulse("error")
    }
  }

  async function copyJob(job) {
    ctx.error.value = ""
    try {
      if (!job || typeof job !== "object") {
        return
      }

      const srcName = String(job?.name || "").trim()
      const srcCommand = String(job?.command || "").trim()
      const baseName = srcName || srcCommand || "Job"
      const copiedName = `${baseName} (${ctx.t("common.copy")})`
      const args = normalizeArgs(job?.args)

      const savedRaw = await ctx.callCronT(5000, "UpsertJob", {
        id: "",
        name: copiedName,
        folder: job?.folder ?? "",
        cron: job?.cron ?? "0 * * * *",
        command: job?.command ?? "",
        args,
        workDir: job?.workDir ?? "",
        inheritEnv: job?.inheritEnv !== false,
        hotkey: "",
        flagProcessCreation: String(job?.flagProcessCreation ?? ""),
        timeout: Number(job?.timeout) || 0,
        concurrencyPolicy: job?.concurrencyPolicy ? String(job.concurrencyPolicy) : "skip",
        enabled: !!job?.enabled,
        maxConsecutiveFailures: ctx.normalizeMaxConsecutiveFailures(job?.maxConsecutiveFailures),
      })

      const saved = ctx.normalizeObjectResult(savedRaw)
      if (!saved?.id) {
        throw new Error(ctx.t("errors.failed_to_save_job"))
      }

      await refreshJobs()
      loadJobToForm(saved)
      await ctx.focusLogs(String(saved.id || ""))
      ctx.showToast(ctx.t("toast.copied_with_name", { name: saved?.name || copiedName }), "success")
    } catch (e) {
      ctx.reportError(e)
    }
  }

  async function deleteJob(id) {
    ctx.error.value = ""
    try {
      const jobId = typeof id === "string" ? id : ""
      if (!jobId || ctx.pendingDeleteJobs.has(jobId)) {
        return
      }

      const deleting = findJobById(jobId)
      const deletingName = String(deleting?.name || "").trim() || String(deleting?.command || "").trim() || jobId
      const wasSelected = ctx.selectedJobId.value === jobId

      ctx.jobs.value = Array.isArray(ctx.jobs.value) ? ctx.jobs.value.filter((j) => String(j?.id || "") !== jobId) : []
      if (wasSelected) {
        resetFormWithEditor(false)
      }

      const undo = async () => {
        const pending = ctx.pendingDeleteJobs.get(jobId)
        if (!pending) {
          return
        }
        clearTimeout(pending.timer)
        ctx.pendingDeleteJobs.delete(jobId)
        await refreshJobs()
        if (wasSelected) {
          const restored = findJobById(jobId)
          if (restored) {
            loadJobToForm(restored)
          }
        }
        ctx.showToast(ctx.t("toast.delete_undone"), "success")
      }

      ctx.showToast(ctx.t("toast.deleted_with_name", { name: deletingName }), "info", {
        actionLabel: ctx.t("common.undo"),
        onAction: undo,
        durationMs: 5000,
      })

      const timer = setTimeout(async () => {
        ctx.pendingDeleteJobs.delete(jobId)
        try {
          await ctx.callCronT(5000, "DeleteJob", jobId)
        } catch (e) {
          ctx.reportError(e)
        }
        await refreshJobs()
      }, 5000)

      ctx.pendingDeleteJobs.set(jobId, { timer })
    } catch (e) {
      ctx.error.value = String(e)
    }
  }

  async function setJobFolder(jobId, folder) {
    ctx.error.value = ""
    try {
      const id = typeof jobId === "string" ? jobId : ""
      if (!id) {
        return
      }
      const updated = await callSetJobFolder(id, typeof folder === "string" ? folder : "")
      await refreshJobsAndSyncSelectedJob(ctx, { updatedJob: updated })
    } catch (e) {
      ctx.error.value = String(e)
    }
  }

  async function setJobsFolder(jobIds, folder) {
    ctx.error.value = ""
    try {
      const ids = normalizeJobIds(jobIds)
      if (!ids.length) {
        return
      }
      const f = typeof folder === "string" ? folder : ""

      for (const id of ids) {
        await callSetJobFolder(id, f)
      }

      await refreshJobsAndSyncSelectedJob(ctx, { selectedIds: ids })
    } catch (e) {
      ctx.error.value = String(e)
    }
  }

  async function toggleJob(job) {
    ctx.error.value = ""
    try {
      const updatedRaw = await ctx.callCronT(5000, "SetJobEnabled", job.id, !job.enabled)
      const updated = requireUpdatedJob(ctx, updatedRaw)
      await refreshJobsAndSyncSelectedJob(ctx, { updatedJob: updated })
    } catch (e) {
      ctx.error.value = String(e)
    }
  }

  async function setJobsEnabled(jobIds, enabled) {
    ctx.error.value = ""
    try {
      const ids = normalizeJobIds(jobIds)
      if (!ids.length) {
        return
      }
      const v = !!enabled
      for (const id of ids) {
        const updatedRaw = await ctx.callCronT(5000, "SetJobEnabled", id, v)
        requireUpdatedJob(ctx, updatedRaw)
      }

      await refreshJobsAndSyncSelectedJob(ctx, { selectedIds: ids })
    } catch (e) {
      ctx.reportError(e)
    }
  }

  async function runNow(jobId) {
    ctx.error.value = ""
    try {
      const job = findJobById(jobId)
      const entryRaw = await ctx.callCronT(getRunTimeoutMs(job?.timeout), "RunNow", jobId)
      resolveRunEntry(entryRaw, "errors.failed_to_run_job")
    } catch (e) {
      ctx.reportError(e)
    }
  }

  async function runPreviewFromForm() {
    ctx.error.value = ""
    try {
      const args = normalizeArgs(ctx.form.args)
      const entryRaw = await ctx.callCronT(getRunTimeoutMs(ctx.form.timeout), "RunPreview", {
        command: ctx.form.command,
        args,
        workDir: ctx.form.workDir,
        inheritEnv: ctx.form.inheritEnv !== false,
        flagProcessCreation: String(ctx.form.flagProcessCreation ?? ""),
        timeout: Number(ctx.form.timeout) || 0,
        jobId: ctx.form.id,
        jobName: ctx.form.name,
      })
      resolveRunEntry(entryRaw, "errors.failed_to_run_preview")
    } catch (e) {
      ctx.reportError(e)
    }
  }

  return {
    refreshJobs,
    loadJobToForm,
    resetForm,
    resetFormWithEditor,
    editJob,
    selectJob,
    saveJob,
    copyJob,
    deleteJob,
    setJobFolder,
    setJobsFolder,
    toggleJob,
    setJobsEnabled,
    runNow,
    runPreviewFromForm,
  }
}
