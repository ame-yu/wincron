import cronTimeModule from "cron/dist/time.js"

const { CronTime } = cronTimeModule

export const AT_STARTUP_LABEL = "At startup"

const cronDescriptorAliases = Object.freeze({
  "@annually": "@yearly",
  "@midnight": "@daily",
})

const durationUnits = Object.freeze({
  ns: 1e-6,
  us: 1e-3,
  "\u00b5s": 1e-3,
  "\u03bcs": 1e-3,
  ms: 1,
  s: 1000,
  m: 60 * 1000,
  h: 60 * 60 * 1000,
})

function toErrorMessage(error) {
  return error instanceof Error ? error.message : String(error ?? "invalid cron")
}

function splitCronTimeZone(expr) {
  const match = String(expr || "").trim().match(/^(?:CRON_TZ|TZ)=([^\s]+)\s+(.+)$/i)
  if (!match) {
    return { expr: String(expr || "").trim(), timeZone: undefined }
  }
  return { expr: match[2].trim(), timeZone: match[1] }
}

function normalizeDescriptor(expr) {
  const lower = String(expr || "").trim().toLowerCase()
  return cronDescriptorAliases[lower] ?? String(expr || "").trim()
}

function parseEveryDelayMs(expr) {
  const raw = String(expr || "").trim()
  if (!raw.toLowerCase().startsWith("@every ")) {
    return null
  }

  let duration = raw.slice(7).trim()
  if (!duration) {
    throw new Error(`invalid cron: failed to parse duration ${raw}`)
  }

  let sign = 1
  if (duration.startsWith("+")) {
    duration = duration.slice(1)
  } else if (duration.startsWith("-")) {
    sign = -1
    duration = duration.slice(1)
  }

  if (!duration) {
    throw new Error(`invalid cron: failed to parse duration ${raw}`)
  }

  const tokenPattern = /(\d+(?:\.\d+)?|\.\d+)(ns|us|\u00b5s|\u03bcs|ms|s|m|h)/gy
  let totalMs = 0

  while (tokenPattern.lastIndex < duration.length) {
    const match = tokenPattern.exec(duration)
    if (!match) {
      throw new Error(`invalid cron: failed to parse duration ${raw}`)
    }

    const value = Number(match[1])
    const unitMs = durationUnits[match[2]]
    if (!Number.isFinite(value) || !Number.isFinite(unitMs)) {
      throw new Error(`invalid cron: failed to parse duration ${raw}`)
    }
    totalMs += value * unitMs
  }

  totalMs *= sign
  if (!Number.isFinite(totalMs)) {
    throw new Error(`invalid cron: failed to parse duration ${raw}`)
  }

  if (totalMs < 1000) {
    totalMs = 1000
  }

  return totalMs - (totalMs % 1000)
}

function toIsoString(date) {
  const next = date instanceof Date ? date : new Date(date)
  if (!Number.isFinite(next.getTime())) {
    throw new Error("failed to compute next run")
  }
  return next.toISOString()
}

function computeEveryNextRuns(expr, count, now) {
  const delayMs = parseEveryDelayMs(expr)
  if (delayMs == null) {
    return []
  }
  const total = Math.max(0, Math.trunc(Number(count) || 0))
  if (total === 0) {
    return []
  }
  const base = now instanceof Date ? now : new Date(now)
  let nextMs = base.getTime() + delayMs - base.getMilliseconds()
  const runs = []
  for (let index = 0; index < total; index += 1) {
    runs.push(toIsoString(nextMs))
    nextMs += delayMs
  }
  return runs
}

function computeCronTimeNextRuns(expr, timeZone, count) {
  try {
    const total = Math.max(0, Math.trunc(Number(count) || 0))
    if (total === 0) {
      return []
    }
    return new CronTime(expr, timeZone)
      .sendAt(total)
      .map((next) => {
        const iso = typeof next?.toISO === "function" ? next.toISO({ suppressMilliseconds: true }) : ""
        if (!iso) {
          throw new Error("failed to compute next run")
        }
        return iso
      })
  } catch (error) {
    const message = toErrorMessage(error)
    throw new Error(message.startsWith("invalid cron:") ? message : `invalid cron: ${message}`)
  }
}

export function previewCronNextRun(cronExpr, options = {}) {
  return previewCronNextRuns(cronExpr, { ...options, count: 1 })[0] ?? ""
}

export function previewCronNextRuns(cronExpr, options = {}) {
  const raw = String(cronExpr || "").trim()
  if (!raw) {
    return []
  }

  const { expr, timeZone } = splitCronTimeZone(raw)
  const normalized = normalizeDescriptor(expr)
  if (!normalized) {
    return []
  }

  const count = Math.max(0, Math.trunc(Number(options.count) || 3))
  if (count === 0) {
    return []
  }

  if (normalized.toLowerCase() === "@reboot") {
    return Array.from({ length: count }, () => AT_STARTUP_LABEL)
  }

  const everyNextRuns = computeEveryNextRuns(normalized, count, options.now ?? new Date())
  if (everyNextRuns.length > 0) {
    return everyNextRuns
  }

  return computeCronTimeNextRuns(normalized, timeZone, count)
}

export function getJobNextRun(job, options = {}) {
  if (!job?.enabled) {
    return ""
  }
  try {
    return previewCronNextRun(job?.cron, options)
  } catch {
    return ""
  }
}

export function withJobNextRunAt(job, options = {}) {
  if (!job || typeof job !== "object") {
    return job
  }
  return {
    ...job,
    nextRunAt: getJobNextRun(job, options),
  }
}

export function withJobsNextRunAt(jobs, options = {}) {
  const now = options.now ?? new Date()
  return Array.isArray(jobs) ? jobs.map((job) => withJobNextRunAt(job, { ...options, now })) : []
}
