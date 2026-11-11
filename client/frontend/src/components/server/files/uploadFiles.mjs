export async function uploadFiles(server, destination, files, folder, onState, onProgress, confirmOverwrite) {
  if (folder) {
    onState('create folders')
    const folders = new Set()
    for (const file of files) {
      const parts = file.webkitRelativePath.split('/')
      parts.pop()
      if (parts.length) folders.add([destination, ...parts].filter(Boolean).join('/'))
    }
    for (const path of folders) {
      const type = await server.fileExists(path)
      if (type === 'file') throw new Error(`Directory exists as a file: ${path}`)
      if (!type) await server.createFolder(path)
    }
  }
  onState('upload files')
  for (let i = 0; i < files.length; i++) {
    const file = files[i]
    const path = [destination, file.webkitRelativePath || file.name].filter(Boolean).join('/')
    onProgress(i, 0)
    const progress = event => onProgress(i, event.loaded)
    try {
      await server.uploadFile(path, file, progress, true)
    } catch (error) {
      if (error.code !== 'FILE_EXISTS') throw error
      if (!confirmOverwrite(path)) {
        const cancelled = new Error('Upload cancelled')
        cancelled.code = 'UPLOAD_CANCELLED'
        throw cancelled
      }
      await server.uploadFile(path, file, progress)
    }
    onProgress(i, file.size)
  }
}
