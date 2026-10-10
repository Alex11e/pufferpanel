<script setup>
import { computed, inject, onMounted, onUnmounted, ref, shallowRef } from 'vue'
import { RouterLink } from 'vue-router'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import PowerControls from '@/components/server/PowerControls.vue'

const api = inject('api')
const servers = ref([])
const allServers = ref([])
const tickets = ref([])
const loading = ref(true)
const loadError = ref('')
const totalServers = ref(0)
const statusUpdatedAt = ref(null)
const favoriteStats = ref({})
const favoriteControllers = shallowRef({})
let statusInterval = null
let statusRefreshing = false

function readFavorites() {
  try {
    const stored = JSON.parse(localStorage.getItem('favoriteServers') || '[]')
    return Array.isArray(stored) ? stored : []
  } catch {
    return []
  }
}

const favorites = ref(readFavorites())

const online = computed(() => servers.value.filter(server => server.online === 'online').length)
const offline = computed(() => servers.value.filter(server => server.online === 'offline').length)
const offlineServers = computed(() => servers.value.filter(server => server.online === 'offline').slice(0, 5))
const favoriteServers = computed(() => servers.value.filter(server => favorites.value.includes(server.id)).slice(0, 6))
const recentTickets = computed(() => tickets.value.slice(0, 5))
const expiringServers = computed(() => {
  const deadline = Date.now() + 7 * 24 * 60 * 60 * 1000
  return allServers.value
    .map(server => ({ server, expiryTime: Date.parse(server.expiresAt) }))
    .filter(item => Number.isFinite(item.expiryTime) && item.expiryTime <= deadline)
    .sort((a, b) => a.expiryTime - b.expiryTime)
    .slice(0, 5)
})

async function loadAllServers(firstPage) {
  const firstServers = firstPage.servers || []
  const total = firstPage.paging?.total ?? firstServers.length
  const pageSize = firstPage.paging?.pageSize ?? firstServers.length
  const pageCount = pageSize > 0 ? Math.ceil(total / pageSize) : 1
  if (pageCount <= 1) return firstServers

  const remainingPages = await Promise.all(
    Array.from({ length: pageCount - 1 }, (_, index) => api.server.list(index + 2, pageSize))
  )
  return firstServers.concat(...remainingPages.map(page => page.servers || []))
}

async function loadFavoriteControllers() {
  const favoriteIds = favoriteServers.value.map(server => server.id)
  const results = await Promise.allSettled(favoriteIds.map(id => api.server.get(id)))
  const controllers = {}
  for (const result of results) {
    if (result.status === 'fulfilled') controllers[result.value.id] = result.value
  }
  Object.values(favoriteControllers.value).forEach(server => server.closeSocket())
  favoriteControllers.value = controllers
}

function address(server) {
  if (server.subdomain) return server.subdomain
  const host = server.ip && server.ip !== '0.0.0.0' ? server.ip : server.node?.publicHost
  return host + (server.port ? `:${server.port}` : '')
}

function formatBytes(value) {
  let bytes = Number(value)
  if (!Number.isFinite(bytes) || bytes < 0) return ''
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let unit = 0
  while (bytes >= 1024 && unit < units.length - 1) {
    bytes /= 1024
    unit++
  }
  return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 }).format(bytes)} ${units[unit]}`
}

async function refreshFavoriteStats() {
  const results = await Promise.all(favoriteServers.value.map(async server => {
    const controller = favoriteControllers.value[server.id]
    if (server.online !== 'online' || !controller?.hasScope('server.stats')) return [server.id, null]
    try {
      const stats = await controller.getStats()
      if (!Number.isFinite(Number(stats?.cpu)) || !Number.isFinite(Number(stats?.memory))) return [server.id, null]
      return [server.id, { cpu: Number(stats.cpu), memory: Number(stats.memory) }]
    } catch {
      return [server.id, null]
    }
  }))
  favoriteStats.value = Object.fromEntries(results.filter(([, stats]) => stats))
}

async function updateServerStatuses() {
  await Promise.all(servers.value.map(async server => {
    if (!server.canGetStatus) return
    try { server.online = await api.server.getStatus(server.id) } catch { server.online = undefined }
  }))
  statusUpdatedAt.value = new Date()
}

async function refreshStatuses() {
  if (loading.value || statusRefreshing || !servers.value.length) return
  statusRefreshing = true
  try {
    await updateServerStatuses()
    await refreshFavoriteStats()
  } finally {
    statusRefreshing = false
  }
}

async function refresh() {
  loading.value = true
  loadError.value = ''
  const [serverResult, ticketResult] = await Promise.allSettled([api.server.list(1), api.get('/api/tickets')])
  if (serverResult.status === 'fulfilled') {
    servers.value = serverResult.value.servers || []
    totalServers.value = serverResult.value.paging?.total ?? servers.value.length
    try {
      allServers.value = await loadAllServers(serverResult.value)
    } catch {
      allServers.value = servers.value
      loadError.value = 'A lejárati figyelmeztetések betöltése nem sikerült.'
    }
    await updateServerStatuses()
    await loadFavoriteControllers()
    await refreshFavoriteStats()
  } else {
    loadError.value = 'A szerverek betöltése nem sikerült.'
  }
  if (ticketResult.status === 'fulfilled') {
    tickets.value = ticketResult.value.data || []
  } else {
    loadError.value = loadError.value ? `${loadError.value} A hibajegyek betöltése nem sikerült.` : 'A hibajegyek betöltése nem sikerült.'
  }
  loading.value = false
}

onMounted(() => {
  refresh()
  statusInterval = setInterval(refreshStatuses, 30 * 1000)
})
onUnmounted(() => {
  clearInterval(statusInterval)
  Object.values(favoriteControllers.value).forEach(server => server.closeSocket())
})
</script>

<template>
  <div class="dashboard">
    <div class="heading"><div><h1><icon name="home" /> Kezdőlap</h1><p>Gyors áttekintés a szervereidről és támogatási ügyeidről.</p><small v-if="statusUpdatedAt">Státusz frissítve: {{ statusUpdatedAt.toLocaleTimeString() }}</small></div><btn :disabled="loading" @click="refresh"><icon :name="loading ? 'loading' : 'reload'" :spin="loading" /> Frissítés</btn></div>
    <loader v-if="loading && !servers.length" />
    <template v-else>
      <p v-if="loadError" class="load-error" role="alert">{{ loadError }}</p>
      <div class="metrics"><div><b>{{ totalServers }}</b><span>Szerver</span></div><div class="online"><b>{{ online }}</b><span>Online az első oldalon</span></div><div class="offline"><b>{{ offline }}</b><span>Offline az első oldalon</span></div><div><b>{{ tickets.filter(ticket => ticket.status !== 'closed').length }}</b><span>Nyitott hibajegy</span></div></div>
      <div class="quick"><router-link to="/servers"><btn color="primary"><icon name="server" /> Szervereim</btn></router-link><router-link to="/servers/new"><btn v-if="$api.auth.hasScope('server.create')"><icon name="plus" /> Új szerver</btn></router-link><router-link to="/support"><btn><icon name="help" /> Támogatás</btn></router-link><router-link to="/self"><btn><icon name="account" /> Fiókom</btn></router-link></div>
      <section v-if="offlineServers.length" class="attention-section"><div class="section-title"><h2>Offline szerverek</h2><router-link to="/servers">Összes szerver</router-link></div><router-link v-for="server in offlineServers" :key="server.id" :to="`/servers/view/${server.id}`" class="attention-row"><span class="attention-marker" /><span class="attention-copy"><strong>{{ server.name }}</strong><small>{{ address(server) }}</small></span><icon name="chevron-right" /></router-link></section>
      <section v-if="expiringServers.length" class="expiry-section"><div class="section-title"><h2>Figyelmet igényel</h2><router-link to="/servers">Összes szerver</router-link></div><router-link v-for="item in expiringServers" :key="item.server.id" :to="`/servers/view/${item.server.id}`" class="expiry-row"><span :class="['expiry-marker', { expired: item.expiryTime <= Date.now() }]" /><span class="expiry-copy"><strong>{{ item.server.name }}</strong><small>{{ item.expiryTime <= Date.now() ? 'Lejárt' : `Lejár ${Math.ceil((item.expiryTime - Date.now()) / 86400000)} napon belül` }} · {{ new Date(item.expiryTime).toLocaleDateString() }}</small></span><icon name="chevron-right" /></router-link></section>
      <section><div class="section-title"><h2>Kedvenc szerverek</h2><router-link to="/servers">Összes szerver</router-link></div><p v-if="!favoriteServers.length" class="muted">A szerverlistában a csillag ikonra kattintva adhatsz hozzá kedvenceket.</p><div v-else class="server-grid"><div v-for="server in favoriteServers" :key="server.id" class="server-card"><router-link :to="`/servers/view/${server.id}`" class="server-link"><span :class="['dot', server.online]" /><strong>{{ server.name }}</strong><small>{{ address(server) }}</small><small v-if="favoriteStats[server.id]">CPU {{ new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 }).format(favoriteStats[server.id].cpu) }}% · RAM {{ formatBytes(favoriteStats[server.id].memory) }}</small></router-link><power-controls v-if="favoriteControllers[server.id]" :server="favoriteControllers[server.id]" /></div></div></section>
      <section><div class="section-title"><h2>Legutóbbi hibajegyek</h2><router-link to="/support">Összes jegy</router-link></div><p v-if="!recentTickets.length" class="muted">Nincs hibajegyed.</p><router-link v-for="ticket in recentTickets" v-else :key="ticket.id" to="/support" class="ticket"><span :class="['priority', ticket.priority]" /><strong>#{{ ticket.id }} · {{ ticket.subject }}</strong><small>{{ ticket.status }} · {{ ticket.category }}</small></router-link></section>
    </template>
  </div>
</template>

<style scoped lang="scss">
.heading, .section-title { display:flex; align-items:center; justify-content:space-between; gap:12px; } .heading h1 { display:flex; align-items:center; gap:10px; margin-bottom:4px; } .heading p, .muted, small { color:var(--color-text-secondary); } .metrics { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:12px; margin:20px 0; } .metrics > div, section { padding:16px; border-radius:8px; background:var(--color-background-secondary); } .metrics b, .metrics span { display:block; } .metrics b { font-size:1.7rem; } .metrics .online b { color:var(--color-success); } .metrics .offline b { color:var(--color-error); } .quick { display:flex; flex-wrap:wrap; gap:9px; } section { margin-top:16px; } section h2 { margin:0; } .section-title a { color:var(--color-primary); } .server-grid { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:10px; margin-top:12px; } .server-card, .ticket { display:block; padding:12px; color:var(--color-text); text-decoration:none; border-radius:6px; background:var(--color-background); } .server-card:hover, .ticket:hover { outline:1px solid var(--color-primary); } .server-link { display:block; color:inherit; text-decoration:none; } .server-card strong, .server-card small, .ticket strong, .ticket small { display:block; } .dot, .priority { display:inline-block; width:8px; height:8px; margin-right:7px; border-radius:50%; background:var(--color-text-secondary); } .dot.online { background:var(--color-success); } .dot.offline, .priority.urgent { background:var(--color-error); } .priority.high { background:var(--color-warning, #e6a700); } .ticket { margin-top:8px; } @media(max-width:700px) { .metrics, .server-grid { grid-template-columns:repeat(2,minmax(0,1fr)); } .heading { align-items:flex-start; flex-direction:column; } }
.expiry-section { border-left:3px solid var(--color-warning, #e6a700); }
.expiry-row { display:flex; align-items:center; gap:10px; margin-top:8px; padding:10px; color:var(--color-text); text-decoration:none; border-radius:6px; background:var(--color-background); }
.expiry-row:hover { outline:1px solid var(--color-primary); }
.expiry-marker { flex:0 0 auto; width:9px; height:9px; border-radius:50%; background:var(--color-warning, #e6a700); }
.expiry-marker.expired { background:var(--color-error); }
.expiry-copy { min-width:0; flex:1; }
.expiry-copy strong, .expiry-copy small { display:block; overflow-wrap:anywhere; }
.attention-section { border-left:3px solid var(--color-error); }
.attention-row { display:flex; align-items:center; gap:10px; margin-top:8px; padding:10px; color:var(--color-text); text-decoration:none; border-radius:6px; background:var(--color-background); }
.attention-row:hover { outline:1px solid var(--color-primary); }
.attention-marker { flex:0 0 auto; width:9px; height:9px; border-radius:50%; background:var(--color-error); }
.attention-copy { min-width:0; flex:1; }
.attention-copy strong, .attention-copy small { display:block; overflow-wrap:anywhere; }
</style>
