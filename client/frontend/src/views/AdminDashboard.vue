<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import Btn from '@/components/ui/Btn.vue'

const api = inject('api')
const events = inject('events')
const overview = ref(null)
const ports = ref([])
const activity = ref([])
const backups = ref([])
const loading = ref(true)
const error = ref('')
const announcements = ref([])
const announcement = ref({ title: '', message: '', level: 'info', active: true })
const adminServers = ref([])
const selectedServers = ref([])
const actionRunning = ref(false)
const announcementSaving = ref(false)
const selectedCount = computed(() => selectedServers.value.length)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [overviewResponse, portsResponse, activityResponse, backupsResponse, announcementResponse, serversResponse] = await Promise.all([
      api.get('/api/admin/overview'),
      api.get('/api/admin/ports'),
      api.server.getRecentActivity(),
      api.get('/api/admin/backups'),
      api.get('/api/announcements/admin'),
      api.server.list(1)
    ])
    overview.value = overviewResponse.data
    ports.value = portsResponse.data || []
    activity.value = activityResponse || []
    backups.value = backupsResponse.data || []
    announcements.value = announcementResponse.data || []
    adminServers.value = serversResponse.servers || []
  } catch {
    error.value = 'Az admin adatok betöltése nem sikerült. Próbáld meg újra.'
  } finally {
    loading.value = false
  }
}

async function saveAnnouncement() {
  if (!announcement.value.title.trim() || !announcement.value.message.trim()) return
  announcementSaving.value = true
  try {
    await api.post('/api/announcements/admin', announcement.value)
    announcement.value = { title: '', message: '', level: 'info', active: true }
    await load()
  } finally { announcementSaving.value = false }
}

async function deleteAnnouncement(item) {
  events.emit('confirm', `Közlemény törlése: ${item.title}?`, {
    text: 'Törlés', icon: 'remove', color: 'error', action: async () => {
      await api.delete(`/api/announcements/admin/${item.id}`)
      await load()
    }
  })
}

function bulkAction(action) {
  const labels = { start: 'elindítása', restart: 'újraindítása', stop: 'leállítása' }
  events.emit('confirm', {
    title: `${selectedCount.value} szerver ${labels[action]}`,
    body: 'A művelet minden kiválasztott szerverre lefut. Biztosan folytatod?'
  }, {
    text: 'Végrehajtás', icon: action === 'stop' ? 'stop' : 'apply', color: action === 'stop' ? 'error' : 'primary', action: async () => {
      actionRunning.value = true
      try {
        const response = await api.post('/api/admin/servers/action', { serverIds: selectedServers.value, action })
        const failed = (response.data || []).filter(result => !result.success)
        if (failed.length) error.value = `${failed.length} szerver művelete nem sikerült.`
      } finally { actionRunning.value = false }
    }
  })
}

function activityTitle(record) {
  return `${record.username} — ${record.action}`
}

onMounted(load)
</script>

<template>
  <div class="admin-dashboard">
    <div class="heading">
      <div>
        <h1><icon name="admin" /> Admin központ</h1>
        <p>Minden szerverhez teljes hozzáférés és panel-szintű áttekintés.</p>
      </div>
      <btn :disabled="loading" @click="load"><icon :name="loading ? 'loading' : 'reload'" :spin="loading" /> Frissítés</btn>
    </div>
    <loader v-if="loading && !overview" />
    <div v-else-if="error" class="error">{{ error }}</div>
    <template v-else-if="overview">
      <div class="metrics">
        <div class="metric"><strong>{{ overview.servers }}</strong><span>Szerver</span></div>
        <div class="metric"><strong>{{ overview.users }}</strong><span>Felhasználó</span></div>
        <div class="metric"><strong>{{ overview.nodes }}</strong><span>Node</span></div>
        <div class="metric"><strong>{{ overview.allocatedPorts }}</strong><span>Lefoglalt port</span></div>
        <div class="metric"><strong>{{ overview.backups }}</strong><span>Biztonsági mentés</span></div>
        <div class="metric"><strong>{{ overview.automaticBackupServers }}</strong><span>Automata mentés</span></div>
      </div>
      <section class="access"><icon name="success" /> Az admin szerepkör minden szerverhez és annak konzoljához, fájljaihoz, mentéseihez és beállításaihoz hozzáfér.</section>
      <div class="quick-links">
        <router-link to="/servers"><btn color="primary"><icon name="server" /> Összes szerver kezelése</btn></router-link>
        <router-link to="/users"><btn><icon name="users" /> Felhasználók kezelése</btn></router-link>
        <router-link to="/nodes"><btn><icon name="node" /> Node-ok és portok</btn></router-link>
        <router-link to="/templates"><btn><icon name="template" /> Sablonok és eggek</btn></router-link>
      </div>
      <section>
        <h2>Panel-közlemények</h2>
        <p class="hint">Bejelentkezés után minden felhasználó látja. A bezárt közlemény módosítás után ismét megjelenik.</p>
        <div class="announcement-form">
          <input v-model="announcement.title" maxlength="140" placeholder="Cím" aria-label="Közlemény címe">
          <select v-model="announcement.level" aria-label="Közlemény típusa"><option value="info">Információ</option><option value="warning">Figyelmeztetés</option><option value="maintenance">Karbantartás</option></select>
          <textarea v-model="announcement.message" maxlength="4000" placeholder="Közlemény szövege" aria-label="Közlemény szövege" />
          <label><input v-model="announcement.active" type="checkbox"> Aktív</label>
          <btn color="primary" :disabled="announcementSaving || !announcement.title.trim() || !announcement.message.trim()" @click="saveAnnouncement">Közzététel</btn>
        </div>
        <div v-if="announcements.length === 0" class="empty">Nincs létrehozott közlemény.</div>
        <div v-for="item in announcements" v-else :key="item.id" class="announcement-row"><span><strong>{{ item.title }}</strong><small>{{ item.level }} · {{ item.active ? 'aktív' : 'inaktív' }}</small></span><btn variant="icon" tooltip="Törlés" @click="deleteAnnouncement(item)"><icon name="remove" /></btn></div>
      </section>
      <section>
        <h2>Tömeges szerverműveletek</h2>
        <p class="hint">Az első 100 szerver közül választhatsz ki legfeljebb 25-öt. Minden végrehajtás előtt külön megerősítés szükséges.</p>
        <div class="bulk-actions"><btn color="primary" :disabled="!selectedCount || actionRunning" @click="bulkAction('start')">Indítás ({{ selectedCount }})</btn><btn :disabled="!selectedCount || actionRunning" @click="bulkAction('restart')">Újraindítás</btn><btn color="error" :disabled="!selectedCount || actionRunning" @click="bulkAction('stop')">Leállítás</btn></div>
        <div class="server-select"><label v-for="server in adminServers" :key="server.id"><input v-model="selectedServers" type="checkbox" :value="server.id" :disabled="!selectedServers.includes(server.id) && selectedCount >= 25"> <span>{{ server.name }} <small>({{ server.id }})</small></span></label></div>
      </section>
      <section>
        <h2>Portkihasználtság</h2>
        <div v-if="ports.length === 0" class="empty">Nincs elérhető node.</div>
        <div v-for="entry in ports" :key="entry.node.id" class="port-row">
          <div><strong>{{ entry.node.name }}</strong><small>{{ entry.node.portRangeStart }}–{{ entry.node.portRangeEnd }} · TCP + UDP</small></div>
          <div class="usage"><span :style="{ width: `${entry.capacity ? (entry.used / entry.capacity) * 100 : 0}%` }" /></div>
          <b>{{ entry.used }} / {{ entry.capacity }}</b>
        </div>
      </section>
      <section>
        <h2>Legutóbbi biztonsági mentések</h2>
        <div v-if="backups.length === 0" class="empty">Még nincs biztonsági mentés.</div>
        <router-link v-for="backup in backups" v-else :key="backup.id" :to="`/servers/view/${backup.serverId}`" class="backup"><icon name="backup" /><span><strong>{{ backup.name }}</strong><small>{{ backup.serverName || backup.serverId }}</small></span><small>{{ new Date(backup.createdAt).toLocaleString() }}</small></router-link>
      </section>
      <section>
        <h2>Legutóbbi műveletek</h2>
        <div v-if="activity.length === 0" class="empty">Még nincs naplózott művelet.</div>
        <div v-for="record in activity" v-else :key="record.id" class="activity"><icon name="stats" /><span>{{ activityTitle(record) }}</span><small>{{ record.serverId }} · {{ record.ipAddress }}</small></div>
      </section>
    </template>
  </div>
</template>

<style scoped lang="scss">
.heading { display:flex; align-items:center; justify-content:space-between; gap:16px; margin-bottom:20px; } .heading h1 { display:flex; align-items:center; gap:10px; margin-bottom:4px; } .heading p, small, .hint { color:var(--color-text-secondary); } .metrics { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:12px; } .metric, .access, section, .error { background:var(--color-background-secondary); border-radius:8px; padding:16px; } .metric strong { display:block; font-size:1.8rem; } .metric span { color:var(--color-text-secondary); } .access { margin:16px 0; color:var(--color-success); display:flex; gap:10px; align-items:center; } .error { color:var(--color-error); } .quick-links, .bulk-actions { display:flex; flex-wrap:wrap; gap:10px; margin-bottom:12px; } section { margin-top:16px; } section h2 { margin-top:0; } .port-row, .activity, .backup, .announcement-row { display:flex; align-items:center; gap:12px; padding:10px 0; border-bottom:1px solid var(--color-background); } .port-row:last-child, .activity:last-child, .backup:last-child, .announcement-row:last-child { border-bottom:0; } .port-row small, .backup small, .announcement-row small { display:block; } .usage { flex:1; height:8px; overflow:hidden; background:var(--color-background); border-radius:99px; } .usage span { display:block; height:100%; background:var(--color-primary); } .activity small, .backup > small, .announcement-row > :last-child { margin-left:auto; } .backup { color:inherit; text-decoration:none; } .backup:hover { color:var(--color-primary); } .empty { color:var(--color-text-secondary); } .announcement-form { display:grid; grid-template-columns:1fr auto; gap:9px; margin:12px 0; } .announcement-form input, .announcement-form textarea, .announcement-form select { padding:9px; border:1px solid var(--color-background); border-radius:5px; background:var(--color-background); color:var(--color-text); } .announcement-form textarea { grid-column:1 / -1; min-height:70px; resize:vertical; } .announcement-form label { display:flex; align-items:center; gap:6px; } .server-select { max-height:250px; overflow:auto; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:6px; } .server-select label { padding:7px; border-radius:5px; background:var(--color-background); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; } @media (max-width:640px) { .metrics, .server-select { grid-template-columns:repeat(2,minmax(0,1fr)); } .heading { align-items:flex-start; flex-direction:column; } .activity small, .backup > small { display:none; } .announcement-form { grid-template-columns:1fr; } }
</style>
