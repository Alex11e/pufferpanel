export function normalizeShortcut(value) {
  const steps = value.trim().split(/\s+/)
  if (!value.trim() || steps.length > 3) throw new Error('invalid')
  const result = steps.map(step => {
    const parts = step.split('+')
    const key = parts.pop()
    const modifiers = parts.map(part => ({ ctrl: 'Control', control: 'Control', alt: 'Alt', shift: 'Shift', meta: 'Meta' })[part.toLowerCase()])
    if (modifiers.some(part => !part) || new Set(modifiers).size !== modifiers.length || !/^[a-z0-9]$/i.test(key)) {
      throw new Error('invalid')
    }
    const character = modifiers.includes('Shift') ? key.toUpperCase() : key.toLowerCase()
    return [...['Control', 'Alt', 'Meta', 'Shift'].filter(part => modifiers.includes(part)), character].join('+')
  }).join(' ')
  if (!result.startsWith('Control+Alt+')) throw new Error('invalid')
  return result
}

export function shortcutConflict(entries, group, original, proposed) {
  return entries.some(entry =>
    (entry.group === group || entry.group === 'root' || group === 'root') &&
    !(entry.group === group && entry.original === original) &&
    entry.effective && (
      entry.effective.toLowerCase() === proposed.toLowerCase() ||
      entry.effective.toLowerCase().startsWith(proposed.toLowerCase() + ' ') ||
      proposed.toLowerCase().startsWith(entry.effective.toLowerCase() + ' ')
    )
  )
}
