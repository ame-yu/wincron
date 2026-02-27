export function formatDateTime(raw) {
  const s = String(raw || "")
  if (!s) {
    return ""
  }
  const ms = Date.parse(s)
  if (!Number.isFinite(ms)) {
    return s
  }
  return new Date(ms).toLocaleString()
}
