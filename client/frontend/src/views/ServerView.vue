<script setup>
import { shallowRef, inject, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import Loader from '@/components/ui/Loader.vue'
import PlayitAgent from '@/components/server/PlayitAgent.vue'

const api = inject('api')
const route = useRoute()

const server = shallowRef(null)
const serverComponent = shallowRef(Loader)
const loadError = shallowRef('')

onMounted(async () => {
  try {
    server.value = await api.server.get(route.params.id)
    serverComponent.value = (await import(`../components/serverTypes/${server.value.type}.vue`)).default
  } catch {
    if (server.value) {
      try {
        serverComponent.value = (await import('../components/serverTypes/generic.vue')).default
      } catch (error) {
        loadError.value = error.message || String(error)
      }
    } else {
      loadError.value = 'A szerver betöltése nem sikerült.'
    }
  }
})

onUnmounted(() => {
  server.value?.closeSocket?.()
})
</script>

<template>
  <div v-if="server != null" :class="['serverview', server ? server.type : '']">
    <component :is="serverComponent" :server="server" />
    <playit-agent v-if="$api.auth.hasScope('admin')" :server="server" />
  </div>
  <loader v-else-if="!loadError" />
  <p v-else class="load-error" role="alert">{{ loadError }}</p>
</template>
