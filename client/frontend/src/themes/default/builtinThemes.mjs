export const builtinPresets = {
  PufferPanel: null,
  'PufferPanel Ocean': { mode: 'mode-dark mode-ocean', color: '#58b6ff' },
  'PufferPanel Midnight': { mode: 'mode-dark mode-midnight', color: '#b799ff' },
  'PufferPanel Forest': { mode: 'mode-dark mode-forest', color: '#6ed5a4' },
  'PufferPanel Daylight': { mode: 'mode-light mode-daylight', color: '#346de4' }
}

export function builtinManifest(name, base) {
  if (!Object.hasOwn(builtinPresets, name)) return null
  const manifest = JSON.parse(JSON.stringify(base))
  const preset = builtinPresets[name]
  if (preset) {
    for (const option of manifest.settings.mode.options) option.default = option.value === preset.mode
    manifest.settings.color.default = preset.color
  }
  return manifest
}
