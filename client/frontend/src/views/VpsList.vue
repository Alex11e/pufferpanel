<script setup>
import { ref, computed, inject, onMounted, onUnmounted, watch } from 'vue'
import { RouterLink } from 'vue-router'
import Btn from '@/components/ui/Btn.vue'
import { serverIconName } from '@/utils/serverIcon'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const events = inject('events')
const search = ref('')
const selectedStatus = ref('')
const loadError = ref('')
const actionError = ref('')

const props = defineProps({
  type: { type: String, default: 'hypervm' },
  title: { type: String, default: 'VPS' },
  hint: { type: String, default: 'Virtuális gépek (QEMU) saját konzollal, VNC-vel és ISO-kezeléssel.' },
  createRoute: { type: String, default: 'VpsCreate' },
  empty: { type: String, default: 'Még nincs VPS.' }
})

const vpsList = ref([])
const loading = ref(true)
const busy = ref({})
let interval = null

const canCreate = computed(() => api.auth.hasScope('server.create'))
const statusLabels = { online: 'Fut', offline: 'Leállítva', installing: 'Telepítés', unknown: 'Állapot nem érhető el' }
const visibleVps = computed(() => vpsList.value.filter(vps => {
  const matchesSearch = `${vps.name} ${vps.node?.name || ''}`.toLowerCase().includes(search.value.trim().toLowerCase())
  return matchesSearch && (!selectedStatus.value || vps.status === selectedStatus.value)
}))
const counts = computed(() => ({
  total: vpsList.value.length,
  online: vpsList.value.filter(vps => vps.status === 'online').length,
  offline: vpsList.value.filter(vps => vps.status === 'offline').length
}))

function errorMessage(error) {
  return error?.msg || error?.message || 'Ismeretlen hiba történt.'
}

async function loadStatus(vps) {
  try {
    vps.status = await api.server.getStatus(vps.id)
  } catch {
    vps.status = 'unknown'
  }
}

async function load() {
  const servers = []
  let page = 1
  let total = 0
  let hasMore = true
  while (hasMore) {
    const data = await api.server.list(page, 100, undefined, props.type)
    servers.push(...(data.servers || []))
    total = data.paging?.total || servers.length
    hasMore = Boolean(data.paging && servers.length < total && data.servers?.length)
    page += 1
  }
  await Promise.all(servers.map(loadStatus))
  vpsList.value = servers
}

async function power(vps, action) {
  busy.value = { ...busy.value, [vps.id]: true }
  actionError.value = ''
  try {
    await api.server[action](vps.id)
    await loadStatus(vps)
  } catch (error) {
    actionError.value = `${vps.name}: ${errorMessage(error)}`
  } finally {
    busy.value = { ...busy.value, [vps.id]: false }
  }
}

function confirmKill(vps) {
  events.emit('confirm', `Kényszerített leállítás: ${vps.name}? A vendég rendszer nem áll le rendesen.`, {
    text: 'Leállítás', icon: 'stop', color: 'error', action: () => power(vps, 'kill')
  })
}

async function reload() {
  loading.value = true
  loadError.value = ''
  try {
    await load()
  } catch (error) {
    loadError.value = errorMessage(error)
  } finally {
    loading.value = false
  }
}

watch(() => props.type, reload)

onMounted(() => {
  interval = setInterval(() => vpsList.value.forEach(loadStatus), 30000)
  reload()
})

onUnmounted(() => clearInterval(interval))
</script>

<template>
  <div class="vps-list">
    <div class="heading">
      <div>
        <h1><icon name="server" /> {{ title }}</h1>
        <p class="hint">{{ hint }}</p>
      </div>
      <router-link v-if="canCreate" :to="{ name: createRoute }"><btn color="primary"><icon name="plus" /> Új</btn></router-link>
    </div>
    <div class="toolbar">
      <input v-model="search" type="search" :placeholder="`${title} vagy node keresése`" :aria-label="`${title} vagy node keresése`">
      <btn v-for="filter in [{ value: '', label: 'Mind' }, { value: 'online', label: 'Fut' }, { value: 'offline', label: 'Leállítva' }, { value: 'installing', label: 'Telepítés' }]" :key="filter.value" :color="selectedStatus === filter.value ? 'primary' : undefined" @click="selectedStatus = filter.value">{{ filter.label }}</btn>
      <btn variant="icon" tooltip="Lista frissítése" :disabled="loading" @click="reload"><icon name="reload" /></btn>
    </div>
    <div class="summary"><span>{{ counts.total }} {{ title }}</span><span>{{ counts.online }} fut</span><span>{{ counts.offline }} leállítva</span></div>
    <p v-if="loadError" class="error" role="alert">{{ loadError }}</p>
    <p v-if="actionError" class="error" role="alert">{{ actionError }}</p>
    <loader v-if="loading" />
    <div v-else-if="!vpsList.length && !loadError" class="empty">{{ empty }}</div>
    <div v-else-if="!visibleVps.length && !loadError" class="empty">Nincs a keresésnek megfelelő találat.</div>
    <div v-for="vps in visibleVps" v-else :key="vps.id" class="vps-card">
      <div class="vps-info">
        <router-link class="vps-name" :to="{ name: 'ServerView', params: { id: vps.id } }"><icon :name="serverIconName(vps)" /><strong>{{ vps.name }}</strong></router-link>
        <small>{{ vps.node?.name }} · <span :class="['status', vps.status]">{{ statusLabels[vps.status] || 'Ismeretlen' }}</span></small>
      </div>
      <div class="vps-actions">
        <btn v-if="vps.status === 'offline'" color="primary" :disabled="busy[vps.id]" @click="power(vps, 'start')"><icon name="play" /> Indítás</btn>
        <template v-else-if="vps.status === 'online'">
          <btn :disabled="busy[vps.id]" @click="power(vps, 'restart')"><icon name="reload" /> Újraindítás</btn>
          <btn :disabled="busy[vps.id]" @click="power(vps, 'stop')"><icon name="stop" /> Leállítás</btn>
          <btn color="error" :disabled="busy[vps.id]" @click="confirmKill(vps)"><icon name="stop" /> Kill</btn>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 20px; flex-wrap: wrap; }
.heading h1 { display: flex; align-items: center; gap: 10px; margin-bottom: 4px; }
.hint, small, .empty { color: var(--color-text-secondary); }
.toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 10px; }
.toolbar input { flex: 1 1 220px; min-width: 160px; padding: 9px; border: 1px solid var(--color-background); border-radius: 5px; background: var(--color-background-secondary); color: var(--color-text); }
.summary { display: flex; gap: 16px; margin: 8px 0 16px; color: var(--color-text-secondary); }
.error { color: var(--color-error); }
.vps-card { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; padding: 16px; margin-bottom: 12px; border-radius: 8px; background: var(--color-background-secondary); border: 1px solid var(--color-background); box-shadow: 0 1px 3px rgba(0, 0, 0, .12); }
.vps-info { display: flex; flex-direction: column; gap: 4px; }
.vps-name { display: inline-flex; align-items: center; gap: .55rem; }
.vps-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.status.online { color: var(--color-success); }
.status.offline { color: var(--color-text-secondary); }
.status.installing { color: var(--color-warning); }
</style>
