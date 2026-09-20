<script setup>
import { ref, computed, inject, onMounted, onUnmounted, nextTick } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import Btn from '@/components/ui/Btn.vue'
import TextField from '@/components/ui/TextField.vue'

const api = inject('api')
const { t } = useI18n()

const servers = ref([])
let lastPage = 0
let loadingPage = false
const allServersLoaded = ref(false)
const loaderRef = ref(null)
const firstEntry = ref(null)
const search = ref('')
const selectedTag = ref('')
const favorites = ref(JSON.parse(localStorage.getItem('favoriteServers') || '[]'))
const recentActivity = ref([])
let interval = null

const tags = computed(() => [...new Set(servers.value.flatMap(server => (server.tags || '').split(',').map(tag => tag.trim()).filter(Boolean)))].sort())
const summary = computed(() => ({ total: servers.value.length, online: servers.value.filter(server => server.online === 'online').length, offline: servers.value.filter(server => server.online === 'offline').length, favorites: favorites.value.length }))
const visibleServers = computed(() => servers.value.filter(server => {
  const text = `${server.name} ${server.type} ${server.tags || ''} ${server.node?.name || ''}`.toLowerCase()
  return (!search.value || text.includes(search.value.toLowerCase())) && (!selectedTag.value || (server.tags || '').split(',').map(tag => tag.trim()).includes(selectedTag.value))
}).sort((a, b) => Number(favorites.value.includes(b.id)) - Number(favorites.value.includes(a.id)) || a.name.localeCompare(b.name)))

function addServers(newServers) {
  newServers.map(server => servers.value.push(server))
  refreshServerStatus()
}

async function refreshServerStatus() {
  servers.value.map(async s => {
    if (s.canGetStatus) {
      s.online = 'loading'
      try {
        s.online = await api.server.getStatus(s.id)
      } catch {
        s.online = undefined
      }
    }
  })
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
  const data = await api.server.list(page)
  addServers(data.servers)
  lastPage = data.paging.page
  allServersLoaded.value = data.paging.page * data.paging.pageSize >= (data.paging.total || 0)
  nextTick(() => {
    loadingPage = false
    if (!allServersLoaded.value && isLoaderVisible()) loadPage(lastPage + 1)
  })
}

function onScroll() {
  if (!loadingPage && isLoaderVisible()) loadPage(lastPage + 1)
}

onMounted(() => {
  interval = setInterval(refreshServerStatus, 30 * 1000)
  nextTick(() => {
    loadPage()
    if (api.auth.hasScope('admin')) api.server.getRecentActivity().then(records => { recentActivity.value = records })
    window.addEventListener('scroll', onScroll)
  })
})

onUnmounted(() => {
  clearInterval(interval)
  window.removeEventListener('scroll', onScroll)
})

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
  localStorage.setItem('favoriteServers', JSON.stringify(favorites.value))
}

function activityLabel(action) { return t(`servers.activity.${action}`) }
</script>

<template>
  <div class="serverlist">
    <h1 v-text="t('servers.Servers')" />
    <div class="server-dashboard">
      <div class="metric"><span>{{ summary.total }}</span>{{ t('servers.TotalServers') }}</div>
      <div class="metric online"><span>{{ summary.online }}</span>{{ t('common.Online') }}</div>
      <div class="metric offline"><span>{{ summary.offline }}</span>{{ t('common.Offline') }}</div>
      <div class="metric"><span>{{ summary.favorites }}</span>{{ t('servers.Favorites') }}</div>
    </div>
    <text-field v-model="search" :label="t('servers.SearchServers')" icon="search" />
    <div v-if="tags.length" class="tag-filter">
      <btn :color="selectedTag === '' ? 'primary' : undefined" @click="selectedTag = ''">{{ t('servers.AllTags') }}</btn>
      <btn v-for="tag in tags" :key="tag" :color="selectedTag === tag ? 'primary' : undefined" @click="selectedTag = selectedTag === tag ? '' : tag">{{ tag }}</btn>
    </div>
    <div v-hotkey="'l'" class="list" @hotkey="focusList()">
      <div v-for="server in visibleServers" :key="server.id" :class="['list-item', 'server-wrapper', `server-wrapper-${(server.icon || 'none')}`]">
        <btn class="favorite" variant="icon" :tooltip="t('servers.ToggleFavorite')" @click="toggleFavorite(server.id)"><icon :name="favorites.includes(server.id) ? 'star' : 'star-outline'" /></btn>
        <router-link :ref="setFirstEntry" :to="{ name: 'ServerView', params: { id: server.id } }">
          <div
            :class="['server', `server-${(server.icon || 'none')}`]"
            :data-online="server.online"
          >
            <span class="title" :title="server.name">{{server.name}}</span>
            <span class="type">{{server.type}}</span>
            <span class="subline">{{getServerAddress(server)}} @ {{server.node.name}}</span>
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
.tag-filter { display: flex; flex-wrap: wrap; gap: 8px; margin: 8px 0 18px; }
.server-wrapper { position: relative; }
.favorite { position: absolute; top: 8px; right: 8px; z-index: 2; }
.recent-activity { margin-top: 28px; }
.activity-row { display: flex; align-items: center; gap: 10px; padding: 10px 0; border-bottom: 1px solid var(--color-background-secondary); }
.activity-row small { margin-left: auto; color: var(--color-text-secondary); }
@media (max-width: 640px) { .server-dashboard { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
