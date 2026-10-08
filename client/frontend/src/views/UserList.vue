<script setup>
import { ref, computed, inject, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import md5 from 'js-md5'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import Btn from '@/components/ui/Btn.vue'
import TextField from '@/components/ui/TextField.vue'

const api = inject('api')
const { t } = useI18n()

const users = ref([])
let lastPage = 0
let loadingPage = false
const allUsersLoaded = ref(false)
const loaderRef = ref(null)
const firstEntry = ref(null)
const search = ref('')
const loadError = ref('')
const searchResults = ref(null)
const searching = ref(false)
const searchError = ref('')
const otpFilter = ref('')
const canViewOtp = computed(() => api.auth.hasScope('users.perms.view'))
const visibleUsers = computed(() => (search.value.trim() ? searchResults.value || [] : users.value).filter(user => {
  const query = search.value.trim().toLowerCase()
  const matchesSearch = !query || `${user.username} ${user.email}`.toLowerCase().includes(query)
  const matchesOtp = !canViewOtp.value || !otpFilter.value || Boolean(user.otpActive) === (otpFilter.value === 'enabled')
  return matchesSearch && matchesOtp
}))
let searchTimer = null
let searchRequestId = 0

function addUsers(newUsers) {
  newUsers.map(user => users.value.push(user))
}

function isLoaderVisible() {
  if (!loaderRef.value) return false
  const vw = window.innerWidth || document.documentElement.clientWidth
  const vh = window.innerHeight || document.documentElement.clientHeight
  const rect = loaderRef.value.$el.getBoundingClientRect()
  return rect.top >= 0 && rect.left >= 0 && rect.bottom <= vh && rect.right <= vw
}

async function loadPage(page = 1) {
  if (loadingPage) return
  loadingPage = true
  loadError.value = ''
  try {
    const data = await api.user.list(page)
    addUsers(data.users)
    lastPage = data.paging.page
    allUsersLoaded.value = data.paging.page * data.paging.pageSize >= (data.paging.total || 0)
  } catch (error) {
    loadError.value = error.msg || error.message || 'A felhasználók betöltése nem sikerült.'
  } finally {
    await nextTick()
    loadingPage = false
    if (!search.value.trim() && !loadError.value && !allUsersLoaded.value && isLoaderVisible()) loadPage(lastPage + 1)
  }
}

function onScroll() {
  if (!search.value.trim() && !loadingPage && !loadError.value && !allUsersLoaded.value && isLoaderVisible()) loadPage(lastPage + 1)
}

async function searchUsers(field, query, requestId) {
  const matches = []
  let page = 1
  let total = Infinity
  while (matches.length < total && requestId === searchRequestId) {
    const response = await api.get('/api/users', { [field]: `*${query}*`, page, limit: 100 })
    const data = response.data
    const pageUsers = data.users || []
    matches.push(...pageUsers)
    total = data.paging?.total ?? matches.length
    if (!pageUsers.length) break
    page += 1
  }
  return matches
}

watch(search, value => {
  const requestId = ++searchRequestId
  clearTimeout(searchTimer)
  searchError.value = ''
  const query = value.trim()
  if (!query) {
    searchResults.value = null
    searching.value = false
    nextTick(() => {
      if (!allUsersLoaded.value && isLoaderVisible()) loadPage(lastPage + 1)
    })
    return
  }

  searching.value = true
  searchResults.value = []
  searchTimer = setTimeout(async () => {
    try {
      const [usernameMatches, emailMatches] = await Promise.all([
        searchUsers('username', query, requestId),
        searchUsers('email', query, requestId)
      ])
      if (requestId !== searchRequestId) return
      const uniqueUsers = new Map([...usernameMatches, ...emailMatches].map(user => [user.id, user]))
      searchResults.value = [...uniqueUsers.values()].sort((a, b) => a.username.localeCompare(b.username))
    } catch (error) {
      if (requestId === searchRequestId) searchError.value = error?.msg || error?.message || 'A keresés nem sikerült.'
    } finally {
      if (requestId === searchRequestId) searching.value = false
    }
  }, 250)
})

onMounted(() => {
  nextTick(() => {
    loadPage()
    window.addEventListener('scroll', onScroll)
  })
})

onUnmounted(() => {
  window.removeEventListener('scroll', onScroll)
})

function setFirstEntry(ref) {
  if (!firstEntry.value) firstEntry.value = ref
}

function focusList() {
  firstEntry.value?.$el.focus()
}
</script>

<template>
  <div class="userlist">
    <h1 v-text="t('users.Users')" />
    <text-field v-model="search" :label="`${t('users.Username')} / ${t('users.Email')}`" icon="search" />
    <p v-if="searchError" class="load-error" role="alert">{{ searchError }}</p>
    <select v-if="canViewOtp" v-model="otpFilter" aria-label="Szűrés kétfaktoros hitelesítés szerint">
      <option value="">Minden 2FA állapot</option>
      <option value="enabled">2FA bekapcsolva</option>
      <option value="disabled">2FA nélkül</option>
    </select>
    <p v-if="loadError" class="load-error" role="alert">
      {{ loadError }}
      <btn :disabled="loadingPage" @click="loadPage(lastPage + 1)">Újrapróbálás</btn>
    </p>
    <div v-hotkey="'l'" class="list" @hotkey="focusList()">
      <div v-for="user in visibleUsers" :key="user.id" class="list-item">
        <router-link :ref="setFirstEntry" :to="{ name: 'UserView', params: { id: user.id } }">
          <div class="user">
            <img :src="'https://www.gravatar.com/avatar/' + md5(user.email) + '?d=mp'" class="avatar" />
            <div>
              <span class="title">{{user.username}}{{ $api.auth.hasScope('users.perms.view') && user.otpActive ? ' (' + t('users.OtpAbreviated') + ')' : '' }}</span>
              <span class="subline">{{user.email}}</span>
            </div>
          </div>
        </router-link>
      </div>
      <div v-if="searching" class="list-item"><loader small /></div>
      <div v-if="!search.trim() && !allUsersLoaded && !loadError" ref="loaderRef" class="list-item">
        <loader small />
      </div>
      <p v-if="((search.trim() && !searching) || allUsersLoaded) && visibleUsers.length === 0" class="list-item">Nincs a keresésnek megfelelő felhasználó.</p>
      <div v-if="$api.auth.hasScope('users.info.edit')" class="list-item">
        <router-link v-hotkey="'c'" :to="{ name: 'UserCreate' }">
          <div class="createLink"><icon name="plus" />{{ t('users.Add') }}</div>
        </router-link>
      </div>
    </div>
  </div>
</template>
