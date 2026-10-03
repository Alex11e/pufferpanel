<script setup>
import { inject, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const route = useRoute()
const router = useRouter()
const status = ref('checking')
const error = ref('')
const purchase = ref(null)
let pollTimer = null
let attempts = 0
let provisioning = false

function retry() {
  clearTimeout(pollTimer)
  attempts = 0
  continuePurchase()
}

async function continuePurchase() {
  if (provisioning) return
  const id = route.query.purchaseId
  if (!id) {
    status.value = 'error'
    error.value = 'Hiányzik a vásárlás azonosítója.'
    return
  }
  status.value = 'checking'
  error.value = ''
  try {
    const response = await api.get(`/api/billing/purchases/${encodeURIComponent(id)}`)
    purchase.value = response.data
    if (purchase.value.serverId) {
      router.replace({ name: 'ServerView', params: { id: purchase.value.serverId }, query: { created: true } })
      return
    }
    if (purchase.value.status === 'pending' && attempts++ < 45) {
      pollTimer = setTimeout(continuePurchase, 2000)
      return
    }
    if (purchase.value.status !== 'paid') {
      status.value = 'error'
      error.value = purchase.value.status === 'failed' ? 'A fizetés nem sikerült vagy lejárt.' : 'A fizetés visszaigazolása még nem érkezett meg.'
      return
    }
    provisioning = true
    status.value = 'provisioning'
    const definition = await api.get(`/api/billing/purchases/${encodeURIComponent(id)}/provision`)
    const serverId = await api.server.create(definition.data)
    router.replace({ name: 'ServerView', params: { id: serverId }, query: { created: true } })
  } catch (failure) {
    status.value = 'error'
    error.value = failure?.msg || failure?.message || 'A vásárlás folytatása nem sikerült.'
  } finally {
    provisioning = false
  }
}

onMounted(continuePurchase)
onUnmounted(() => clearTimeout(pollTimer))
</script>

<template>
  <div class="billing-complete">
    <loader v-if="status === 'checking' || status === 'provisioning'" />
    <template v-else-if="status === 'error'">
      <h1><icon name="error" /> A rendelés nem fejeződött be</h1>
      <p class="error" role="alert">{{ error }}</p>
      <btn color="primary" @click="retry"><icon name="reload" /> Újraellenőrzés</btn>
      <btn @click="router.push({ name: 'ServerCreate' })">Vissza a szerverkészítéshez</btn>
    </template>
  </div>
</template>

<style scoped lang="scss">
.billing-complete { max-width: 620px; display: flex; flex-direction: column; gap: 12px; }
h1 { display: flex; align-items: center; gap: 10px; }
.error { color: var(--color-error); }
</style>
