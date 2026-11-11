export function sortFiles(a, b) {
  if (a.isFile !== b.isFile) return a.isFile ? 1 : -1
  return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
}

export function validItemName(name) {
  return typeof name === 'string' && !!name.trim() && name !== '.' && name !== '..' &&
    !/[\\/]/.test(name) && !Array.from(name).some(character => character.charCodeAt(0) < 32 || character.charCodeAt(0) === 127)
}

export function visibleFiles(files, search) {
  const query = search.trim().toLocaleLowerCase()
  return files.filter(file => file.name === '..' || file.name.toLocaleLowerCase().includes(query))
}

export function selectedPaths(directory, files) {
  return files.filter(file => file.isSelected && file.name !== '..')
    .map(file => `${directory}/${file.name}`)
}

export function retainSelection(entries, previous) {
  const selected = new Set(previous.filter(file => file.isSelected && file.name !== '..').map(file => file.name))
  return entries.map(file => ({ ...file, isSelected: file.name !== '..' && selected.has(file.name) }))
}

export async function deleteFiles(server, paths) {
  const results = await Promise.allSettled(paths.map(path => server.deleteFile(path)))
  return results.flatMap((result, index) => result.status === 'rejected' ? [{ path: paths[index], error: result.reason }] : [])
}
