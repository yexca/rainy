/** Restore an occurrence in the saved order, keeping its position after missing files are removed. */
export function restoreQueue<T extends { id: string; missing?: boolean }>(
  ids: readonly string[],
  tracks: readonly T[],
  currentId: string,
  currentIndex: number | undefined,
  position: number,
): { queue: T[]; index: number; position: number } {
  const savedIndex = currentIndex !== undefined && Number.isInteger(currentIndex)
    && currentIndex >= 0 && ids[currentIndex] === currentId
    ? currentIndex
    : ids.indexOf(currentId)
  const byId = new Map(tracks.filter((track) => !track.missing).map((track) => [track.id, track]))
  const queue: T[] = []
  let index = -1
  for (let i = 0; i < ids.length; i++) {
    const track = byId.get(ids[i])
    if (!track) continue
    if (i === savedIndex) index = queue.length
    // Each occurrence has its own object identity for queue editing and shuffle.
    queue.push({ ...track })
  }
  return {
    queue,
    index: queue.length === 0 ? -1 : Math.max(0, index),
    position: index >= 0 && Number.isFinite(position) ? Math.max(0, position) : 0,
  }
}
