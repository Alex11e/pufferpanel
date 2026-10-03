export const serverIconOptions = [
  'server',
  'web',
  'database',
  'robot',
  'minecraft',
  'gamepad-variant',
  'cloud'
]

const iconByType = {
  webhosting: 'web',
  dbhosting: 'database',
  discordbot: 'robot',
  hypervm: 'server',
  'minecraft-java': 'minecraft',
  'minecraft-bedrock': 'minecraft',
  ark: 'gamepad-variant',
  arma3: 'gamepad-variant',
  factorio: 'gamepad-variant',
  rust: 'gamepad-variant',
  terraria: 'gamepad-variant',
  valheim: 'gamepad-variant'
}

export function serverIconName(server) {
  const configured = server?.icon || ''
  if (serverIconOptions.includes(configured)) return configured
  return iconByType[configured] || iconByType[server?.type] || 'server'
}