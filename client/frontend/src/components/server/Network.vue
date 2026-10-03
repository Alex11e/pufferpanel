<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const props = defineProps({ server: { type: Object, required: true } })
const api = inject('api')
const toast = inject('toast')
const { t } = useI18n()
const allocations = ref([])
const requestedPort = ref('')
const guestPort = ref('')
const protocols = ref('tcp,udp')
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const canEditPorts = computed(() => api.auth.hasScope('nodes.edit') || api.auth.hasScope('admin'))
const isVps = computed(() => props.server.type === 'hypervm')
const canAllocatePort = computed(() => {
  if (busy.value || loading.value) return false
  const port = Number(isVps.value ? guestPort.value : requestedPort.value)
  return isVps.value ? Number.isInteger(port) && port >= 1 && port <= 65535 : !requestedPort.value || Number.isInteger(port) && port >= 1 && port <= 65535
})

onMounted(refresh)

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    allocations.value = await api.server.allocations(props.server.id)
  } catch (cause) {
    error.value = cause?.msg || cause?.message || t('servers.PortAllocationFailed')
  } finally {
    loading.value = false
  }
}

async function allocatePort() {
  busy.value = true
  error.value = ''
  try {
    const request = isVps.value
      ? { targetPort: Number(guestPort.value), protocols: protocols.value }
      : { port: requestedPort.value ? Number(requestedPort.value) : 0 }
    const allocation = await api.server.allocatePort(props.server.id, request)
    allocations.value = [...allocations.value, allocation].sort((a, b) => a.port - b.port)
    requestedPort.value = ''
    guestPort.value = ''
    toast.success(t('servers.PortAllocated', { port: allocation.port }))
  } catch (cause) {
    error.value = cause?.msg || cause?.message || t('servers.PortAllocationFailed')
  } finally {
    busy.value = false
  }
}

async function releasePort(allocation) {
  busy.value = true
  error.value = ''
  try {
    await api.server.releasePort(props.server.id, allocation.id)
    allocations.value = allocations.value.filter(item => item.id !== allocation.id)
    toast.success(t('servers.PortReleased', { port: allocation.port }))
  } catch (cause) {
    error.value = cause?.msg || cause?.message || t('servers.PortAllocationFailed')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="network">
    <h2 v-text="t('servers.Network')" />
    <div class="primary-port">
      <strong v-text="t('servers.PrimaryPort')" />
      <span>{{ server.node?.publicHost || server.ip }}<template v-if="server.port">:{{ server.port }}</template></span>
    </div>

    <div class="allocation-heading">
      <h3 v-text="t('servers.AllocatedPorts')" />
    </div>
    <div v-if="canEditPorts" class="allocation-form">
      <template v-if="isVps">
        <label>{{ t('servers.GuestPort') }}<input v-model="guestPort" type="number" min="1" max="65535"></label>
        <label>{{ t('servers.Protocol') }}<select v-model="protocols"><option value="tcp,udp">TCP + UDP</option><option value="tcp">TCP</option><option value="udp">UDP</option></select></label>
      </template>
      <label v-else>{{ t('servers.PortNumber') }}<input v-model="requestedPort" type="number" :min="server.node?.portRangeStart || 1" :max="server.node?.portRangeEnd || 65535"></label>
      <btn color="primary" :disabled="!canAllocatePort" @click="allocatePort">
        <icon :name="busy ? 'loading' : 'plus'" :spin="busy" /> {{ t(isVps ? 'servers.AddForward' : 'servers.AllocatePort') }}
      </btn>
    </div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <loader v-if="loading" small />
    <div v-else-if="allocations.length" class="allocation-list">
      <div v-for="allocation in allocations" :key="allocation.id" class="allocation">
        <span class="port">{{ server.node?.publicHost }}:{{ allocation.port }}</span>
        <span class="details">
          {{ allocation.protocols.toUpperCase() }}
          <template v-if="allocation.targetPort"> · {{ t('servers.GuestPort') }} {{ allocation.targetPort }}</template>
          <template v-if="allocation.purpose"> · {{ t('servers.PortPurpose', { purpose: allocation.purpose }) }}</template>
        </span>
        <btn v-if="canEditPorts && allocation.port !== server.port && !(isVps && allocation.purpose === 'vnc')" variant="icon" :tooltip="t('nodes.ReleasePort')" :disabled="busy" @click="releasePort(allocation)">
          <icon name="remove" />
        </btn>
      </div>
    </div>
    <p v-else class="empty" v-text="t('servers.NoAllocatedPorts')" />
  </section>
</template>

<style scoped>
.network { max-width: 52rem; }
.primary-port { display: flex; justify-content: space-between; gap: 1rem; padding: .8rem 0; border-bottom: 1px solid var(--color-background-secondary); }
.allocation-heading { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin-top: 1.25rem; }
.allocation-heading h3 { margin: 0; }
.allocation-form { display: flex; align-items: flex-end; gap: .75rem; flex-wrap: wrap; margin-top: .75rem; }
.allocation-form label { display: grid; gap: .35rem; color: var(--color-text-secondary); }
.allocation-form input, .allocation-form select { min-width: 10rem; padding: .6rem; border: 1px solid var(--color-background); border-radius: .3rem; background: var(--color-background-secondary); color: var(--color-text); }
.allocation-list { display: grid; gap: .5rem; margin-top: .75rem; }
.allocation { display: flex; align-items: center; gap: .75rem; padding: .7rem .9rem; background: var(--color-background-secondary); border-radius: .4rem; }
.port { font-weight: 600; overflow-wrap: anywhere; }
.details { flex: 1; color: var(--color-text-secondary); }
.empty { color: var(--color-text-secondary); }
.error { color: var(--color-error); }
</style>