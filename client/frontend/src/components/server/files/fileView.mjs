export function sortFiles(a, b) {
  if (a.isFile !== b.isFile) return a.isFile ? 1 : -1
  return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
}

export function visibleFiles(files, search) {
  const query = search.trim().toLocaleLowerCase()
  return files.filter(file => file.name === '..' || file.name.toLocaleLowerCase().includes(query))
}
