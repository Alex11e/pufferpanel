import { test } from 'node:test'
import assert from 'node:assert/strict'
import { deleteFiles, retainSelection, selectedPaths, sortFiles, validItemName, visibleFiles } from './fileView.mjs'

test('folders sort before files and names sort case-insensitively', () => {
  const entries = [
    { name: 'z.txt', isFile: true },
    { name: 'b', isFile: false },
    { name: 'Alpha', isFile: false },
    { name: 'a.txt', isFile: true }
  ]
  assert.deepEqual(entries.sort(sortFiles).map(entry => entry.name), ['Alpha', 'b', 'a.txt', 'z.txt'])
  assert.equal(sortFiles(entries[0], entries[0]), 0)
})

test('folder filter is case-insensitive and keeps the parent navigation entry', () => {
  const entries = [{ name: '..' }, { name: 'Config' }, { name: 'server.cfg' }]
  assert.deepEqual(visibleFiles(entries, ' CONFIG ').map(entry => entry.name), ['..', 'Config'])
  assert.deepEqual(visibleFiles(entries, '').map(entry => entry.name), ['..', 'Config', 'server.cfg'])
})

test('selection snapshots preserve exact paths and exclude parent navigation', () => {
  const files = [{ name: 'one.txt', isSelected: true }, { name: '..', isSelected: true }, { name: 'two', isSelected: false }]
  const paths = selectedPaths('data', files)
  files[0].isSelected = false
  assert.deepEqual(paths, ['data/one.txt'])
  assert.deepEqual(retainSelection([{ name: '..' }, { name: 'one.txt' }, { name: 'new' }], [
    { name: '..', isSelected: true }, { name: 'one.txt', isSelected: true }
  ]).map(file => file.isSelected), [false, true, false])
})

test('new names cannot traverse folders or contain control characters', () => {
  for (const name of ['', ' ', '.', '..', '../server', 'sub/file', 'sub\\file', 'bad\u0000name', 'bad\nname']) {
    assert.equal(validItemName(name), false, name)
  }
  for (const name of ['server.properties', '.env', 'szerver fájl', 'name..']) {
    assert.equal(validItemName(name), true, name)
  }
})

test('batch deletion waits for every request and reports each failure', async () => {
  const paths = ['/one', '/two', '/three']
  const completed = []
  const failed = await deleteFiles({
    deleteFile: async path => {
      await new Promise(resolve => setTimeout(resolve, path === '/three' ? 10 : 1))
      completed.push(path)
      if (path === '/two') throw new Error('denied')
    }
  }, paths)
  assert.deepEqual(completed.sort(), [...paths].sort())
  assert.equal(failed.length, 1)
  assert.equal(failed[0].path, '/two')
  assert.match(failed[0].error.message, /denied/)
})
