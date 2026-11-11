<script setup>
import { ref, inject, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/ui/Icon.vue'
import Btn from '@/components/ui/Btn.vue'
import TextField from '@/components/ui/TextField.vue'

import ConsoleWorker from '@/utils/consoleWorker.js?worker&inline'
let worker = new ConsoleWorker()
let lastElem = null

const { t } = useI18n()
const config = inject('config')
const panelName = config.branding.name

const command = ref('')
const console = ref(null)
const loadError = ref('')
const commandError = ref('')
const sending = ref(false)
let lastMessageTime = 0
let snapshotLoaded = false
let loading = false
let mounted = false

const props = defineProps({
  server: { type: Object, required: true }
})

let unbindEvent = null
let task = null
onMounted(() => {
  mounted = true
  worker.addEventListener("message", onWorkerMessage)
  if (props.server.hasScope('server.console')) {
    unbindEvent = props.server.on('console', onMessage)
    task = props.server.startTask(() => {
      if (!snapshotLoaded || props.server.needsPolling()) loadConsole()
    }, 5000)
  }
})

onUnmounted(() => {
  mounted = false
  if (unbindEvent) unbindEvent()
  if (task) props.server.stopTask(task)
  worker.removeEventListener('message', onWorkerMessage)
  worker.terminate()
  clearConsole()
})

async function loadConsole() {
  if (loading) return
  loading = true
  try {
    const result = await props.server.getConsole(snapshotLoaded ? lastMessageTime : 0)
    if (!mounted) return
    if (!snapshotLoaded) {
      worker.removeEventListener('message', onWorkerMessage)
      worker.terminate()
      worker = new ConsoleWorker()
      worker.addEventListener('message', onWorkerMessage)
      clearConsole()
    }
    snapshotLoaded = true
    loadError.value = ''
    onMessage(result)
  } catch {
    if (mounted) loadError.value = t('servers.ConsoleLoadFailed')
  } finally {
    loading = false
  }
}

function onMessage(e) {
  if (e.epoch > 0) lastMessageTime = e.epoch
  worker.postMessage({ ...e, panelName })
}

function onWorkerMessage(e) {
  if (!console.value) return
  const newElems = []
  e.data.map(update => {
    if (update.op === 'update' && lastElem) {
      lastElem.innerHTML = update.content
    } else {
      const el = document.createElement('div')
      el.innerHTML = update.content
      newElems.push(el)
      lastElem = el
    }
  })
  if (newElems.length + console.value.children.length > 1200) {
    let elems = Array.from(console.value.children).concat(newElems)
    elems = elems.slice(elems.length - 1000, elems.length)
    console.value.replaceChildren(elems)
  } else {
    console.value.append(...newElems)
  }
}

function clearConsole() {
  if (console.value) console.value.replaceChildren()
  lastElem = null
}

const history = ref([])
const historyIndex = ref(-1)
const temporaryCommand = ref('')

async function sendCommand() {
  const sentCommand = command.value.trim()
  if (!sentCommand || sending.value) return
  commandError.value = ''
  sending.value = true
  try {
    await props.server.sendCommand(sentCommand)
  } catch {
    commandError.value = t('servers.ConsoleSendFailed')
    return
  } finally {
    sending.value = false
  }

  if (historyIndex.value !== -1) {
    history.value.splice(historyIndex.value, 1)
  }

  if (history.value.length === 0 || history.value[history.value.length - 1] !== sentCommand) {
    history.value.push(sentCommand)
  }

  historyIndex.value = -1
  temporaryCommand.value = ''

  if (history.value.length > 100) {
    history.value.splice(0, 1)
  }

  if (command.value.trim() === sentCommand) command.value = ''
}

function clearCommandHistory() {
  history.value = []
  historyIndex.value = -1
  temporaryCommand.value = ''
}

function previousCommand() {
  if (historyIndex.value === -1 && history.value.length > 0) {
    historyIndex.value = history.value.length - 1
    temporaryCommand.value = command.value
  } else if (historyIndex.value > 0) {
    historyIndex.value--
  } else {
    return
  }

  command.value = history.value[historyIndex.value]
}

function nextCommand() {
  if (historyIndex.value === -1) {
    return
  }

  historyIndex.value++

  if (historyIndex.value >= history.value.length) {
    historyIndex.value = -1
    command.value = temporaryCommand.value
  } else {
    command.value = history.value[historyIndex.value]
  }
}

</script>

<template>
  <div>
    <h2 v-text="t('servers.Console')" />
    <div v-if="loadError" class="alert error" role="alert">
      {{ loadError }}
      <btn @click="loadConsole">{{ t('servers.Retry') }}</btn>
    </div>
    <div v-if="commandError" class="alert error" role="alert">{{ commandError }}</div>
    <icon v-if="server.hasScope('server.console')" v-hotkey="'c x'" name="clear-console" @click="clearConsole()" />
    <div v-if="server.hasScope('server.console')" dir="ltr" class="console-wrapper">
      <div ref="console" class="console" />
    </div>
    <div v-if="server.hasScope('server.console.send')" dir="ltr" class="command">
      <text-field
        v-model="command"
        v-hotkey="'c c'"
        :label="t('servers.Command')"
        @keyup.enter="sendCommand()"
        @keydown.up.prevent="previousCommand()"
        @keydown.down.prevent="nextCommand()"
      />
      <icon name="send" @click="sendCommand()" />
      <btn variant="icon" :tooltip="t('servers.ClearCommandHistory')" :disabled="history.length === 0" @click="clearCommandHistory()"><icon name="remove" /></btn>
    </div>
  </div>
</template>
