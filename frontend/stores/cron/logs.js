import { Clipboard } from "@wailsio/runtime"

export const LOG_PAGE_SIZE = 20

export function createLogActions(ctx) {
  const toId = (value) => String(value || "")
  const toLogs = (list) => (Array.isArray(list) ? list : [])
  const sameList = (a, b) => a.length === b.length && a.every((value, index) => value === b[index])
  const withId = (value, fn) => {
    const id = toId(value)
    if (id) {
      return fn(id)
    }
  }
  const outputText = (entry) => String(entry?.stdout || "") || String(entry?.stderr || "")
  const state = {
    lastId: null,
    inflightId: "",
    inflightMode: "",
    inflight: null,
    seq: 0,
    liveEntries: new Map(),
    loadedStoredCount: 0,
  }

  const isRunningEntry = (entry) => !!String(entry?.startedAt || "").trim() && !String(entry?.finishedAt || "").trim()
  const matchesFocus = (entry, focusJobId = ctx.logFocusJobId.value || "") =>
    !focusJobId || toId(entry?.jobId) === focusJobId

  const replaceLog = (list, entry) => {
    const id = toId(entry?.id)
    const source = toLogs(list)
    if (!id) {
      return [...source]
    }

    let found = false
    const next = source.map((item) => {
      if (toId(item?.id) !== id) {
        return item
      }
      found = true
      return entry
    })
    return found ? next : [...next, entry]
  }

  const mergeLogs = (list, focusJobId = ctx.logFocusJobId.value || "") => {
    const merged = new Map()

    const append = (entry) => {
      const id = toId(entry?.id)
      if (!id || !matchesFocus(entry, focusJobId)) {
        return
      }
      merged.set(id, entry)
    }

    toLogs(list).forEach(append)
    for (const entry of state.liveEntries.values()) {
      append(entry)
    }
    return [...merged.values()]
  }

  const setLogs = (list, focusJobId = ctx.logFocusJobId.value || "") => {
    ctx.logs.value = mergeLogs(list, focusJobId)
  }

  const syncRunningJobIds = () => {
    const next = [...new Set(
      [...state.liveEntries.values()]
        .map((entry) => toId(entry?.jobId))
        .filter(Boolean),
    )].sort()
    const current = Array.isArray(ctx.runningJobIds?.value) ? [...ctx.runningJobIds.value] : []
    if (!sameList(current, next)) {
      ctx.runningJobIds.value = next
    }
  }

  const syncLiveEntriesFromPage = (list, focusJobId = "") => {
    const scopeId = toId(focusJobId)

    if (scopeId) {
      for (const [entryId, entry] of state.liveEntries.entries()) {
        if (toId(entry?.jobId) === scopeId) {
          state.liveEntries.delete(entryId)
        }
      }
    } else {
      state.liveEntries.clear()
    }

    for (const entry of toLogs(list)) {
      if (!isRunningEntry(entry)) {
        continue
      }
      const entryId = toId(entry?.id)
      if (!entryId) {
        continue
      }
      state.liveEntries.set(entryId, entry)
    }

    syncRunningJobIds()
  }

  const normalizeLogPage = (result) => {
    const page = ctx.normalizeObjectResult(result, ["result", "data", "page"]) || {}
    const items = ctx.normalizeArrayResult(page?.items ?? page?.entries ?? page?.logs ?? [])
    const storedCount = Number(page?.storedCount)
    const totalCount = Number(page?.totalCount)

    return {
      items,
      storedCount: Number.isFinite(storedCount) ? Math.max(0, Math.trunc(storedCount)) : items.length,
      totalCount: Number.isFinite(totalCount) ? Math.max(0, Math.trunc(totalCount)) : items.length,
      hasMore: !!page?.hasMore,
    }
  }

  const applyLogPage = (list, focusJobId, { append = false, storedCount = 0, totalCount = 0, hasMore = false } = {}) => {
    if (!append) {
      syncLiveEntriesFromPage(list, focusJobId)
    }
    const source = append ? [...toLogs(ctx.logs.value), ...toLogs(list)] : toLogs(list)
    setLogs(source, focusJobId)
    state.loadedStoredCount = append ? state.loadedStoredCount + storedCount : storedCount
    ctx.logsTotalCount.value = Math.max(append ? ctx.logsTotalCount.value : 0, totalCount, source.length)
    ctx.logsHasMore.value = hasMore
    state.lastId = focusJobId
  }

  const resetPagination = () => {
    state.loadedStoredCount = 0
    ctx.logsHasMore.value = false
    ctx.logsTotalCount.value = 0
    ctx.logsLoading.value = false
    ctx.logsLoadingMore.value = false
  }

  const setPagingLoading = (mode, value) => {
    ctx.logsLoading.value = mode === "load" ? value : false
    ctx.logsLoadingMore.value = mode === "more" ? value : false
  }

  const clearInflight = (id, mode, seq) => {
    if (seq === state.seq && state.inflightId === id && state.inflightMode === mode) {
      state.inflightId = ""
      state.inflightMode = ""
      state.inflight = null
    }
  }

  const requestLogPage = (id, { mode, offset = 0, append = false, reset = false, onError } = {}) => {
    const seq = ++state.seq
    state.inflightId = id
    state.inflightMode = mode

    if (reset) {
      resetPagination()
      setLogs([], id)
    }

    setPagingLoading(mode, true)
    state.inflight = (async () => {
      try {
        const result = await ctx.callCronT(5000, "ListLogsPage", id, offset, LOG_PAGE_SIZE)
        const page = normalizeLogPage(result)
        if (seq === state.seq) {
          applyLogPage(page.items, id, {
            append,
            storedCount: page.storedCount,
            totalCount: page.totalCount,
            hasMore: page.hasMore,
          })
        }
        return append ? ctx.logs.value : page.items
      } catch (e) {
        if (seq === state.seq) {
          onError?.(e)
        }
        return append ? ctx.logs.value : []
      } finally {
        if (seq === state.seq) {
          setPagingLoading(mode, false)
        }
        clearInflight(id, mode, seq)
      }
    })()

    return state.inflight
  }

  const runAction = async (action) => {
    ctx.error.value = ""
    try {
      return await action()
    } catch (e) {
      ctx.reportError(e)
      return null
    }
  }

  async function copyEntryOutput(entry) {
    const text = outputText(entry)
    if (!text) {
      return false
    }
    await Clipboard.SetText(text)
    ctx.showToast(ctx.t("toast.copied"), "success")
    return true
  }

  function syncLiveLog(entry) {
    const id = toId(entry?.id)
    if (!id) {
      return
    }

    const existedInLive = state.liveEntries.has(id)
    const existedInLogs = toLogs(ctx.logs.value).some((item) => toId(item?.id) === id)

    if (isRunningEntry(entry)) {
      const existing = toLogs(ctx.logs.value).find((item) => toId(item?.id) === id)
      if (existing && !isRunningEntry(existing)) {
        return
      }
      state.liveEntries.set(id, entry)
    } else {
      state.liveEntries.delete(id)
    }

    syncRunningJobIds()

    if (!matchesFocus(entry)) {
      return
    }

    if (isRunningEntry(entry) && !existedInLive && !existedInLogs) {
      ctx.logsTotalCount.value += 1
    }
    ctx.logs.value = replaceLog(ctx.logs.value, entry)
    if (ctx.logsTotalCount.value < ctx.logs.value.length) {
      ctx.logsTotalCount.value = ctx.logs.value.length
    }
  }

  async function focusLogs(jobId, options = {}) {
    const id = toId(jobId)
    const force = !!options?.force
    const changed = ctx.logFocusJobId.value !== id
    ctx.logFocusJobId.value = id
    if (changed || force || state.lastId !== id) {
      await loadLogs(id, { ...options, reset: changed || !!options?.reset })
    }
  }

  async function reloadFocusedLogs() {
    await loadLogs(ctx.logFocusJobId.value || "", { force: true })
  }

  async function loadLogs(jobId, options = {}) {
    const id = toId(jobId)
    const force = !!options?.force
    const reset = !!options?.reset
    if (!force && state.lastId === id) {
      return ctx.logs.value
    }
    if (!force && state.inflightId === id && state.inflightMode === "load" && state.inflight) {
      return state.inflight
    }

    return requestLogPage(id, {
      mode: "load",
      reset,
      onError: (e) => {
        resetPagination()
        setLogs([], id)
        state.lastId = null
        ctx.reportError(e)
      },
    })
  }

  async function loadMoreLogs(jobId = ctx.logFocusJobId.value || "") {
    const id = toId(jobId)
    if (state.lastId === null || state.lastId !== id) {
      return loadLogs(id)
    }

    const needsMore = ctx.logsHasMore.value || toLogs(ctx.logs.value).length < (Number(ctx.logsTotalCount.value) || 0)
    if (ctx.logsLoading.value || ctx.logsLoadingMore.value || !needsMore) {
      return ctx.logs.value
    }
    if (state.inflightId === id && state.inflightMode === "more" && state.inflight) {
      return state.inflight
    }

    return requestLogPage(id, {
      mode: "more",
      offset: state.loadedStoredCount,
      append: true,
      onError: (e) => ctx.reportError(e),
    })
  }

  async function clearLogs() {
    await runAction(async () => {
      await ctx.callCronT(5000, "ClearLogs")
      resetPagination()
      setLogs([])
      state.lastId = null
    })
  }

  async function clearJobLogs(jobId) {
    await runAction(async () => {
      await withId(jobId, async (id) => {
        await ctx.callCronT(5000, "ClearJobLogs", id)
        if (state.lastId === id) {
          resetPagination()
          state.lastId = null
        }
        if (ctx.logFocusJobId.value === id) {
          await reloadFocusedLogs()
        }
      })
    })
  }

  async function copyLastOutput(jobId) {
    await runAction(async () => {
      await withId(jobId, async (id) => {
        const result = await ctx.callCronT(5000, "ListLogs", id, 4)
        const list = ctx.normalizeArrayResult(result)
        const entry = toLogs(list).find((item) => String(item?.finishedAt || "").trim()) || list[0] || null
        await copyEntryOutput(entry)
      })
    })
  }

  async function copyLogOutput(entry) {
    await runAction(async () => {
      await copyEntryOutput(entry)
    })
  }

  async function terminateRunningLog(entry) {
    await runAction(() => withId(entry?.id, (id) => ctx.callCronT(5000, "TerminateLogEntry", id)))
  }

  async function terminateRunningJob(jobId) {
    await runAction(async () => {
      const id = toId(jobId)
      if (!id) {
        return
      }

      const runningEntries = [...state.liveEntries.values()]
        .filter((entry) => toId(entry?.jobId) === id && isRunningEntry(entry))
        .sort((a, b) => Date.parse(b?.startedAt || "") - Date.parse(a?.startedAt || ""))

      for (const entry of runningEntries) {
        const entryId = toId(entry?.id)
        if (!entryId) {
          continue
        }
        await ctx.callCronT(5000, "TerminateLogEntry", entryId)
      }
    })
  }

  async function deleteLogEntry(entryId) {
    await runAction(async () => {
      await withId(entryId, async (id) => {
        const removed = toLogs(ctx.logs.value).find((entry) => toId(entry?.id) === id) || null
        await ctx.callCronT(5000, "DeleteLogEntry", id)
        state.liveEntries.delete(id)
        syncRunningJobIds()
        setLogs(toLogs(ctx.logs.value).filter((l) => toId(l?.id) !== id))
        if (removed && !isRunningEntry(removed) && matchesFocus(removed) && state.loadedStoredCount > 0) {
          state.loadedStoredCount -= 1
        }
        if (removed && matchesFocus(removed) && ctx.logsTotalCount.value > 0) {
          ctx.logsTotalCount.value -= 1
        }
        if (state.lastId === ctx.logFocusJobId.value && !ctx.logs.value.length) {
          state.lastId = null
        }
      })
    })
  }

  return {
    focusLogs,
    reloadFocusedLogs,
    loadLogs,
    loadMoreLogs,
    clearLogs,
    clearJobLogs,
    copyLastOutput,
    copyLogOutput,
    terminateRunningLog,
    terminateRunningJob,
    deleteLogEntry,
    syncLiveLog,
  }
}
