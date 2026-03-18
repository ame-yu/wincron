import { refreshJobsAndSyncSelectedJob, requireUpdatedJob } from "./jobUpdate.js"
import { previewCronNextRuns as previewCronNextRunsList } from "../../ui/cron.js"

export function createSettingsActions(ctx) {
  async function updateSetting(callName, value, apply) {
    try {
      await ctx.callSettingsT(5000, callName, value)
      apply(value)
      ctx.showToast(ctx.t("toast.saved"), "success")
    } catch (e) {
      ctx.reportError(e)
      await loadSettings()
    }
  }

  async function loadSettings() {
    try {
      const result = await ctx.callSettingsT(5000, "GetSettings")
      const settings = ctx.normalizeSettingsResult(result)
      ctx.silentStart.value = !!settings?.silentStart
      ctx.lightweightMode.value = settings?.lightweightMode !== false
      ctx.autoStart.value = !!settings?.autoStart
      ctx.runInTray.value = settings?.runInTray !== false
    } catch (e) {
      ctx.reportError(e)
    }
  }

  async function loadGlobalEnabled() {
    try {
      const result = await ctx.callCronT(5000, "GetGlobalEnabled")
      ctx.globalEnabled.value = !!result
    } catch (e) {
      ctx.reportError(e)
    }
  }

  async function setGlobalEnabled(enabled) {
    const v = !!enabled
    try {
      await ctx.callCronT(5000, "SetGlobalEnabled", v)
      ctx.globalEnabled.value = v
      ctx.showToast(v ? ctx.t("global.enabled") : ctx.t("global.disabled"), "success")
    } catch (e) {
      ctx.reportError(e)
    }
  }

  async function validateJobHotkey(hotkey) {
    const hk = typeof hotkey === "string" ? hotkey : ""
    const result = await ctx.callCronT(5000, "ValidateJobHotkey", hk)
    return typeof result === "string" ? result : String(result ?? "")
  }

  async function setJobHotkey(jobId, hotkey) {
    ctx.error.value = ""
    try {
      const id = typeof jobId === "string" ? jobId : ""
      if (!id) {
        return
      }
      const hk = typeof hotkey === "string" ? hotkey : ""
      const updatedRaw = await ctx.callCronT(5000, "SetJobHotkey", id, hk)
      const updated = requireUpdatedJob(ctx, updatedRaw)
      await refreshJobsAndSyncSelectedJob(ctx, { updatedJob: updated })
    } catch (e) {
      ctx.error.value = String(e)
      ctx.showToast(ctx.error.value, "danger")
      throw e
    }
  }

  async function pauseHotkeys() {
    try {
      await ctx.callCronT(5000, "PauseHotkeys")
    } catch (e) {
      ctx.reportError(e)
    }
  }

  async function resumeHotkeys() {
    try {
      await ctx.callCronT(5000, "ResumeHotkeys")
    } catch (e) {
      ctx.reportError(e)
    }
  }

  async function previewNextRuns(cronExpr, count = 3) {
    return previewCronNextRunsList(cronExpr, { count })
  }

  async function setSilentStart(enabled) {
    await updateSetting("SetSilentStart", !!enabled, (next) => {
      ctx.silentStart.value = next
    })
  }

  async function setLightweightMode(enabled) {
    await updateSetting("SetLightweightMode", !!enabled, (next) => {
      ctx.lightweightMode.value = next
    })
  }

  async function setAutoStart(enabled) {
    await updateSetting("SetAutoStart", !!enabled, (next) => {
      ctx.autoStart.value = next
    })
  }

  async function setRunInTray(enabled) {
    await updateSetting("SetRunInTray", !!enabled, (next) => {
      ctx.runInTray.value = next
    })
  }

  async function openDataDir() {
    ctx.error.value = ""
    try {
      const result = await ctx.callSettingsT(5000, "OpenDataDir")
      const dir = typeof result === "string" ? result : ""
      ctx.showToast(dir ? ctx.t("toast.opened_data_dir_with_path", { dir }) : ctx.t("toast.opened_data_dir"), "success")
      return dir
    } catch (e) {
      ctx.reportError(e)
    }
  }

  async function openEnvironmentVariables() {
    ctx.error.value = ""
    try {
      await ctx.callSettingsT(5000, "OpenEnvironmentVariables")
      ctx.showToast(ctx.t("toast.opened_environment_variables"), "success")
    } catch (e) {
      ctx.reportError(e)
    }
  }

  return {
    loadSettings,
    loadGlobalEnabled,
    setSilentStart,
    setLightweightMode,
    setAutoStart,
    setRunInTray,
    setGlobalEnabled,
    validateJobHotkey,
    setJobHotkey,
    pauseHotkeys,
    resumeHotkeys,
    previewNextRuns,
    openDataDir,
    openEnvironmentVariables,
  }
}
