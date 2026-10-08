<script setup>
import { computed, inject, ref } from 'vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import Overlay from '@/components/ui/Overlay.vue'
import TextField from '@/components/ui/TextField.vue'

const props = defineProps({ server: { type: Object, required: true } })
const api = inject('api')
const events = inject('events')
const opened = ref(false)
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const state = ref(null)
const loginUrl = ref('')
const secret = ref('')

const stateName = computed(() => state.value?.state || 'unavailable')
const stateData = computed(() => state.value?.data || {})
const tunnels = computed(() => stateData.value.tunnels || [])
const pendingTunnels = computed(() => stateData.value.pending_tunnels || [])
const stateLabel = computed(() => ({
  waiting_for_secret: 'Nincs fiókhoz kapcsolva',
  has_invalid_secret: 'Érvénytelen agent secret',
  disabled_over_limit: 'Playit account limit elérve',
  starting: 'Agent indul',
  running: 'Fut',
  stopping: 'Leáll',
  error: 'Agent hiba',
  unavailable: 'Nem érhető el'
})[stateName.value] || stateName.value)

async function refresh() {
  opened.value = true
  loading.value = true
  error.value = ''
  notice.value = ''
  try {
    const response = await api.get(`/api/servers/${props.server.id}/playit`)
    state.value = response.data
    loginUrl.value = response.data?.data?.login_link || ''
    if (stateName.value === 'waiting_for_secret' && !loginUrl.value) {
      try {
        const loginResponse = await api.get(`/api/servers/${props.server.id}/playit/login-url`)
        loginUrl.value = loginResponse.data?.data?.login_url || ''
      } catch {
        loginUrl.value = ''
      }
    }
  } catch (failure) {
    state.value = null
    error.value = failure?.msg || failure?.message || 'A node Playit agentje nem érhető el.'
  } finally {
    loading.value = false
  }
}

async function provisionSecret() {
  if (!secret.value.trim()) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    await api.put(`/api/servers/${props.server.id}/playit/secret`, { secret: secret.value })
    secret.value = ''
    notice.value = 'A secret átadva az agentnek. Frissítsd az állapotot pár másodperc múlva.'
    await refresh()
  } catch (failure) {
    error.value = failure?.msg || failure?.message || 'A Playit setup nem sikerült.'
  } finally {
    busy.value = false
  }
}

function stopAgent() {
  events.emit('confirm', 'Leállítod a Playit agentet ezen a node-on? Ez az összes ezen a node-on futó tunnelre hat.', {
    text: 'Agent leállítása', icon: 'stop', color: 'error', action: async () => {
      busy.value = true
      error.value = ''
      try {
        await api.post(`/api/servers/${props.server.id}/playit/stop`, {})
        notice.value = 'A leállítási kérés elküldve.'
        state.value = { state: 'stopping' }
      } catch (failure) {
        error.value = failure?.msg || failure?.message || 'Az agent leállítása nem sikerült.'
      } finally {
        busy.value = false
      }
    }
  })
}
</script>

<template>
  <section v-if="api.auth.hasScope('admin')" class="playit-agent">
    <div class="heading">
      <div><h2><icon name="network" /> Playit agent</h2><p>Node: {{ server.node?.name || server.node?.publicHost }} · ez az agent a node összes serverével közös.</p></div>
      <btn @click="refresh"><icon name="settings" /> Playit kezelése</btn>
    </div>
    <p class="hint">A Playit agentnek ezen a node-on kell futnia. Linuxon ellenőrizd, hogy a PufferPanel szolgáltatás felhasználója hozzáfér a <code>/run/playit/playitd.sock</code> sockethez.</p>
    <overlay v-model="opened" title="Playit agent" closable class="playit-overlay">
      <loader v-if="loading" />
      <template v-else>
        <p v-if="error" class="error" role="alert">{{ error }}</p>
        <p v-if="notice" class="notice" role="status">{{ notice }}</p>
        <p>Állapot: <strong :class="['status', stateName]">{{ stateLabel }}</strong></p>
        <p v-if="stateData.agent_id">Agent ID: <code>{{ stateData.agent_id }}</code></p>
        <p v-if="stateData.version">Verzió: {{ stateData.version }}</p>
        <p v-if="stateData.login_link"><a :href="stateData.login_link" target="_blank" rel="noopener noreferrer">Playit account összekapcsolása</a></p>
        <p v-else-if="loginUrl"><a :href="loginUrl" target="_blank" rel="noopener noreferrer">Playit account megnyitása</a></p>
        <div v-if="stateName === 'waiting_for_secret'" class="setup">
          <text-field v-model="secret" type="password" autocomplete="new-password" label="Playit agent secret" />
          <btn color="primary" :disabled="busy || !secret.trim()" @click="provisionSecret"><icon name="key" /> Secret beállítása</btn>
        </div>
        <section class="tunnels">
          <h3>Aktív tunnelök</h3>
          <p v-if="!tunnels.length && !pendingTunnels.length" class="muted">Az agent nem jelentett tunnelöket.</p>
          <div v-for="(tunnel, index) in tunnels" :key="`${tunnel.display_address}-${index}`" class="tunnel-row">
            <span><strong>{{ tunnel.display_address || 'Tunnel' }}</strong><small>{{ tunnel.destination }}</small></span>
            <span :class="tunnel.is_disabled ? 'disabled' : 'enabled'">{{ tunnel.is_disabled ? tunnel.disabled_reason || 'Letiltva' : 'Aktív' }}</span>
          </div>
          <div v-for="tunnel in pendingTunnels" :key="tunnel.id" class="tunnel-row pending">
            <span><strong>{{ tunnel.id }}</strong><small>{{ tunnel.status_msg }}</small></span><span>Függőben</span>
          </div>
          <a href="https://playit.gg/account/agents" target="_blank" rel="noopener noreferrer">Tunnel létrehozása vagy beállítása a Playit fiókban</a>
        </section>
        <div class="actions">
          <btn :disabled="busy" @click="refresh"><icon name="reload" /> Frissítés</btn>
          <btn v-if="stateName === 'running'" color="error" :disabled="busy" @click="stopAgent"><icon name="stop" /> Agent leállítása</btn>
        </div>
      </template>
    </overlay>
  </section>
</template>

<style scoped lang="scss">
.playit-agent { margin-top:16px; padding:14px 16px; background:var(--color-background-secondary); border-radius:6px; }
.heading, .actions, .tunnel-row { display:flex; align-items:center; justify-content:space-between; gap:12px; }
.heading h2 { display:flex; align-items:center; gap:8px; margin:0; }
.heading p, .muted, small { color:var(--color-text-secondary); }
.heading p { margin:5px 0 0; }
.playit-overlay { max-width:760px; }
.tunnels { margin:16px 0; }
.tunnel-row { padding:10px 0; border-bottom:1px solid var(--color-background); }
.tunnel-row small { display:block; }
.enabled { color:var(--color-success); }
.disabled, .error { color:var(--color-error); }
.notice { color:var(--color-success); }
.setup, .actions { display:flex; align-items:flex-end; gap:8px; flex-wrap:wrap; margin-top:12px; }
.setup :deep(.input-field-wrapper) { flex:1; min-width:min(100%, 20rem); }
.actions { justify-content:flex-start; }
@media(max-width:600px) { .heading, .tunnel-row { align-items:flex-start; flex-direction:column; } }
</style>