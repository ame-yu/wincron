export function normalizeList(list) {
  const seen = new Set()
  const out = []
  for (const value of Array.isArray(list) ? list : []) {
    const normalized = String(value || "")
    if (normalized && !seen.has(normalized)) {
      seen.add(normalized)
      out.push(normalized)
    }
  }
  return out
}

export function normalizeJobIds(ids) {
  return normalizeList(ids)
}

export function tokenFor(type, value) {
  const normalized = String(value || "").trim()
  return normalized ? `${type}:${normalized}` : ""
}

export function getSelectedFolderNames(selectedTokens, normalizeFolder = (value) => String(value || "").trim()) {
  return normalizeList(selectedTokens)
    .filter((token) => token.startsWith("folder:"))
    .map((token) => normalizeFolder(token.slice(7)))
    .filter(Boolean)
}

export function getSelectedDirectJobIds(selectedTokens) {
  return normalizeJobIds(
    normalizeList(selectedTokens)
      .filter((token) => token.startsWith("job:"))
      .map((token) => token.slice(4)),
  )
}

export function getSelectedJobIdsExpanded(selectedTokens, getFolderJobIds, normalizeFolder = (value) => String(value || "").trim()) {
  const resolveFolderJobIds = typeof getFolderJobIds === "function" ? getFolderJobIds : () => []
  const out = new Set(getSelectedDirectJobIds(selectedTokens))
  for (const folderName of getSelectedFolderNames(selectedTokens, normalizeFolder)) {
    for (const id of normalizeJobIds(resolveFolderJobIds(folderName))) {
      out.add(id)
    }
  }
  return [...out]
}

export function getLastSelectedJobId(selectedTokens) {
  const tokens = normalizeList(selectedTokens)
  for (let i = tokens.length - 1; i >= 0; i -= 1) {
    const token = tokens[i]
    if (token.startsWith("job:")) {
      return token.slice(4)
    }
  }
  return ""
}
