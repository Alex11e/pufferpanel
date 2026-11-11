import { test } from 'node:test'
import assert from 'node:assert/strict'
import { normalizeShortcut, shortcutConflict } from './hotkeySettings.mjs'

test('shortcuts normalize modifiers and reject unsafe or malformed input', () => {
  assert.equal(normalizeShortcut('ctrl+alt+shift+F   a'), 'Control+Alt+Shift+F a')
  for (const value of ['', 'Control+', 'Ctrl+Ctrl+a', 'Control+x,Control+y', 'a b c d', 'Escape', 'Control+k']) {
    assert.throws(() => normalizeShortcut(value))
  }
})

test('conflicts account for global and current route but not other routes', () => {
  const entries = [
    { group: 'root', original: 'g s', effective: 'g s' },
    { group: 'ServerView', original: 't c', effective: 't c' },
    { group: 'Self', original: 't s', effective: 't s' }
  ]
  assert.equal(shortcutConflict(entries, 'ServerView', 't c', 'g s'), true)
  assert.equal(shortcutConflict(entries, 'ServerView', 't c', 't s'), false)
  assert.equal(shortcutConflict(entries, 'ServerView', 't c', 't c'), false)
  assert.equal(shortcutConflict(entries, 'root', 'g s', 't c'), true)
  assert.equal(shortcutConflict(entries, 'ServerView', 't c', 'g'), true)
})
