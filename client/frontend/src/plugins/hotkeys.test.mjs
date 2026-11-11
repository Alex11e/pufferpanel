import { test } from 'node:test'
import assert from 'node:assert/strict'
import hotkeys from './hotkeys.js'

test('rebinding preserves action identity and unregisters on unmount', () => {
  class TestNode extends EventTarget {}
  class TestElement extends TestNode {
    nodeName = 'DIV'
    offsetWidth = 1
    offsetHeight = 1
    getClientRects() { return [1] }
    getAttribute() { return null }
    click() { this.clicked = true }
  }
  const document = new TestNode()
  globalThis.Node = TestNode
  globalThis.HTMLElement = TestElement
  globalThis.document = document
  globalThis.window = { setTimeout, clearTimeout, navigator: { platform: 'Win32' } }
  const stored = new Map()
  globalThis.localStorage = {
    getItem: key => stored.get(key) ?? null,
    setItem: (key, value) => stored.set(key, value)
  }
  const providers = {}
  const directives = {}
  hotkeys.install({
    provide: (key, value) => { providers[key] = value },
    directive: (key, value) => { directives[key] = value }
  })
  const el = new TestElement()
  const calls = []
  const instance = { _: { uid: 1, root: { uid: 1 } } }
  directives.hotkey.beforeMount(el, { value: ['r r', 'r s'], instance }, {
    props: { onHotkey: key => calls.push(key) }
  })
  function press(key, ctrlKey = false, altKey = false) {
    const event = new Event('keydown', { cancelable: true })
    Object.assign(event, { key, ctrlKey, altKey, metaKey: false, shiftKey: false })
    document.dispatchEvent(event)
  }
  press('r')
  press('s')
  assert.deepEqual(calls, ['r s'])

  providers.hotkeySettings.save('root', 'r s', 'Control+Alt+x')
  press('x', true, true)
  assert.deepEqual(calls, ['r s', 'r s'])
  press('r')
  press('s')
  assert.equal(calls.length, 2)

  providers.hotkeySettings.reset('root', 'r s')
  press('r')
  press('s')
  assert.equal(calls.length, 3)

  providers.hotkeySettings.save('root', 'r r', '')
  press('r')
  press('r')
  assert.equal(calls.length, 3)
  providers.hotkeySettings.reset('root', 'r r')
  press('r')
  press('r')
  assert.deepEqual(calls, ['r s', 'r s', 'r s', 'r r'])

  const button = new TestElement()
  directives.hotkey.beforeMount(button, { value: 'a', instance }, { props: {} })
  providers.hotkeySettings.save('root', 'a', 'Control+Alt+b')
  assert.throws(() => providers.hotkeySettings.save('root', 'r s', 'Control+Alt+b'), /conflict/)
  press('b', true, true)
  assert.equal(button.clicked, true)
  directives.hotkey.beforeUnmount(button)
  directives.hotkey.beforeUnmount(el)
  press('r')
  press('s')
  assert.equal(calls.length, 4)
})
