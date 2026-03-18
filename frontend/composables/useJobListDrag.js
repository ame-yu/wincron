import { useDragTransfer } from "./useDragTransfer.js"
import { normalizeJobIds, normalizeList, tokenFor } from "../ui/selectionTokens.js"

const defaultNormalizeFolder = (value) => String(value || "").trim()

const reorderList = (list, draggedValue, targetValue, insertAfter) => {
  if (!list.includes(draggedValue) || !list.includes(targetValue) || draggedValue === targetValue) {
    return null
  }

  const withoutDragged = list.filter((value) => value !== draggedValue)
  const targetIndex = withoutDragged.indexOf(targetValue)
  if (targetIndex < 0) return null

  const insertAt = insertAfter ? targetIndex + 1 : targetIndex
  return [...withoutDragged.slice(0, insertAt), draggedValue, ...withoutDragged.slice(insertAt)]
}

export function useJobListDrag(options = {}) {
  const cron = options.cron
  const jobs = options.jobs
  const folderManager = options.folderManager || {}
  const isSelected = typeof options.isSelected === "function" ? options.isSelected : () => false
  const getSelectedJobIdsExpanded =
    typeof options.getSelectedJobIdsExpanded === "function" ? options.getSelectedJobIdsExpanded : () => []

  const normalizeFolder =
    typeof folderManager.normalizeFolder === "function" ? folderManager.normalizeFolder : defaultNormalizeFolder
  const {
    rootOrder,
    jobOrder,
    folderOpen,
    ensureFolder,
    persistJobOrder,
    persistRootOrder,
    removeJobsFromRootOrder,
    ensureJobsInRootOrder,
  } = folderManager

  const dragTransfer = useDragTransfer({ normalizeFolderName: normalizeFolder })
  const { clearDragState, getDragPayload, onFolderDragStart, onDragStart, isDragging } = dragTransfer

  const getJobs = () => (Array.isArray(jobs?.value) ? jobs.value : [])

  const findJob = (jobId) => getJobs().find((job) => String(job?.id || "") === String(jobId || ""))

  const setFolderExpanded = (folderName) => {
    const name = normalizeFolder(folderName)
    if (!name || !folderOpen?.value) return
    folderOpen.value = { ...folderOpen.value, [name]: true }
  }

  const reorderRootTokens = (draggedToken, targetToken, options = {}) => {
    const ensuredTokens = normalizeList([...(rootOrder?.value || []), ...(options.ensureTokens || [])])
    const draggedIndex = ensuredTokens.indexOf(draggedToken)
    const targetIndex = ensuredTokens.indexOf(targetToken)
    if (draggedIndex < 0 || targetIndex < 0 || draggedToken === targetToken) return false

    const insertAfter = typeof options.insertAfter === "boolean" ? options.insertAfter : draggedIndex < targetIndex
    const next = reorderList(ensuredTokens, draggedToken, targetToken, insertAfter)
    if (!next) return false

    persistRootOrder(next)
    return true
  }

  async function moveJobsToFolder(jobIds, folderName) {
    const ids = normalizeJobIds(jobIds)
    if (!ids.length) return false

    const targetFolder = folderName ? ensureFolder(folderName) : ""
    await cron.setJobsFolder(ids, targetFolder)
    if (targetFolder) {
      setFolderExpanded(targetFolder)
      removeJobsFromRootOrder(ids)
    } else {
      ensureJobsInRootOrder(ids)
    }
    return true
  }

  function reorderFolderOnFolder(draggedFolderName, targetFolderName) {
    const draggedFolder = ensureFolder(draggedFolderName)
    const targetFolder = ensureFolder(targetFolderName)
    if (!draggedFolder || !targetFolder || draggedFolder === targetFolder) return false

    const draggedToken = tokenFor("folder", draggedFolder)
    const targetToken = tokenFor("folder", targetFolder)
    return reorderRootTokens(draggedToken, targetToken, {
      ensureTokens: [draggedToken, targetToken],
      insertAfter: false,
    })
  }

  function reorderFolderOnJob(draggedFolderName, targetJobId) {
    const targetId = String(targetJobId || "")
    if (!targetId) return false

    const targetJob = findJob(targetId)
    if (!targetJob || normalizeFolder(targetJob?.folder)) return false

    const draggedFolder = ensureFolder(draggedFolderName)
    if (!draggedFolder) return false

    const draggedToken = tokenFor("folder", draggedFolder)
    const targetToken = tokenFor("job", targetId)
    return reorderRootTokens(draggedToken, targetToken, {
      ensureTokens: [draggedToken, targetToken],
    })
  }

  async function moveJobsToTargetFolder(jobIds, targetJobId) {
    const targetJob = findJob(targetJobId)
    if (!targetJob) return false

    return moveJobsToFolder(jobIds, normalizeFolder(targetJob?.folder))
  }

  async function reorderSingleJob(draggedJobId, targetJobId) {
    const draggedId = String(draggedJobId || "")
    const targetId = String(targetJobId || "")
    if (!draggedId || !targetId || draggedId === targetId) return false

    const draggedJob = findJob(draggedId)
    const targetJob = findJob(targetId)
    if (!draggedJob || !targetJob) return false

    const draggedFolder = normalizeFolder(draggedJob?.folder)
    const targetFolder = normalizeFolder(targetJob?.folder)
    if (draggedFolder !== targetFolder) {
      await cron.setJobFolder(draggedId, targetFolder)
    }

    const currentJobOrder = normalizeList(jobOrder?.value)
    let insertAfter = (() => {
      const draggedIndex = currentJobOrder.indexOf(draggedId)
      const targetIndex = currentJobOrder.indexOf(targetId)
      return draggedIndex >= 0 && targetIndex >= 0 ? draggedIndex < targetIndex : true
    })()

    if (!targetFolder) {
      const rootTokens = normalizeList(rootOrder?.value)
      const draggedIndex = rootTokens.indexOf(tokenFor("job", draggedId))
      const targetIndex = rootTokens.indexOf(tokenFor("job", targetId))
      if (draggedIndex >= 0 && targetIndex >= 0) {
        insertAfter = draggedIndex < targetIndex
      }
    }

    const orderedJobIds = normalizeList([...currentJobOrder, draggedId, targetId])
    const nextJobOrder = reorderList(orderedJobIds, draggedId, targetId, insertAfter)
    if (!nextJobOrder) return false
    persistJobOrder(nextJobOrder)

    const draggedToken = tokenFor("job", draggedId)
    const targetToken = tokenFor("job", targetId)
    if (targetFolder) {
      const currentRootOrder = normalizeList(rootOrder?.value)
      if (currentRootOrder.includes(draggedToken)) {
        persistRootOrder(currentRootOrder.filter((token) => token !== draggedToken))
      }
    } else {
      reorderRootTokens(draggedToken, targetToken, {
        ensureTokens: [draggedToken, targetToken],
      })
    }

    return true
  }

  function buildDropPayload(event) {
    const payload = getDragPayload(event)
    if (payload.kind === "folder") {
      return {
        kind: "folder",
        folderName: normalizeFolder(payload.folderName),
        jobIds: [],
        primaryJobId: "",
      }
    }

    const jobIds = normalizeJobIds(payload.jobIds)
    return {
      kind: jobIds.length ? "jobs" : "",
      folderName: "",
      jobIds,
      primaryJobId: jobIds.includes(payload.primaryJobId) ? payload.primaryJobId : jobIds[0] || "",
    }
  }

  function finishDrop(handled) {
    if (handled) clearDragState()
    return handled
  }

  function onJobDragStart(event, jobId) {
    const id = String(jobId || "")
    if (!id) return

    const token = tokenFor("job", id)
    const selectedJobIds = isSelected(token) ? normalizeJobIds(getSelectedJobIdsExpanded()) : []
    onDragStart(event, id, selectedJobIds.length > 1 ? selectedJobIds : [id])
  }

  async function onDropToFolder(event, folderName) {
    const payload = buildDropPayload(event)
    if (payload.kind === "folder") {
      finishDrop(reorderFolderOnFolder(payload.folderName, folderName))
      return
    }
    if (!payload.jobIds.length) return

    finishDrop(await moveJobsToFolder(payload.jobIds, folderName))
  }

  async function onDropToUnfiled(event) {
    const payload = buildDropPayload(event)
    if (payload.kind !== "jobs" || !payload.jobIds.length) return

    finishDrop(await moveJobsToFolder(payload.jobIds, ""))
  }

  async function onDropToJob(event, targetJobId) {
    const payload = buildDropPayload(event)
    const targetId = String(targetJobId || "")
    if (!targetId) return

    if (payload.kind === "folder") {
      finishDrop(reorderFolderOnJob(payload.folderName, targetId))
      return
    }
    if (!payload.jobIds.length) return

    if (payload.jobIds.length > 1) {
      finishDrop(await moveJobsToTargetFolder(payload.jobIds, targetId))
      return
    }

    finishDrop(await reorderSingleJob(payload.primaryJobId || payload.jobIds[0], targetId))
  }

  return {
    clearDragState,
    isDragging,
    onFolderDragStart,
    onJobDragStart,
    onDropToFolder,
    onDropToJob,
    onDropToUnfiled,
  }
}
