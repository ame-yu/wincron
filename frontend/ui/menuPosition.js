export function getMenuPosition(e, { menuWidth = 220, menuHeight = 200, padding = 8 } = {}) {
  const x = Number(e?.clientX) || 0
  const y = Number(e?.clientY) || 0

  const maxX = Math.max(padding, window.innerWidth - menuWidth - padding)
  const maxY = Math.max(padding, window.innerHeight - menuHeight - padding)

  return {
    x: Math.min(Math.max(padding, x), maxX),
    y: Math.min(Math.max(padding, y), maxY),
  }
}
