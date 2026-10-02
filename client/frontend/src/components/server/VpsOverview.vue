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
let timer = null

async function refresh() {
  try {
    status.value = await props.server.getStatus()
  } catch {
    status.value = null
  }
}

async function power(action) {
  busy.value = true
  try {
    await props.server[action]()
    await refresh()
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

onMounted(async () => {
  await refresh()
  timer = setInterval(refresh, 10000)
  if (props.server.hasScope('server.data.view')) {
    const data = await props.server.getData()
    vars.value = data.data || data
  }
})

onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="vps-overview">
    <h2>VPS</h2>
    <loader v-if="status === null" small />
    <template v-else>
      <p>Állapot: <strong :class="['status', status]">{{ { online: 'Fut', offline: 'Leállítva', installing: 'Telepítés' }[status] || status }}</strong></p>
      <div class="actions">
        <btn v-if="status === 'offline' && server.hasScope('server.start')" color="primary" :disabled="busy" @click="power('start')"><icon name="play" /> Indítás</btn>
        <template v-if="status === 'online'">
          <btn v-if="server.hasScope('server.start')" :disabled="busy" @click="power('restart')"><icon name="reload" /> Újraindítás</btn>
          <btn v-if="server.hasScope('server.stop')" :disabled="busy" @click="power('stop')"><icon name="stop" /> Leállítás (ACPI)</btn>
          <btn v-if="server.hasScope('server.stop')" color="error" :disabled="busy" @click="confirmKill"><icon name="stop" /> Kill</btn>
        </template>
      </div>
    </template>

    <section v-if="vars" class="connection">
      <h3>VNC kapcsolat</h3>
      <p v-if="Number(value('port', 0)) > 0">Böngészős konzol: <a :href="`http://${server.node?.publicHost}:${value('port')}/vnc.html?autoconnect=true`" target="_blank" rel="noopener noreferrer">http://{{ server.node?.publicHost }}:{{ value('port') }}</a></p>
      <p>Cím: <code>{{ server.node?.publicHost }}:{{ 5900 + Number(value('vnc_display', 1)) }}</code></p>
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
.status.online { color: var(--color-success); }
.status.installing { color: var(--color-warning); }
.connection { margin-top: 16px; padding: 16px; border-radius: 8px; background: var(--color-background-secondary); border: 1px solid var(--color-background); }
.hint { color: var(--color-text-secondary); }
ul { padding-left: 1.2em; }
</style>
