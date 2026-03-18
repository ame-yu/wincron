import { defineStore } from "pinia"
import i18n from "../i18n.js"
import { createConfigActions } from "./cron/config.js"
import { createJobActions } from "./cron/jobs.js"
import { createLifecycleActions } from "./cron/lifecycle.js"
import { createLogActions } from "./cron/logs.js"
import { createRuntime } from "./cron/runtime.js"
import { createSettingsActions } from "./cron/settings.js"
import { createCronState } from "./cron/state.js"

export const useCronStore = defineStore("cron", () => {
  const t = (...args) => i18n.global.t(...args)
  const state = createCronState()
  const runtime = createRuntime()
  const ctx = { t, ...state, ...runtime }

  Object.assign(ctx, createLogActions(ctx))
  Object.assign(ctx, createSettingsActions(ctx))
  Object.assign(ctx, createJobActions(ctx))
  Object.assign(ctx, createConfigActions(ctx))
  Object.assign(ctx, createLifecycleActions(ctx))

  return {
    error: ctx.error,
    toast: ctx.toast,
    toastKind: ctx.toastKind,
    toastActionLabel: ctx.toastActionLabel,
    editorPulse: ctx.editorPulse,
    silentStart: ctx.silentStart,
    lightweightMode: ctx.lightweightMode,
    autoStart: ctx.autoStart,
    runInTray: ctx.runInTray,
    globalEnabled: ctx.globalEnabled,
    jobs: ctx.jobs,
    jobsLoaded: ctx.jobsLoaded,
    runningJobIds: ctx.runningJobIds,
    selectedJobId: ctx.selectedJobId,
    logFocusJobId: ctx.logFocusJobId,
    focusLogs: ctx.focusLogs,
    logs: ctx.logs,
    logsLoading: ctx.logsLoading,
    logsLoadingMore: ctx.logsLoadingMore,
    logsHasMore: ctx.logsHasMore,
    logsTotalCount: ctx.logsTotalCount,
    editorVisible: ctx.editorVisible,
    form: ctx.form,
    isFormDirty: ctx.isFormDirty,
    refreshJobs: ctx.refreshJobs,
    loadJobToForm: ctx.loadJobToForm,
    editJob: ctx.editJob,
    selectJob: ctx.selectJob,
    resetForm: ctx.resetForm,
    saveJob: ctx.saveJob,
    copyJob: ctx.copyJob,
    deleteJob: ctx.deleteJob,
    setJobFolder: ctx.setJobFolder,
    setJobsFolder: ctx.setJobsFolder,
    toggleJob: ctx.toggleJob,
    setJobsEnabled: ctx.setJobsEnabled,
    runNow: ctx.runNow,
    runPreviewFromForm: ctx.runPreviewFromForm,
    loadLogs: ctx.loadLogs,
    loadMoreLogs: ctx.loadMoreLogs,
    clearLogs: ctx.clearLogs,
    clearJobLogs: ctx.clearJobLogs,
    copyLastOutput: ctx.copyLastOutput,
    copyLogOutput: ctx.copyLogOutput,
    terminateRunningLog: ctx.terminateRunningLog,
    terminateRunningJob: ctx.terminateRunningJob,
    deleteLogEntry: ctx.deleteLogEntry,
    exportConfig: ctx.exportConfig,
    checkImportConflicts: ctx.checkImportConflicts,
    importConfig: ctx.importConfig,
    loadSettings: ctx.loadSettings,
    loadGlobalEnabled: ctx.loadGlobalEnabled,
    setSilentStart: ctx.setSilentStart,
    setLightweightMode: ctx.setLightweightMode,
    setAutoStart: ctx.setAutoStart,
    setRunInTray: ctx.setRunInTray,
    setGlobalEnabled: ctx.setGlobalEnabled,
    validateJobHotkey: ctx.validateJobHotkey,
    setJobHotkey: ctx.setJobHotkey,
    pauseHotkeys: ctx.pauseHotkeys,
    resumeHotkeys: ctx.resumeHotkeys,
    previewNextRuns: ctx.previewNextRuns,
    openDataDir: ctx.openDataDir,
    openEnvironmentVariables: ctx.openEnvironmentVariables,
    init: ctx.init,
    triggerToastAction: ctx.triggerToastAction,
    dispose: ctx.dispose,
  }
})
