import { test } from 'node:test'
import assert from 'node:assert/strict'
import { uploadFiles } from './uploadFiles.mjs'

test('uploads folders and files to the captured destination with progress', async () => {
  const calls = []
  const server = {
    fileExists: async path => { calls.push(['exists', path]); return false },
    createFolder: async path => { calls.push(['mkdir', path]) },
    uploadFile: async (path, file, progress, createOnly) => { calls.push(['upload', path, createOnly]); progress({ loaded: 2 }) }
  }
  const files = [
    { name: 'one.txt', webkitRelativePath: 'pack/deep/one.txt', size: 3 },
    { name: 'two.txt', webkitRelativePath: 'pack/deep/two.txt', size: 4 }
  ]
  const states = []
  const progress = []
  await uploadFiles(server, 'target', files, true, state => states.push(state), (index, loaded) => progress.push([index, loaded]))
  assert.deepEqual(states, ['create folders', 'upload files'])
  assert.deepEqual(calls, [
    ['exists', 'target/pack/deep'], ['mkdir', 'target/pack/deep'],
    ['upload', 'target/pack/deep/one.txt', true], ['upload', 'target/pack/deep/two.txt', true]
  ])
  assert.deepEqual(progress, [[0, 0], [0, 2], [0, 3], [1, 0], [1, 2], [1, 4]])
})

test('reports folder conflicts and stops when an upload fails', async () => {
  const calls = []
  const files = [{ name: 'one', webkitRelativePath: 'pack/one', size: 1 }, { name: 'two', webkitRelativePath: '', size: 1 }]
  const server = {
    fileExists: async () => 'file',
    uploadFile: async path => { calls.push(path); throw new Error('upload failed') }
  }
  await assert.rejects(uploadFiles(server, '', files, true, () => {}, () => {}), /Directory exists as a file/)
  assert.deepEqual(calls, [])
  await assert.rejects(uploadFiles(server, 'target', files, false, () => {}, () => {}), /upload failed/)
  assert.deepEqual(calls, ['target/pack/one'])
})

test('existing files require explicit confirmation before replacement', async () => {
  const calls = []
  const server = {
    uploadFile: async (path, file, progress, createOnly) => {
      calls.push([path, createOnly])
      if (createOnly) throw Object.assign(new Error('exists'), { code: 'FILE_EXISTS' })
    }
  }
  const files = [{ name: 'file.txt', webkitRelativePath: '', size: 2 }]
  const states = () => {}
  const progress = () => {}
  await assert.rejects(uploadFiles(server, 'target', files, false, states, progress, () => false), { code: 'UPLOAD_CANCELLED' })
  assert.deepEqual(calls, [['target/file.txt', true]])
  calls.length = 0
  await uploadFiles(server, 'target', files, false, states, progress, path => path === 'target/file.txt')
  assert.deepEqual(calls, [['target/file.txt', true], ['target/file.txt', undefined]])
})
