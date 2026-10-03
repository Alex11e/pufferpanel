<script setup>
import { ref, inject, onMounted, onUnmounted } from 'vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const props = defineProps({
  server: { type: Object, required: true }
})

const events = inject('events')
const status = ref(null)
const vars = ref(null)
const busy = ref(false)
const refreshing = ref(false)
const statusError = ref('')
const dataError = ref('')
const actionError = ref('')
let timer = null

function errorMessage(error) {
  return error?.msg || error?.message || 'Ismeretlen hiba történt.'
}

async function refresh() {
  if (refreshing.value) return
  refreshing.value = true
  try {
    status.value = await props.server.getStatus()
    statusError.value = ''
  } catch (error) {
    status.value = 'unknown'
    statusError.value = errorMessage(error)
  } finally {
    refreshing.value = false
  }
}

async function power(action) {
  busy.value = true
  actionError.value = ''
  try {
    await props.server[action]()
    await refresh()
  } catch (error) {
    actionError.value = errorMessage(error)
  } finally {
    busy.value = false
  }
}

function confirmKill() {
  events.emit('confirm', 'Kényszerített leállítás? A vendég rendszer nem áll le rendesen.', {
    text: 'Kill', icon: 'stop', color: 'error', action: () => power('kill')
  })
}

function value(key, fallback = '') {
  const entry = vars.value?.[key]
  return entry && entry.value !== undefined && entry.value !== '' ? entry.value : fallback
}

function allocationPort(purpose) {
  return props.server.allocations?.find(allocation => allocation.purpose === purpose)?.port
}

function noVncPort() {
  return Number(props.server.port || value('port', 0))
}

function vncPort() {
  return Number(allocationPort('vnc') || (5900 + Number(value('vnc_display', 1))))
}

function forwardedPorts() {
  return (props.server.allocations || []).filter(allocation => allocation.purpose === 'forward' && allocation.targetPort)
}

onMounted(() => {
  timer = setInterval(refresh, 10000)
  refresh()
  if (props.server.hasScope('server.data.view')) {
    props.server.getData()
      .then(data => { vars.value = data.data || data })
      .catch(error => { dataError.value = errorMessage(error) })
  }
})

onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="vps-overview">
    <div class="title-row">
      <h2>VPS</h2>
      <btn variant="icon" tooltip="Állapot frissítése" :disabled="refreshing" @click="refresh"><icon name="reload" /></btn>
    </div>
    <loader v-if="status === null" small />
    <template v-else>
      <p>Állapot: <strong :class="['status', status]">{{ { online: 'Fut', offline: 'Leállítva', installing: 'Telepítés', unknown: 'Ismeretlen' }[status] || status }}</strong></p>
      <p v-if="statusError" class="error" role="alert">{{ statusError }}</p>
      <p v-if="actionError" class="error" role="alert">{{ actionError }}</p>
      <div class="actions">
        <btn v-if="status === 'offline' && server.hasScope('server.start')" color="primary" :disabled="busy" @click="power('start')"><icon name="play" /> Indítás</btn>
        <template v-if="status === 'online'">
          <btn v-if="server.hasScope('server.start')" :disabled="busy" @click="power('restart')"><icon name="reload" /> Újraindítás</btn>
          <btn v-if="server.hasScope('server.stop')" :disabled="busy" @click="power('stop')"><icon name="stop" /> Leállítás (ACPI)</btn>
          <btn v-if="server.hasScope('server.stop')" color="error" :disabled="busy" @click="confirmKill"><icon name="stop" /> Kill</btn>
        </template>
      </div>
    </template>

    <p v-if="dataError" class="error" role="alert">A VPS beállításai nem tölthetők be: {{ dataError }}</p>
    <section v-if="vars" class="connection">
      <h3>VNC kapcsolat</h3>
      <p v-if="noVncPort() > 0">noVNC port: <a :href="`http://${server.node?.publicHost}:${noVncPort()}/vnc.html?autoconnect=true`" target="_blank" rel="noopener noreferrer">{{ server.node?.publicHost }}:{{ noVncPort() }}</a></p>
      <p>VNC port: <code>{{ server.node?.publicHost }}:{{ vncPort() }}</code></p>
      <h3>Automatikusan kiosztott portok</h3>
      <ul>
        <li v-for="allocation in forwardedPorts()" :key="allocation.id">
          {{ allocation.protocols.toUpperCase() }} · {{ server.node?.publicHost }}:{{ allocation.port }} → vendég {{ allocation.targetPort }}
        </li>
        <li v-if="!forwardedPorts().length" class="hint">Nincs további vendégport továbbítás.</li>
      </ul>
      <p class="hint">Használj VNC klienst. A jelszó a Beállítások fülön módosítható, módosítás után indítsd újra a VPS-t.</p>
      <h3>Konfiguráció</h3>
      <ul>
        <li>Memória: {{ value('ram', '-') }} MB</li>
        <li>Lemez: {{ value('disk_file', '-') }} ({{ value('disk_size', '-') }} GB létrehozáskor)</li>
        <li>Boot ISO: {{ value('iso_file', 'nincs') }}</li>
        <li>UEFI: {{ value('use_uefi') === true || value('use_uefi') === 'true' ? 'igen' : 'nem' }} · KVM: {{ value('use_kvm') === true || value('use_kvm') === 'true' ? 'igen' : 'nem' }}</li>
      </ul>
      <p class="hint">ISO-t a Fájlok fülön tölthetsz fel, majd a Beállítások fülön add meg a nevét.</p>
    </section>
  </div>
</template>

<style scoped lang="scss">
.actions { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 16px; }
.title-row { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.title-row h2 { margin: 0; }
.error { color: var(--color-error); }
.status.online { color: var(--color-success); }
.status.installing { color: var(--color-warning); }
.connection { margin-top: 16px; padding: 16px; border-radius: 8px; background: var(--color-background-secondary); border: 1px solid var(--color-background); }
.hint { color: var(--color-text-secondary); }
ul { padding-left: 1.2em; }
</style>
