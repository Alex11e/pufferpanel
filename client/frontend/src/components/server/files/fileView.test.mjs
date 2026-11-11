import { test } from 'node:test'
import assert from 'node:assert/strict'
import { sortFiles, visibleFiles } from './fileView.mjs'

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
