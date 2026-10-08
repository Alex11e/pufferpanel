<script setup>
import { inject, onMounted, ref } from 'vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const events = inject('events')
const providers = ref([])
const connections = ref([])
const loading = ref(true)
const error = ref('')
const notice = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [providerResponse, connectionResponse] = await Promise.all([
      api.get('/auth/social/providers'),
      api.get('/api/self/social')
    ])
    providers.value = providerResponse.data || []
    connections.value = connectionResponse.data || []
  } catch (failure) {
    error.value = failure?.msg || failure?.message || 'A kapcsolt fiókok betöltése nem sikerült.'
  } finally {
    loading.value = false
  }
}

function disconnect(connection) {
  events.emit('confirm', `Leválasztod ezt a bejelentkezést: ${connection.provider}?`, {
    text: 'Leválasztás', icon: 'remove', color: 'error', action: async () => {
      try {
        await api.delete(`/api/self/social/${encodeURIComponent(connection.key)}`)
        await load()
      } catch (failure) {
        error.value = failure?.msg || failure?.message || 'A fiók leválasztása nem sikerült.'
      }
    }
  })
}

onMounted(() => {
  const query = new URLSearchParams(window.location.search)
  if (query.get('socialConnected') === '1') notice.value = 'A külső fiók sikeresen összekapcsolva.'
  if (query.get('socialError') === '1') notice.value = 'A külső fiók összekapcsolása nem sikerült.'
  load()
})
</script>

<template>
  <section class="social-connections">
    <h1>Kapcsolt bejelentkezések</h1>
    <p class="hint">Külső fiókok összekapcsolása vagy leválasztása.</p>
    <p v-if="notice" class="notice" role="status">{{ notice }}</p>
    <loader v-if="loading" />
    <p v-else-if="error" class="error" role="alert">{{ error }} <btn @click="load"><icon name="reload" /> Újrapróbálás</btn></p>
    <template v-else>
      <div v-if="!providers.length" class="empty">Jelenleg nincs elérhető külső bejelentkezés.</div>
      <div v-for="provider in providers" :key="provider.key" class="provider-row">
        <div><strong>{{ provider.name }}</strong><small>{{ connections.find(connection => connection.key === provider.key)?.displayName || connections.find(connection => connection.key === provider.key)?.email || 'Nincs összekapcsolva' }}</small></div>
        <btn v-if="connections.some(connection => connection.key === provider.key)" color="error" @click="disconnect(connections.find(connection => connection.key === provider.key))"><icon name="remove" /> Leválasztás</btn>
        <a v-else-if="provider.connectable" :href="`/api/self/social/${encodeURIComponent(provider.key)}/connect`"><btn color="primary"><icon name="account" /> Összekapcsolás</btn></a>
        <small v-else>Az összekapcsolást az admin letiltotta.</small>
      </div>
      <div v-for="connection in connections.filter(item => !providers.some(provider => provider.key === item.key))" :key="connection.key" class="provider-row">
        <div><strong>{{ connection.provider }}</strong><small>{{ connection.displayName || connection.email || 'Összekapcsolva, jelenleg letiltva' }}</small></div>
        <btn color="error" @click="disconnect(connection)"><icon name="remove" /> Leválasztás</btn>
      </div>
    </template>
  </section>
</template>

<style scoped lang="scss">
.social-connections { max-width: 900px; }
.hint, small { color: var(--color-text-secondary); }
.provider-row { display:flex; align-items:center; justify-content:space-between; gap:12px; padding:14px 0; border-bottom:1px solid var(--color-background-secondary); }
.provider-row small { display:block; }
.error { color:var(--color-error); }
.notice { color:var(--color-success); }
.empty { padding:16px 0; color:var(--color-text-secondary); }
</style>