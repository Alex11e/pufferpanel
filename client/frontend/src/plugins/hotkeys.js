import { install, uninstall, normalizeSequence } from '@github/hotkey'
import { ref } from 'vue'
import { normalizeShortcut, shortcutConflict } from './hotkeySettings.mjs'

const storageKey = 'pufferpanel.hotkeys'
const bindings = new Set()
const overrides = ref({})

function getGroup(i, route) {
  let r = route
  if (!r) r = i.$route
  const rootId = i._.root.uid
  if (i._.uid === rootId) return 'root'
  if ((i._.type || {}).name === 'RouterView') return r.name
  return getGroup(i._.parent.ctx, r)
}

function effective(group, key) {
  return overrides.value[group]?.[key] ?? key
}

function rebind(binding) {
  const keys = binding.keys.map(key => effective(binding.group, key)).filter(Boolean)
  if (keys.length) install(binding.el, keys.join(','))
}

function rebindAll() {
  for (const binding of bindings) uninstall(binding.el)
  for (const binding of bindings) rebind(binding)
}

function entries() {
  const active = Array.from(bindings).flatMap(binding => binding.keys.map(original => ({
    group: binding.group,
    original,
    effective: effective(binding.group, original)
  })))
  for (const [group, mappings] of Object.entries(overrides.value)) {
    for (const [original, shortcut] of Object.entries(mappings)) {
      if (!active.some(entry => entry.group === group && entry.original === original)) {
        active.push({ group, original, effective: shortcut })
      }
    }
  }
  return active
}

function save(group, original, value) {
  const replacement = value === '' ? '' : normalizeShortcut(value)
  if (replacement && shortcutConflict(entries(), group, original, replacement)) throw new Error('conflict')
  const updated = { ...overrides.value, [group]: { ...overrides.value[group], [original]: replacement } }
  localStorage.setItem(storageKey, JSON.stringify(updated))
  overrides.value = updated
  rebindAll()
}

function reset(group, original) {
  if (shortcutConflict(entries(), group, original, original)) throw new Error('conflict')
  const updated = { ...overrides.value, [group]: { ...overrides.value[group] } }
  delete updated[group][original]
  localStorage.setItem(storageKey, JSON.stringify(updated))
  overrides.value = updated
  rebindAll()
}

export default {
  install: (app) => {
    try {
      const stored = localStorage.getItem(storageKey)
      if (stored) {
        const parsed = JSON.parse(stored)
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('invalid settings')
        const loaded = {}
        for (const [group, mappings] of Object.entries(parsed)) {
          if (!mappings || typeof mappings !== 'object' || Array.isArray(mappings)) throw new Error('invalid settings')
          for (const value of Object.values(mappings)) {
            if (typeof value !== 'string' || (value && normalizeShortcut(value) !== value)) throw new Error('invalid shortcut')
          }
          loaded[group] = mappings
        }
        overrides.value = loaded
      }
    } catch (error) {
      console.error('Failed to load keyboard shortcuts', error)
    }
    app.provide('hotkeySettings', { effective, save, reset })
    app.provide('hotkeys', () => {
      const res = {}
      for (const binding of bindings) {
        if (!(binding.el.offsetWidth || binding.el.offsetHeight || binding.el.getClientRects().length)) continue
        if (!res[binding.group]) res[binding.group] = []
        res[binding.group].push(...binding.keys)
      }
      return res
    })
    app.directive('hotkey', {
      beforeMount(el, binding, vnode) {
        if (!binding.value) return
        const keys = Array.isArray(binding.value) ? binding.value : [binding.value]
        const item = { el, group: getGroup(binding.instance), keys }
        item.listener = event => {
          if (!(vnode.props || {}).onHotkey) return
          event.preventDefault()
          if (!(el.offsetWidth || el.offsetHeight || el.getClientRects().length)) return
          const pressed = event.detail.path.join(' ')
          const original = keys.find(key => {
            const mapped = effective(item.group, key)
            return mapped && normalizeSequence(mapped) === pressed
          })
          if (original) vnode.props.onHotkey(original)
        }
        el.addEventListener('hotkey-fire', item.listener)
        bindings.add(item)
        rebind(item)
      },
      beforeUnmount(el) {
        const item = Array.from(bindings).find(entry => entry.el === el)
        if (!item) return
        uninstall(el)
        el.removeEventListener('hotkey-fire', item.listener)
        bindings.delete(item)
      }
    })
  }
}
