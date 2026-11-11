import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { builtinManifest, builtinPresets } from './builtinThemes.mjs'

const manifest = JSON.parse(await readFile(new URL('./manifest.json', import.meta.url), 'utf8'))

test('every builtin theme has an isolated manifest and a valid default mode', () => {
  for (const [name, preset] of Object.entries(builtinPresets)) {
    const result = builtinManifest(name, manifest)
    const defaults = result.settings.mode.options.filter(option => option.default)
    assert.equal(defaults.length, 1)
    if (preset) {
      assert.equal(defaults[0].value, preset.mode)
      assert.equal(result.settings.color.default, preset.color)
    }
    result.settings.mode.options[0].value = 'changed'
    assert.notEqual(manifest.settings.mode.options[0].value, 'changed')
  }
  assert.equal(builtinManifest('unknown', manifest), null)
})
