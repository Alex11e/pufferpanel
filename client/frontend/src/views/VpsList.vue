<script setup>
import { ref, computed, inject, onMounted, onUnmounted, watch } from 'vue'
import { RouterLink } from 'vue-router'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const events = inject('events')

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
const statusLabels = { online: 'Fut', offline: 'Leállítva', installing: 'Telepítés', loading: '...' }

async function loadStatus(vps) {
  try {
    vps.status = await api.server.getStatus(vps.id)
  } catch {
    vps.status = undefined
  }
}

async function load() {
  const data = await api.server.list(1, 100, undefined, props.type)
  vpsList.value = data.servers || []
  await Promise.all(vpsList.value.map(loadStatus))
}

async function power(vps, action) {
  busy.value = { ...busy.value, [vps.id]: true }
  try {
    await api.server[action](vps.id)
    await loadStatus(vps)
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
  vpsList.value = []
  try {
    await load()
  } finally {
    loading.value = false
  }
}

watch(() => props.type, reload)

onMounted(async () => {
  await reload()
  interval = setInterval(() => vpsList.value.forEach(loadStatus), 30000)
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
    <loader v-if="loading" />
    <div v-else-if="vpsList.length === 0" class="empty">{{ empty }}</div>
    <div v-for="vps in vpsList" v-else :key="vps.id" class="vps-card">
      <div class="vps-info">
        <router-link :to="{ name: 'ServerView', params: { id: vps.id } }"><strong>{{ vps.name }}</strong></router-link>
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
.vps-card { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; padding: 16px; margin-bottom: 12px; border-radius: 8px; background: var(--color-background-secondary); border: 1px solid var(--color-background); box-shadow: 0 1px 3px rgba(0, 0, 0, .12); }
.vps-info { display: flex; flex-direction: column; gap: 4px; }
.vps-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.status.online { color: var(--color-success); }
.status.offline { color: var(--color-text-secondary); }
.status.installing { color: var(--color-warning); }
</style>
