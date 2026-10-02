<script setup>
import { ref, inject, onMounted, onUnmounted } from 'vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const props = defineProps({
  server: { type: Object, required: true }
})

const events = inject('events')
const status = ref(null)
const busy = ref(false)
let timer = null

async function refresh() {
  try {
    status.value = await props.server.getStatus()
  } catch {
    status.value = null
  }
}

async function power(action) {
  busy.value = true
  try {
    await props.server[action]()
    await refresh()
  } finally {
    busy.value = false
  }
}

function confirmKill() {
  events.emit('confirm', 'Kényszerített leállítás? A folyamatok azonnal megszakadnak.', {
    text: 'Kill', icon: 'stop', color: 'error', action: () => power('kill')
  })
}

onMounted(() => {
  refresh()
  timer = setInterval(refresh, 10000)
})

onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div>
    <loader v-if="status === null" small />
    <template v-else>
      <p>Állapot: <strong :class="['status', status]">{{ { online: 'Fut', offline: 'Leállítva', installing: 'Telepítés' }[status] || status }}</strong></p>
      <div class="actions">
        <btn v-if="status === 'offline' && server.hasScope('server.start')" color="primary" :disabled="busy" @click="power('start')"><icon name="play" /> Indítás</btn>
        <template v-if="status === 'online'">
          <btn v-if="server.hasScope('server.start')" :disabled="busy" @click="power('restart')"><icon name="reload" /> Újraindítás</btn>
          <btn v-if="server.hasScope('server.stop')" :disabled="busy" @click="power('stop')"><icon name="stop" /> Leállítás</btn>
          <btn v-if="server.hasScope('server.stop')" color="error" :disabled="busy" @click="confirmKill"><icon name="stop" /> Kill</btn>
        </template>
      </div>
    </template>
  </div>
</template>

<style scoped lang="scss">
.actions { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 16px; }
.status.online { color: var(--color-success); }
.status.installing { color: var(--color-warning); }
</style>
