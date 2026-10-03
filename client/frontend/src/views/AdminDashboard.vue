<script setup>
import { computed, inject, onMounted, onUnmounted, ref } from 'vue'
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
const announcement = ref({ title: '', message: '', level: 'info', active: true, startsAt: '', expiresAt: '' })
const adminServers = ref([])
const selectedServers = ref([])
const actionRunning = ref(false)
const announcementSaving = ref(false)
const selectedCount = computed(() => selectedServers.value.length)
const expiring = ref([])
const expiryServer = ref('')
const expiryDate = ref('')
const backupLimit = ref(0)
const update = ref(null)
const updateReleases = ref([])
const selectedUpdateVersion = ref('latest')
const releaseListError = ref('')
const system = ref(null)
const updateApplying = ref(false)
const updateActionError = ref('')
let updatePoll = null
let updatePageActive = true

async function loadSystem() {
  system.value = (await api.get('/api/admin/system')).data
}

function formatUptime(seconds) {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return d ? `${d} nap ${h} óra` : h ? `${h} óra ${m} perc` : `${m} perc`
}
const updateChecking = ref(false)
const updateFailed = ref(false)

async function loadUpdateReleases(refresh = false) {
  releaseListError.value = ''
  try {
    const res = await api.get('/api/admin/update/releases', refresh ? { refresh: true } : {})
    updateReleases.value = res.data || []
  } catch (error) {
    releaseListError.value = updateErrorMessage(error)
  }
}

async function checkUpdate(refresh = false, version = selectedUpdateVersion.value) {
  updateChecking.value = true
  updateFailed.value = false
  try {
    const query = {}
    if (refresh) query.refresh = true
    if (version && version !== 'latest') query.version = version
    const res = await api.get('/api/admin/update', query, {}, { unhandledErrors: [502] })
    if (res) update.value = res.data
    else updateFailed.value = true
    return Boolean(res)
  } catch {
    updateFailed.value = true
    return false
  } finally { updateChecking.value = false }
}

function selectUpdateVersion() {
  checkUpdate(true, selectedUpdateVersion.value)
}

function refreshUpdates() {
  checkUpdate(true)
  loadUpdateReleases(true)
}

function formatAssetSize(size) {
  if (!size) return 'Ismeretlen méret'
  if (size < 1024 * 1024) return `${Math.ceil(size / 1024)} KB`
  return `${(size / (1024 * 1024)).toFixed(1)} MB`
}

async function pollUpdate() {
  const checked = await checkUpdate()
  if (updatePageActive && checked && update.value?.running) {
    updatePoll = setTimeout(pollUpdate, 2000)
  }
}

function updateErrorMessage(error) {
  const detail = error?.msg || error?.message || error?.response?.error?.msg || error?.response?.error
  return typeof detail === 'string' ? detail : 'A frissítés nem indítható el.'
}

function applyUpdate() {
  events.emit('confirm', `Frissítés indítása: ${update.value.current} → ${update.value.latest}. A panel a művelet során újraindulhat. Folytatod?`, {
    text: 'Frissítés', icon: 'apply', color: 'primary', action: async () => {
      updateActionError.value = ''
      updateApplying.value = true
      try {
        await api.post('/api/admin/update/apply', { version: update.value.latest })
        await checkUpdate()
        if (update.value?.running) updatePoll = setTimeout(pollUpdate, 2000)
      } catch (applyError) {
        updateActionError.value = updateErrorMessage(applyError)
      } finally {
        updateApplying.value = false
      }
    }
  })
}

onUnmounted(() => {
  updatePageActive = false
  clearTimeout(updatePoll)
})

async function loadLimits() {
  backupLimit.value = 0
  expiryDate.value = ''
  if (!expiryServer.value) return
  const data = (await api.get(`/api/admin/servers/${encodeURIComponent(expiryServer.value)}/limits`)).data
  backupLimit.value = data.backupLimit || 0
  if (data.expiresAt) {
    const d = new Date(data.expiresAt)
    // datetime-local expects local time without timezone
    expiryDate.value = new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
  }
}

async function saveBackupLimit() {
  if (!expiryServer.value) return
  await api.put(`/api/admin/servers/${encodeURIComponent(expiryServer.value)}/backup-limit`, { limit: Math.max(0, Math.floor(Number(backupLimit.value) || 0)) })
}

async function loadExpiring() {
  expiring.value = (await api.get('/api/admin/expiring')).data || []
}

async function saveExpiry(id, date) {
  if (!id) return
  await api.put(`/api/admin/servers/${encodeURIComponent(id)}/expiry`, { expiresAt: date ? new Date(date).toISOString() : null })
  expiryServer.value = ''
  expiryDate.value = ''
  await loadExpiring()
}

function daysLeft(date) {
  return Math.ceil((new Date(date) - Date.now()) / 86400000)
}

const expiredIds = computed(() => expiring.value.filter(s => new Date(s.expiresAt) < new Date()).map(s => s.id).slice(0, 25))

function stopExpired() {
  selectedServers.value = [...expiredIds.value]
  bulkAction('stop')
}

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
    await loadExpiring()
    checkUpdate()
    loadUpdateReleases()
    loadSystem()
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
    const startsAt = announcement.value.startsAt ? new Date(announcement.value.startsAt).toISOString() : null
    const expiresAt = announcement.value.expiresAt ? new Date(announcement.value.expiresAt).toISOString() : null
    if (startsAt && expiresAt && new Date(expiresAt) <= new Date(startsAt)) { error.value = 'A lejáratnak a kezdés után kell lennie.'; return }
    await api.post('/api/announcements/admin', { ...announcement.value, startsAt, expiresAt })
    announcement.value = { title: '', message: '', level: 'info', active: true, startsAt: '', expiresAt: '' }
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
        <div class="metric"><strong>{{ overview.openTickets }}</strong><span>Nyitott jegy</span></div>
        <div class="metric"><strong>{{ overview.newUsers7Days }}</strong><span>Új felhasználó (7 nap)</span></div>
      </div>
      <section class="access"><icon name="success" /> Az admin szerepkör minden szerverhez és annak konzoljához, fájljaihoz, mentéseihez és beállításaihoz hozzáfér.</section>
      <section class="update-card" :class="{ available: update && update.updateAvailable }">
        <div class="update-head">
          <h2>Panel frissítések</h2>
          <select v-model="selectedUpdateVersion" :disabled="updateChecking" aria-label="Frissítési verzió" @change="selectUpdateVersion">
            <option value="latest">Legújabb stabil</option>
            <option v-for="release in updateReleases" :key="release.tag" :value="release.tag">{{ release.tag }}{{ release.prerelease ? ' · előzetes' : '' }}</option>
          </select>
          <btn :disabled="updateChecking" @click="refreshUpdates"><icon :name="updateChecking ? 'loading' : 'reload'" :spin="updateChecking" /> Keresés</btn>
        </div>
        <p v-if="releaseListError" class="error" role="alert">Verziólista: {{ releaseListError }}</p>
        <p v-if="updateActionError" class="error" role="alert">{{ updateActionError }}</p>
        <div v-if="updateFailed" class="empty">A frissítések ellenőrzése nem sikerült (nincs internet vagy a GitHub nem elérhető).</div>
        <div v-else-if="!update" class="empty">Ellenőrzés...</div>
        <template v-else>
          <p v-if="system" class="hint">{{ system.os }}/{{ system.arch }} · {{ system.goVersion }} · adatbázis: {{ system.database }} · fut: {{ formatUptime(system.uptimeSeconds) }}</p>
          <p>Telepített verzió: <strong>{{ update.current }}</strong> · Kiválasztott: <strong>{{ update.latest }}</strong></p>
          <p v-if="update.updateAvailable" class="update-new">Új verzió érhető el.</p>
          <p v-else class="empty">A panel naprakész (vagy a verzió nem összehasonlítható).</p>
          <pre v-if="update.updateAvailable && update.notes" class="update-notes">{{ update.notes }}</pre>
          <div v-if="update.updateAvailable" class="quick-links">
            <a v-if="update.url" :href="update.url" target="_blank" rel="noopener noreferrer"><btn>Változások</btn></a>
            <btn v-if="update.canApply" color="primary" :disabled="update.running || updateApplying" @click="applyUpdate"><icon :name="update.running || updateApplying ? 'loading' : 'download'" :spin="update.running || updateApplying" /> {{ update.running || updateApplying ? 'Frissítés folyamatban' : 'Frissítés indítása' }}</btn>
            <small v-else>Az egykattintásos frissítéshez állítsd be a <code>panel.update.command</code> értéket a config fájlban.</small>
          </div>
          <div v-if="update.assets?.length" class="release-assets">
            <h3>Letölthető fájlok</h3>
            <a v-for="asset in update.assets" :key="asset.url" :href="asset.url" target="_blank" rel="noopener noreferrer" class="release-asset">
              <icon name="download" /><span>{{ asset.name }}</span><small>{{ formatAssetSize(asset.size) }}</small>
            </a>
          </div>
          <small v-if="update.lastResult">Utolsó frissítés: {{ update.lastResult }}</small>
        </template>
      </section>
      <div class="quick-links">
        <router-link to="/servers"><btn color="primary"><icon name="server" /> Összes szerver kezelése</btn></router-link>
        <router-link to="/users"><btn><icon name="users" /> Felhasználók kezelése</btn></router-link>
        <router-link to="/nodes"><btn><icon name="node" /> Node-ok és portok</btn></router-link>
        <router-link to="/templates"><btn><icon name="template" /> Sablonok és eggek</btn></router-link>
        <a href="/api/admin/activity/export"><btn><icon name="download" /> Műveleti napló CSV</btn></a>
        <a href="/api/admin/servers/export"><btn><icon name="download" /> Szerverlista CSV</btn></a>
      </div>
      <section>
        <h2>Panel-közlemények</h2>
        <p class="hint">Bejelentkezés után minden felhasználó látja. A bezárt közlemény módosítás után ismét megjelenik.</p>
        <div class="announcement-form">
          <input v-model="announcement.title" maxlength="140" placeholder="Cím" aria-label="Közlemény címe">
          <select v-model="announcement.level" aria-label="Közlemény típusa"><option value="info">Információ</option><option value="warning">Figyelmeztetés</option><option value="maintenance">Karbantartás</option></select>
          <textarea v-model="announcement.message" maxlength="4000" placeholder="Közlemény szövege" aria-label="Közlemény szövege" />
          <label>Megjelenés <input v-model="announcement.startsAt" type="datetime-local"></label>
          <label>Lejárat <input v-model="announcement.expiresAt" type="datetime-local"></label>
          <label><input v-model="announcement.active" type="checkbox"> Aktív</label>
          <btn color="primary" :disabled="announcementSaving || !announcement.title.trim() || !announcement.message.trim()" @click="saveAnnouncement">Közzététel</btn>
        </div>
        <div v-if="announcements.length === 0" class="empty">Nincs létrehozott közlemény.</div>
        <div v-for="item in announcements" v-else :key="item.id" class="announcement-row"><span><strong>{{ item.title }}</strong><small>{{ item.level }} · {{ item.active ? 'aktív' : 'inaktív' }}<template v-if="item.startsAt"> · kezdés: {{ new Date(item.startsAt).toLocaleString() }}</template><template v-if="item.expiresAt"> · lejárat: {{ new Date(item.expiresAt).toLocaleString() }}</template></small></span><btn variant="icon" tooltip="Törlés" @click="deleteAnnouncement(item)"><icon name="remove" /></btn></div>
      </section>
      <section>
        <h2>Szerver lejáratok</h2>
        <p class="hint">Lejárat után a nem admin felhasználók nem tudják elindítani a szervert. A lejárt szervereket a panel 10 percenként automatikusan leállítja.</p>
        <div class="announcement-form">
          <select v-model="expiryServer" aria-label="Szerver" @change="loadLimits"><option value="">Válassz szervert</option><option v-for="server in adminServers" :key="server.id" :value="server.id">{{ server.name }}</option></select>
          <input v-model="expiryDate" type="datetime-local" aria-label="Lejárat">
          <btn color="primary" :disabled="!expiryServer || !expiryDate" @click="saveExpiry(expiryServer, expiryDate)">Beállítás</btn>
          <input v-model="backupLimit" type="number" min="0" max="1000" aria-label="Mentési korlát (0 = nincs)" placeholder="Mentési korlát (0 = nincs)">
          <btn :disabled="!expiryServer" @click="saveBackupLimit">Mentési korlát beállítása</btn>
        </div>
        <btn v-if="expiredIds.length" color="error" :disabled="actionRunning" @click="stopExpired">Lejárt szerverek leállítása ({{ expiredIds.length }})</btn>
        <div v-if="expiring.length === 0" class="empty">Nincs beállított lejárat.</div>
        <div v-for="item in expiring" v-else :key="item.id" class="announcement-row"><span><strong>{{ item.name }}</strong><small>{{ new Date(item.expiresAt).toLocaleString() }} · {{ daysLeft(item.expiresAt) < 0 ? 'lejárt' : daysLeft(item.expiresAt) + ' nap' }}</small></span><btn variant="icon" tooltip="Törlés" @click="saveExpiry(item.id, '')"><icon name="remove" /></btn></div>
      </section>
      <section>
        <h2>Tömeges szerverműveletek</h2>
        <p class="hint">A betöltött szerverek közül választhatsz ki legfeljebb 25-öt. Minden végrehajtás előtt külön megerősítés szükséges.</p>
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

<style scoped lang="scss">
.admin-dashboard { max-width: 1200px; margin: 0 auto; }
.metrics { grid-template-columns: repeat(auto-fill, minmax(190px, 1fr)); }
.metric { border: 1px solid var(--color-background); box-shadow: 0 1px 3px rgba(0, 0, 0, .18); transition: transform .15s ease, box-shadow .15s ease; }
.metric:hover { transform: translateY(-2px); box-shadow: 0 6px 14px rgba(0, 0, 0, .22); }
.metric strong { color: var(--color-primary); }
section { border: 1px solid var(--color-background); box-shadow: 0 1px 3px rgba(0, 0, 0, .12); }
.update-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; }
.update-card.available { border-color: var(--color-primary); }
.update-head select { flex: 1 1 200px; min-width: 180px; max-width: 100%; padding: 8px 10px; color: var(--color-text); background: var(--color-background); border: 1px solid var(--color-background); border-radius: 5px; }
.update-new { color: var(--color-primary); font-weight: 600; }
.update-notes { max-height: 180px; overflow: auto; white-space: pre-wrap; padding: 10px; border-radius: 6px; background: var(--color-background); font-size: .85rem; }
.release-assets { display: grid; gap: 8px; margin-top: 14px; }
.release-assets h3 { margin: 0; }
.release-asset { display: flex; align-items: center; gap: 10px; padding: 10px 12px; color: var(--color-text); background: var(--color-background); border: 1px solid var(--color-background); border-radius: 6px; }
.release-asset span { flex: 1; overflow-wrap: anywhere; }
.release-asset small { white-space: nowrap; }
@media (max-width: 700px) { .heading { flex-direction: column; align-items: flex-start; } .announcement-form { grid-template-columns: 1fr; } }
</style>
