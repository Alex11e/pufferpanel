<script setup>
import { ref, computed, inject, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import Btn from '@/components/ui/Btn.vue'
import TextField from '@/components/ui/TextField.vue'
import { serverIconName } from '@/utils/serverIcon'

const api = inject('api')
const toast = inject('toast')
const { t, tm, rt } = useI18n()

function readStorage(key) {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function readListPreferences() {
  try {
    const stored = JSON.parse(readStorage('serverListPreferences') || '{}')
    return stored && typeof stored === 'object' && !Array.isArray(stored) ? stored : {}
  } catch {
    return {}
  }
}

function readFavorites() {
  try {
    const stored = JSON.parse(readStorage('favoriteServers') || '[]')
    return Array.isArray(stored) ? stored : []
  } catch {
    return []
  }
}

const listPreferences = readListPreferences()
const servers = ref([])
let lastPage = 0
let loadingPage = false
const allServersLoaded = ref(false)
const loaderRef = ref(null)
const firstEntry = ref(null)
const loadError = ref('')
const folderError = ref('')
const search = ref(typeof listPreferences.search === 'string' ? listPreferences.search : '')
const selectedTag = ref(typeof listPreferences.tag === 'string' ? listPreferences.tag : '')
const selectedFolder = ref(typeof listPreferences.folder === 'string' ? listPreferences.folder : '')
const selectedStatus = ref(['', 'online', 'offline'].includes(listPreferences.status) ? listPreferences.status : '')
const sortMode = ref(['name', 'status', 'expiry'].includes(listPreferences.sort) ? listPreferences.sort : 'name')
const favoritesOnly = ref(listPreferences.favoritesOnly === true)
const expiringOnly = ref(listPreferences.expiringOnly === true)
const favorites = ref(readFavorites())
const recentActivity = ref([])
const folders = ref([])
const newFolder = ref('')
let interval = null
let statusRefreshRunning = false
let statusRefreshQueued = false

watch(
  () => [search.value, selectedTag.value, selectedFolder.value, selectedStatus.value, sortMode.value, favoritesOnly.value, expiringOnly.value],
  () => {
    try {
      localStorage.setItem('serverListPreferences', JSON.stringify({
        search: search.value,
        tag: selectedTag.value,
        folder: selectedFolder.value,
        status: selectedStatus.value,
        sort: sortMode.value,
        favoritesOnly: favoritesOnly.value,
        expiringOnly: expiringOnly.value
      }))
    } catch {
      // The list remains usable when persistent browser storage is unavailable.
    }
  }
)

const tags = computed(() => [...new Set(servers.value.flatMap(server => (server.tags || '').split(',').map(tag => tag.trim()).filter(Boolean)))].sort())
const summary = computed(() => ({ total: servers.value.length, online: servers.value.filter(server => server.online === 'online').length, offline: servers.value.filter(server => server.online === 'offline').length, favorites: favorites.value.length }))
function normalizeSearchText(value) {
  return String(value ?? '').normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase()
}

const visibleServers = computed(() => {
  const query = normalizeSearchText(search.value)
  const expiryDeadline = Date.now() + 7 * 24 * 60 * 60 * 1000
  const filtered = servers.value.filter(server => {
    const folderName = folders.value.find(folder => folder.serverIds.includes(server.id))?.name || ''
    const text = normalizeSearchText(`${server.name} ${server.id} ${server.type} ${server.tags || ''} ${server.node?.name || ''} ${server.ip || ''} ${server.port || ''} ${server.subdomain || ''} ${server.node?.publicHost || ''} ${getServerAddress(server)} ${folderName}`)
    const expiryTime = Date.parse(server.expiresAt || '')
    const expiresSoon = Number.isFinite(expiryTime) && expiryTime <= expiryDeadline
    return (!favoritesOnly.value || favorites.value.includes(server.id)) && (!expiringOnly.value || expiresSoon) && (!query || text.includes(query)) && (!selectedTag.value || (server.tags || '').split(',').map(tag => tag.trim()).includes(selectedTag.value)) && (!selectedFolder.value || folders.value.find(folder => String(folder.id) === selectedFolder.value)?.serverIds.includes(server.id)) && (!selectedStatus.value || server.online === selectedStatus.value)
  })
  return filtered.sort((a, b) => {
    const favoriteOrder = Number(favorites.value.includes(b.id)) - Number(favorites.value.includes(a.id))
    if (favoriteOrder) return favoriteOrder

    if (sortMode.value === 'status') {
      const statusOrder = { offline: 0, installing: 1, online: 2 }
      const statusDifference = (statusOrder[a.online] ?? 3) - (statusOrder[b.online] ?? 3)
      if (statusDifference) return statusDifference
    }

    if (sortMode.value === 'expiry') {
      const expiryA = Date.parse(a.expiresAt || '')
      const expiryB = Date.parse(b.expiresAt || '')
      const timeA = Number.isFinite(expiryA) ? expiryA : Number.POSITIVE_INFINITY
      const timeB = Number.isFinite(expiryB) ? expiryB : Number.POSITIVE_INFINITY
      if (timeA !== timeB) return timeA - timeB
    }

    return a.name.localeCompare(b.name)
  })
})

async function addServers(newServers) {
  newServers.map(server => servers.value.push(server))
  await assignDefaultFolders(newServers)
  refreshServerStatus()
}

async function assignDefaultFolders(newServers) {
  for (const server of newServers) {
    const folderName = String(server.type || 'generic').slice(0, 60)
    let folder = folders.value.find(item => item.name === folderName)
    try {
      if (!folder) {
        const response = await api.post('/api/folders', { name: folderName, color: '#4f7cff' })
        folder = response.data
        folders.value.push(folder)
      }
      if (folders.value.some(item => item.serverIds.includes(server.id))) continue
      await api.put(`/api/folders/${folder.id}/servers/${server.id}`)
      folder.serverIds.push(server.id)
    } catch (error) {
      folderError.value = error.message || String(error)
    }
  }
}

async function refreshServerStatus() {
  if (statusRefreshRunning) {
    statusRefreshQueued = true
    return
  }

  statusRefreshRunning = true
  try {
    do {
      statusRefreshQueued = false
      await Promise.all(servers.value.filter(server => server.canGetStatus).map(async server => {
        const hasKnownStatus = Boolean(server.online && server.online !== 'loading')
        if (!hasKnownStatus) server.online = 'loading'
        try {
          server.online = await api.server.getStatus(server.id)
        } catch {
          if (!hasKnownStatus) server.online = undefined
        }
      }))
    } while (statusRefreshQueued)
  } finally {
    statusRefreshRunning = false
  }
}

function isLoaderVisible() {
  if (!loaderRef.value) return false
  const vw = window.innerWidth || document.documentElement.clientWidth
  const vh = window.innerHeight || document.documentElement.clientHeight
  const rect = loaderRef.value.$el.getBoundingClientRect()
  return rect.top >= 0 && rect.left >= 0 && rect.bottom <= vh && rect.right <= vw
}

async function loadPage(page = 1) {
  loadingPage = true
  try {
    const data = await api.server.list(page)
    await addServers(data.servers)
    lastPage = data.paging.page
    allServersLoaded.value = data.paging.page * data.paging.pageSize >= (data.paging.total || 0)
  } catch (error) {
    loadError.value = error.message || String(error)
  } finally {
    nextTick(() => {
      loadingPage = false
      if (!loadError.value && !allServersLoaded.value && isLoaderVisible()) loadPage(lastPage + 1)
    })
  }
}

function onScroll() {
  if (!loadingPage && !loadError.value && isLoaderVisible()) loadPage(lastPage + 1)
}

onMounted(() => {
  interval = setInterval(refreshServerStatus, 30 * 1000)
  nextTick(async () => {
    await loadFolders()
    loadPage()
    if (api.auth.hasScope('admin')) api.server.getRecentActivity().then(records => { recentActivity.value = records })
    window.addEventListener('scroll', onScroll)
  })
})

async function loadFolders() {
  try { const response = await api.get('/api/folders'); folders.value = response.data || [] } catch { folders.value = [] }
}

async function createFolder() {
  if (!newFolder.value.trim()) return
  await api.post('/api/folders', { name: newFolder.value, color: '#4f7cff' })
  newFolder.value = ''
  await loadFolders()
}

function folderForServer(serverId) {
  return folders.value.find(folder => folder.serverIds.includes(serverId))?.id || ''
}

async function setServerFolder(serverId, event) {
  const folderId = event.target.value
  const existing = folders.value.find(folder => folder.serverIds.includes(serverId))
  if (existing) await api.delete(`/api/folders/${existing.id}/servers/${serverId}`)
  if (folderId) await api.put(`/api/folders/${folderId}/servers/${serverId}`)
  await loadFolders()
}

onUnmounted(() => {
  clearInterval(interval)
  window.removeEventListener('scroll', onScroll)
})

function expiryStyle(date) {
  const days = (new Date(date) - Date.now()) / 86400000
  if (days < 0) return { color: 'var(--color-error)' }
  if (days < 7) return { color: 'var(--color-warning, orange)' }
  return {}
}

function getServerAddress(server) {
	if (server.subdomain) return server.subdomain
  let ip = server.node.publicHost
  if (server.ip && server.ip !== '0.0.0.0') {
    ip = server.ip
  }
  return ip + (server.port ? ':' + server.port : '')
}

function setFirstEntry(ref) {
  if (!firstEntry.value) firstEntry.value = ref
}

function focusList() {
  firstEntry.value.$el.focus()
}

function toggleFavorite(id) {
  favorites.value = favorites.value.includes(id) ? favorites.value.filter(item => item !== id) : [...favorites.value, id]
  try {
    localStorage.setItem('favoriteServers', JSON.stringify(favorites.value))
  } catch {
    // Keep the favorite for this page session when persistent storage is unavailable.
  }
}

function clearFilters() {
  search.value = ''
  selectedTag.value = ''
  selectedFolder.value = ''
  selectedStatus.value = ''
  favoritesOnly.value = false
  expiringOnly.value = false
  sortMode.value = 'name'
}

function activityLabel(action) {
  const activity = tm('servers.activity')
  const message = activity?.[action]
  return typeof message === 'string' ? rt(message) : action
}

async function copyAddress(server) {
  try {
    await navigator.clipboard.writeText(getServerAddress(server))
    toast.success('A csatlakozási cím a vágólapra került.')
  } catch { toast.error('A csatlakozási cím másolása nem sikerült.') }
}
</script>

<template>
  <div class="serverlist">
    <h1 v-text="t('servers.Servers')" />
    <p v-if="loadError" class="load-error" role="alert">{{ loadError }}</p>
    <p v-if="folderError" class="load-error" role="alert">{{ folderError }}</p>
    <div class="server-dashboard">
      <div class="metric"><span>{{ summary.total }}</span>{{ t('servers.TotalServers') }}</div>
      <div class="metric online"><span>{{ summary.online }}</span>{{ t('common.Online') }}</div>
      <div class="metric offline"><span>{{ summary.offline }}</span>{{ t('common.Offline') }}</div>
      <div class="metric"><span>{{ summary.favorites }}</span>{{ t('servers.Favorites') }}</div>
    </div>
    <div class="status-filter"><btn :color="selectedStatus === '' ? 'primary' : undefined" @click="selectedStatus = ''">Minden állapot</btn><btn :color="selectedStatus === 'online' ? 'primary' : undefined" @click="selectedStatus = 'online'">Online</btn><btn :color="selectedStatus === 'offline' ? 'primary' : undefined" @click="selectedStatus = 'offline'">Offline</btn><btn :color="favoritesOnly ? 'primary' : undefined" @click="favoritesOnly = !favoritesOnly"><icon :name="favoritesOnly ? 'star' : 'star-outline'" /> {{ t('servers.FavoritesOnly') }}</btn><btn :color="expiringOnly ? 'primary' : undefined" @click="expiringOnly = !expiringOnly">{{ t('servers.ExpiringOnly') }}</btn><select v-model="sortMode" :aria-label="t('servers.SortBy')"><option value="name">{{ t('servers.SortName') }}</option><option value="status">{{ t('servers.SortStatus') }}</option><option value="expiry">{{ t('servers.SortExpiry') }}</option></select></div>
    <text-field v-model="search" :label="t('servers.SearchServers')" icon="search" />
    <div class="folder-tools">
      <select v-model="selectedFolder" aria-label="Szervermappa szűrése"><option value="">Minden szervermappa</option><option v-for="folder in folders" :key="folder.id" :value="String(folder.id)">{{ folder.name }}</option></select>
      <input v-model="newFolder" maxlength="60" placeholder="Új szervermappa" @keyup.enter="createFolder">
      <btn :disabled="!newFolder.trim()" @click="createFolder">Mappa létrehozása</btn>
    </div>
    <div v-if="tags.length" class="tag-filter">
      <btn :color="selectedTag === '' ? 'primary' : undefined" @click="selectedTag = ''">{{ t('servers.AllTags') }}</btn>
      <btn v-for="tag in tags" :key="tag" :color="selectedTag === tag ? 'primary' : undefined" @click="selectedTag = selectedTag === tag ? '' : tag">{{ tag }}</btn>
    </div>
    <div v-hotkey="'l'" class="list" @hotkey="focusList()">
      <div v-if="allServersLoaded && servers.length === 0" class="empty-results">{{ t('servers.NoServers') }}</div>
      <div v-else-if="allServersLoaded && visibleServers.length === 0" class="empty-results"><span>{{ t('servers.NoServersMatch') }}</span><btn variant="text" @click="clearFilters">{{ t('servers.ClearFilters') }}</btn></div>
      <div v-for="server in visibleServers" :key="server.id" :class="['list-item', 'server-wrapper', `server-wrapper-${server.type || 'none'}`]">
        <btn class="favorite" variant="icon" :tooltip="t('servers.ToggleFavorite')" @click="toggleFavorite(server.id)"><icon :name="favorites.includes(server.id) ? 'star' : 'star-outline'" /></btn>
        <div class="folder-picker" @click.stop><select :value="folderForServer(server.id)" aria-label="Szerver mappája" @change="setServerFolder(server.id, $event)"><option value="">Nincs mappa</option><option v-for="folder in folders" :key="folder.id" :value="folder.id">{{ folder.name }}</option></select></div>
        <btn class="copy-address" variant="icon" tooltip="Csatlakozási cím másolása" @click="copyAddress(server)"><icon name="content-copy" /></btn>
        <router-link :ref="setFirstEntry" :to="{ name: 'ServerView', params: { id: server.id } }">
          <div
            class="server server-custom-icon"
            :data-online="server.online"
          >
            <div class="server-title"><icon class="configured-icon" :name="serverIconName(server)" /><span class="title" :title="server.name">{{ server.name }}</span></div>
            <span class="type">{{server.type}}</span>
            <span class="subline">{{getServerAddress(server)}} @ {{server.node.name}}</span>
            <span v-if="server.expiresAt" class="subline" :style="expiryStyle(server.expiresAt)">{{ new Date(server.expiresAt) < new Date() ? 'Lejárt' : 'Lejárat' }}: {{ new Date(server.expiresAt).toLocaleDateString() }}</span>
          </div>
        </router-link>
      </div>
      <div v-if="!allServersLoaded" class="list-item">
        <loader ref="loaderRef" small />
      </div>
      <div v-if="$api.auth.hasScope('server.create')" class="list-item">
        <router-link v-hotkey="'c'" :to="{ name: 'ServerCreate' }">
          <div class="createLink"><icon name="plus" />{{ t('servers.Add') }}</div>
        </router-link>
      </div>
    </div>
    <section v-if="recentActivity.length" class="recent-activity">
      <h2 v-text="t('servers.RecentActivity')" />
      <div v-for="record in recentActivity" :key="record.id" class="activity-row">
        <icon name="stats" />
        <span>{{ record.username }} · {{ activityLabel(record.action) }}</span>
        <small>{{ record.serverId }} · {{ record.ipAddress }}</small>
      </div>
    </section>
  </div>
</template>

<style scoped lang="scss">
.server-dashboard { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-bottom: 18px; }
.metric { padding: 14px; border-radius: 8px; background: var(--color-background-secondary); color: var(--color-text-secondary); }
.metric span { display: block; font-size: 1.6rem; font-weight: 700; color: var(--color-text); }
.metric.online span { color: var(--color-success); }
.metric.offline span { color: var(--color-error); }
.tag-filter, .folder-tools, .status-filter { display: flex; flex-wrap: wrap; gap: 8px; margin: 8px 0 18px; }
.folder-tools input, .folder-tools select, .folder-picker select, .status-filter select { background:var(--color-background-secondary); color:var(--color-text); border:1px solid var(--color-background-secondary); border-radius:5px; padding:8px; }
.server-wrapper { position: relative; }
.server-title { display: flex; align-items: center; gap: .55rem; min-width: 0; padding-right: 12rem; }
.server-title .title { min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.configured-icon { flex: 0 0 auto; font-size: 1.35rem; }
.favorite { position: absolute; top: 8px; right: 8px; z-index: 2; }
.folder-picker { position:absolute; right:46px; top:9px; z-index:2; max-width:140px; }
.folder-picker select { max-width:140px; padding:4px; font-size:.8rem; }
.copy-address { position:absolute; right:8px; bottom:8px; z-index:2; }
.recent-activity { margin-top: 28px; }
.empty-results { display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:10px; padding:18px 12px; color:var(--color-text-secondary); }
.activity-row { display: flex; align-items: center; gap: 10px; padding: 10px 0; border-bottom: 1px solid var(--color-background-secondary); }
.activity-row small { margin-left: auto; color: var(--color-text-secondary); }
@media (max-width: 640px) { .server-dashboard { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
