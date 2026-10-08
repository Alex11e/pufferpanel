<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const props = defineProps({ server: { type: Object, required: true } })
const { t } = useI18n()

const queryState = ref('loading')
const players = ref([])
const selectedPlayer = ref('')
const targetName = ref('')
const itemId = ref('minecraft:diamond')
const itemCount = ref(1)
const refreshing = ref(false)
const commandError = ref('')
const commandOutput = ref([])
const capturingOutput = ref(false)
const canSendCommands = computed(() => props.server.hasScope('server.console.send'))
const validTargetName = computed(() => /^[A-Za-z0-9_]{1,16}$/.test(targetName.value.trim()))
const validItemId = computed(() => /^[a-z0-9_.-]+:[a-z0-9_./-]+$/.test(itemId.value))
let refreshTimer = null
let outputTimer = null
let stopConsoleListener = null
const decoder = new TextDecoder('utf-8')

function queryFailure(failure) {
  return failure?.msg || failure?.message || t('servers.players.queryFailed')
}

async function refreshPlayers() {
  if (refreshing.value) return
  refreshing.value = true
  try {
    const supported = await props.server.canQuery({ onError: () => ({ unavailable: true }) })
    if (supported === null) {
      queryState.value = 'error'
      return
    }
    if (!supported) {
      queryState.value = 'unsupported'
      players.value = []
      selectedPlayer.value = ''
      return
    }

    const result = await props.server.getQuery({ onError: () => ({ data: { networkError: true } }) })
    if (result?.networkError) {
      queryState.value = 'error'
      return
    }
    queryState.value = 'ready'
    players.value = result?.minecraft?.players || []
    if (!players.value.includes(selectedPlayer.value)) selectedPlayer.value = players.value[0] || ''
  } catch (failure) {
    commandError.value = queryFailure(failure)
    queryState.value = 'error'
  } finally {
    refreshing.value = false
  }
}

function decodeConsole(event) {
  const encoded = Array.isArray(event?.logs) ? event.logs.join('') : event?.logs
  if (typeof encoded !== 'string' || encoded.length === 0) return ''
  try {
    const binary = atob(encoded)
    const bytes = Uint8Array.from(binary, character => character.charCodeAt(0))
    const ansiColorCode = new RegExp(`${String.fromCharCode(27)}\\[[0-9;]*m`, 'g')
    return decoder.decode(bytes).replace(ansiColorCode, '')
  } catch {
    return ''
  }
}

function onConsole(event) {
  if (!capturingOutput.value) return
  const output = decodeConsole(event).trim()
  if (!output) return
  commandOutput.value = [...commandOutput.value, output].slice(-10)
}

async function runCommand(command, capture = false) {
  if (!canSendCommands.value || !command) return
  commandError.value = ''
  commandOutput.value = []
  capturingOutput.value = capture
  if (outputTimer) clearTimeout(outputTimer)
  try {
    await props.server.sendCommand(command)
    if (capture) outputTimer = setTimeout(() => { capturingOutput.value = false }, 5000)
  } catch (failure) {
    capturingOutput.value = false
    commandError.value = queryFailure(failure)
  }
}

function kickPlayer() {
  if (!selectedPlayer.value) return
  runCommand(`kick ${selectedPlayer.value}`)
}

function inspectInventory() {
  if (!selectedPlayer.value) return
  runCommand(`data get entity ${selectedPlayer.value} Inventory`, true)
}

function clearInventory() {
  if (!selectedPlayer.value) return
  runCommand(`clear ${selectedPlayer.value}`)
}

function giveItem() {
  if (!selectedPlayer.value || !validItemId.value || !Number.isInteger(itemCount.value) || itemCount.value < 1 || itemCount.value > 64) return
  runCommand(`give ${selectedPlayer.value} ${itemId.value} ${itemCount.value}`)
}

function runPlayerCommand(action) {
  if (!validTargetName.value) return
  runCommand(`${action} ${targetName.value.trim()}`)
}

onMounted(() => {
  stopConsoleListener = props.server.on('console', onConsole)
  refreshPlayers()
  refreshTimer = setInterval(refreshPlayers, 15000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  if (outputTimer) clearTimeout(outputTimer)
  if (stopConsoleListener) stopConsoleListener()
})
</script>

<template>
  <section class="minecraft-players">
    <div class="section-heading">
      <h2 v-text="t('servers.players.title')" />
      <btn variant="icon" :tooltip="t('servers.players.refresh')" :disabled="refreshing" @click="refreshPlayers"><icon :name="refreshing ? 'loading' : 'reload'" :spin="refreshing" /></btn>
    </div>
    <loader v-if="queryState === 'loading'" />
    <p v-else-if="queryState === 'unsupported'" class="hint" v-text="t('servers.players.queryUnsupported')" />
    <p v-else-if="queryState === 'error'" class="error" role="alert" v-text="t('servers.players.queryFailed')" />
    <template v-else>
      <div class="online-summary">{{ t('servers.players.onlineCount', { count: players.length }) }}</div>
      <p v-if="players.length === 0" class="hint" v-text="t('servers.players.noneOnline')" />
      <div v-else class="player-list">
        <label>{{ t('servers.players.onlinePlayer') }}
          <select v-model="selectedPlayer"><option v-for="player in players" :key="player" :value="player">{{ player }}</option></select>
        </label>
        <div class="player-actions">
          <btn v-if="canSendCommands" @click="inspectInventory"><icon name="eye" />{{ t('servers.players.inspectInventory') }}</btn>
          <btn v-if="canSendCommands" @click="clearInventory"><icon name="remove" />{{ t('servers.players.clearInventory') }}</btn>
          <btn v-if="canSendCommands" color="error" @click="kickPlayer"><icon name="stop" />{{ t('servers.players.kick') }}</btn>
        </div>
        <form v-if="canSendCommands" class="give-form" @submit.prevent="giveItem">
          <label>{{ t('servers.players.itemId') }}<input v-model="itemId" required></label>
          <label>{{ t('servers.players.itemCount') }}<input v-model.number="itemCount" type="number" min="1" max="64" required></label>
          <btn color="primary" type="submit" :disabled="!validItemId"><icon name="plus" />{{ t('servers.players.giveItem') }}</btn>
        </form>
      </div>
    </template>

    <div v-if="canSendCommands" class="account-actions">
      <h3>{{ t('servers.players.accountActions') }}</h3>
      <label>{{ t('servers.players.playerName') }}<input v-model="targetName" maxlength="16" autocomplete="off"></label>
      <div class="player-actions">
        <btn :disabled="!validTargetName" @click="runPlayerCommand('whitelist add')"><icon name="plus" />{{ t('servers.players.whitelist') }}</btn>
        <btn :disabled="!validTargetName" @click="runPlayerCommand('whitelist remove')"><icon name="remove" />{{ t('servers.players.unwhitelist') }}</btn>
        <btn :disabled="!validTargetName" @click="runPlayerCommand('ban')"><icon name="stop" />{{ t('servers.players.ban') }}</btn>
        <btn :disabled="!validTargetName" @click="runPlayerCommand('pardon')"><icon name="restore" />{{ t('servers.players.unban') }}</btn>
      </div>
    </div>

    <p v-if="commandError" class="error" role="alert">{{ commandError }}</p>
    <pre v-if="commandOutput.length" class="command-output">{{ commandOutput.join('\n') }}</pre>
  </section>
</template>

<style scoped>
.minecraft-players { max-width: 760px; }
.section-heading, .player-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.section-heading { justify-content: space-between; }
.section-heading h2 { margin: 0; }
.online-summary { margin: 8px 0 12px; color: var(--color-text-secondary); }
.player-list, .account-actions { display: grid; gap: 12px; margin-top: 16px; }
.player-list label, .account-actions label, .give-form label { display: grid; gap: 5px; color: var(--color-text-secondary); }
select, input { min-width: 0; padding: 9px; border: 1px solid var(--color-background); border-radius: 5px; background: var(--color-background-secondary); color: var(--color-text); }
.give-form { display: grid; grid-template-columns: minmax(0, 1fr) 100px auto; align-items: end; gap: 8px; }
.hint { color: var(--color-text-secondary); }
.error { color: var(--color-error); }
.command-output { overflow: auto; max-height: 240px; padding: 12px; background: var(--color-background-secondary); white-space: pre-wrap; overflow-wrap: anywhere; }
@media (max-width: 600px) { .give-form { grid-template-columns: minmax(0, 1fr); } }
</style>
