import { Call } from "@wailsio/runtime"

const cronServiceName = "main.CronService"
const settingsServiceName = "main.SettingsService"
const configServiceName = "main.ConfigService"

function parseJson(value) {
  if (typeof value !== "string") {
    return undefined
  }
  try {
    return JSON.parse(value)
  } catch {
    return undefined
  }
}

function pick(obj, keys) {
  for (const key of keys) {
    const v = obj?.[key]
    if (v != null) {
      return v
    }
  }
  return undefined
}

function normalize(result, { kind, keys, defaultValue }) {
  if (kind === "array") {
    if (Array.isArray(result)) {
      return result
    }
    if (typeof result === "string") {
      const parsed = parseJson(result)
      return Array.isArray(parsed) ? parsed : defaultValue
    }
    if (result && typeof result === "object") {
      const candidate = pick(result, keys)
      return Array.isArray(candidate) ? candidate : defaultValue
    }
    return defaultValue
  }

  if (kind === "settings") {
    if (!result) {
      return defaultValue
    }
    if (typeof result === "string") {
      return parseJson(result) ?? defaultValue
    }
    if (result && typeof result === "object") {
      return result.settings ?? result.data ?? result.result ?? result
    }
    return defaultValue
  }

  if (!result) {
    return defaultValue
  }
  if (typeof result === "string") {
    return parseJson(result) ?? defaultValue
  }
  if (result && typeof result === "object") {
    return pick(result, keys) ?? result
  }
  return defaultValue
}

function normalizeArrayResult(result, keys = ["result", "data", "jobs", "items"]) {
  return normalize(result, { kind: "array", keys, defaultValue: [] })
}

function normalizeSettingsResult(result) {
  const defaults = { runInTray: true, lightweightMode: true }
  const settings = normalize(result, { kind: "settings", keys: [], defaultValue: null })
  return settings == null ? defaults : { ...defaults, ...settings }
}

function normalizeObjectResult(result, keys = ["result", "data", "item"]) {
  return normalize(result, { kind: "object", keys, defaultValue: null })
}

function normalizeStringArrayResult(result) {
  return normalizeArrayResult(result, ["result", "data", "items"]).filter((v) => typeof v === "string")
}

function withTimeout(promise, ms) {
  if (!ms || ms <= 0) {
    return promise
  }
  let timer = null
  return Promise.race([
    promise,
    new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error(`timeout after ${ms}ms`)), ms)
    }),
  ]).finally(() => {
    if (timer) {
      clearTimeout(timer)
    }
  })
}

export function createRuntime() {
  function call(serviceName, methodName, ...args) {
    return Call.ByName(`${serviceName}.${methodName}`, ...args)
  }

  function callWithTimeout(serviceName, methodName, timeoutMs, ...args) {
    return withTimeout(call(serviceName, methodName, ...args), timeoutMs)
  }

  return {
    normalizeArrayResult,
    normalizeObjectResult,
    normalizeSettingsResult,
    normalizeStringArrayResult,
    callCronT: (timeoutMs, methodName, ...args) => callWithTimeout(cronServiceName, methodName, timeoutMs, ...args),
    callSettingsT: (timeoutMs, methodName, ...args) =>
      callWithTimeout(settingsServiceName, methodName, timeoutMs, ...args),
    callConfigT: (timeoutMs, methodName, ...args) => callWithTimeout(configServiceName, methodName, timeoutMs, ...args),
  }
}
