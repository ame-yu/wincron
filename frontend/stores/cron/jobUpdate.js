import { normalizeJobIds } from "../../ui/selectionTokens.js"

function findJobById(jobs, jobId) {
  const id = String(jobId || "")
  if (!id) {
    return null
  }
  return Array.isArray(jobs) ? jobs.find((job) => String(job?.id || "") === id) || null : null
}

export function requireUpdatedJob(ctx, updatedRaw) {
  const updated = ctx.normalizeObjectResult(updatedRaw)
  if (!updated?.id) {
    throw new Error(ctx.t("errors.failed_to_update_job"))
  }
  return updated
}

export async function refreshJobsAndSyncSelectedJob(ctx, options = {}) {
  await ctx.refreshJobs()

  const selectedId = String(ctx.selectedJobId.value || "")
  if (!selectedId || ctx.isFormDirty()) {
    return null
  }

  const selectedIds = normalizeJobIds([
    ...(Array.isArray(options.selectedIds) ? options.selectedIds : []),
    options.updatedJob?.id,
  ])
  if (!selectedIds.includes(selectedId)) {
    return null
  }

  const job = findJobById(ctx.jobs.value, selectedId) || findJobById([options.updatedJob], selectedId)
  if (job) {
    ctx.loadJobToForm(job)
  }
  return job
}
