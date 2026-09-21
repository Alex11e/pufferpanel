<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const servers = ref([])
const tickets = ref([])
const loading = ref(true)
const favorites = ref(JSON.parse(localStorage.getItem('favoriteServers') || '[]'))

const online = computed(() => servers.value.filter(server => server.online === 'online').length)
const offline = computed(() => servers.value.filter(server => server.online === 'offline').length)
const favoriteServers = computed(() => servers.value.filter(server => favorites.value.includes(server.id)).slice(0, 6))
const recentTickets = computed(() => tickets.value.slice(0, 5))

function address(server) {
  if (server.subdomain) return server.subdomain
  const host = server.ip && server.ip !== '0.0.0.0' ? server.ip : server.node?.publicHost
  return host + (server.port ? `:${server.port}` : '')
}

async function refresh() {
  loading.value = true
  try {
    const [serverResponse, ticketResponse] = await Promise.all([api.server.list(1), api.get('/api/tickets')])
    servers.value = serverResponse.servers || []
    tickets.value = ticketResponse.data || []
    await Promise.all(servers.value.map(async server => {
      if (!server.canGetStatus) return
      try { server.online = await api.server.getStatus(server.id) } catch { server.online = undefined }
    }))
  } finally { loading.value = false }
}

onMounted(refresh)
</script>

<template>
  <div class="dashboard">
    <div class="heading"><div><h1><icon name="home" /> Kezdőlap</h1><p>Gyors áttekintés a szervereidről és támogatási ügyeidről.</p></div><btn :disabled="loading" @click="refresh"><icon :name="loading ? 'loading' : 'reload'" :spin="loading" /> Frissítés</btn></div>
    <loader v-if="loading && !servers.length" />
    <template v-else>
      <div class="metrics"><div><b>{{ servers.length }}</b><span>Szerver</span></div><div class="online"><b>{{ online }}</b><span>Online</span></div><div class="offline"><b>{{ offline }}</b><span>Offline</span></div><div><b>{{ tickets.filter(ticket => ticket.status !== 'closed').length }}</b><span>Nyitott hibajegy</span></div></div>
      <div class="quick"><router-link to="/servers"><btn color="primary"><icon name="server" /> Szervereim</btn></router-link><router-link to="/servers/new"><btn v-if="$api.auth.hasScope('server.create')"><icon name="plus" /> Új szerver</btn></router-link><router-link to="/support"><btn><icon name="help" /> Támogatás</btn></router-link><router-link to="/self"><btn><icon name="account" /> Fiókom</btn></router-link></div>
      <section><div class="section-title"><h2>Kedvenc szerverek</h2><router-link to="/servers">Összes szerver</router-link></div><p v-if="!favoriteServers.length" class="muted">A szerverlistában a csillag ikonra kattintva adhatsz hozzá kedvenceket.</p><div v-else class="server-grid"><router-link v-for="server in favoriteServers" :key="server.id" :to="`/servers/view/${server.id}`" class="server-card"><span :class="['dot', server.online]" /><strong>{{ server.name }}</strong><small>{{ address(server) }}</small></router-link></div></section>
      <section><div class="section-title"><h2>Legutóbbi hibajegyek</h2><router-link to="/support">Összes jegy</router-link></div><p v-if="!recentTickets.length" class="muted">Nincs hibajegyed.</p><router-link v-for="ticket in recentTickets" v-else :key="ticket.id" to="/support" class="ticket"><span :class="['priority', ticket.priority]" /><strong>#{{ ticket.id }} · {{ ticket.subject }}</strong><small>{{ ticket.status }} · {{ ticket.category }}</small></router-link></section>
    </template>
  </div>
</template>

<style scoped lang="scss">
.heading, .section-title { display:flex; align-items:center; justify-content:space-between; gap:12px; } .heading h1 { display:flex; align-items:center; gap:10px; margin-bottom:4px; } .heading p, .muted, small { color:var(--color-text-secondary); } .metrics { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:12px; margin:20px 0; } .metrics > div, section { padding:16px; border-radius:8px; background:var(--color-background-secondary); } .metrics b, .metrics span { display:block; } .metrics b { font-size:1.7rem; } .metrics .online b { color:var(--color-success); } .metrics .offline b { color:var(--color-error); } .quick { display:flex; flex-wrap:wrap; gap:9px; } section { margin-top:16px; } section h2 { margin:0; } .section-title a { color:var(--color-primary); } .server-grid { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:10px; margin-top:12px; } .server-card, .ticket { display:block; padding:12px; color:var(--color-text); text-decoration:none; border-radius:6px; background:var(--color-background); } .server-card:hover, .ticket:hover { outline:1px solid var(--color-primary); } .server-card strong, .server-card small, .ticket strong, .ticket small { display:block; } .dot, .priority { display:inline-block; width:8px; height:8px; margin-right:7px; border-radius:50%; background:var(--color-text-secondary); } .dot.online { background:var(--color-success); } .dot.offline, .priority.urgent { background:var(--color-error); } .priority.high { background:var(--color-warning, #e6a700); } .ticket { margin-top:8px; } @media(max-width:700px) { .metrics, .server-grid { grid-template-columns:repeat(2,minmax(0,1fr)); } .heading { align-items:flex-start; flex-direction:column; } }
</style>
