<script setup>
import { computed, inject, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const router = useRouter()
const loading = ref(true)
const submitting = ref(false)
const error = ref('')
const options = ref({ enabled: false, providers: [] })
const plans = ref([])
const nodes = ref([])
const checkoutRequestKey = 'billingCheckoutRequest'
const loadServerRequest = () => {
  const fromHistory = history.state?.serverRequest
  if (fromHistory) {
    sessionStorage.setItem(checkoutRequestKey, JSON.stringify(fromHistory))
    return fromHistory
  }

  try {
    const stored = sessionStorage.getItem(checkoutRequestKey)
    return stored ? JSON.parse(stored) : null
  } catch {
    return null
  }
}
const serverRequest = ref(loadServerRequest())
const planId = ref('')
const cycle = ref('')
const autoRenew = ref(false)
const provider = ref('')
const plan = computed(() => plans.value.find(item => String(item.id) === String(planId.value)) || null)
const node = computed(() => nodes.value.find(item => item.id === serverRequest.value?.node) || null)

function cycleAvailable(value) {
  if (!plan.value) return false
  if (value === 'once') return plan.value.allowOneTime
  if (value === 'month') return plan.value.allowMonthly
  if (value === 'year') return plan.value.allowYearly
  return false
}

function priceFor(value) {
  if (!plan.value) return 0
  if (value === 'once') return plan.value.oneTimePriceMinor
  if (value === 'month') return plan.value.monthlyPriceMinor
  if (value === 'year') return plan.value.yearlyPriceMinor
  return 0
}

function formatPrice(value) {
  const decimals = plan.value?.currency === 'HUF' ? 0 : 2
  return `${(Number(value || 0) / (10 ** decimals)).toFixed(decimals)} ${plan.value?.currency || ''}`
}

function setDefaultCycle() {
  if (cycleAvailable('once')) cycle.value = 'once'
  else if (cycleAvailable('month')) cycle.value = 'month'
  else if (cycleAvailable('year')) cycle.value = 'year'
  else cycle.value = ''
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const request = loadServerRequest()
    serverRequest.value = request
    if (!request) {
      throw new Error('A szerver beállításai elvesztek. Indítsd újra a szerverkészítést.')
    }
    const [optionResponse, planResponse] = await Promise.all([
      api.get('/api/billing/options'),
      api.get('/api/billing/plans')
    ])
    options.value = optionResponse.data
    plans.value = planResponse.data || []
    if (plans.value.length) {
      planId.value = String(plans.value[0].id)
      setDefaultCycle()
    }
    if (options.value.providers.includes('stripe')) provider.value = 'stripe'
    await loadNodes()
  } catch (failure) {
    error.value = failure?.msg || failure?.message || 'A csomagok betöltése nem sikerült.'
  } finally {
    loading.value = false
  }
}

async function loadNodes() {
  nodes.value = []
  if (!plan.value) return
  try {
    const response = await api.get('/api/billing/nodes', { planId: plan.value.id })
    nodes.value = response.data || []
  } catch (failure) {
    error.value = failure?.msg || failure?.message || 'A node-kapacitás lekérdezése nem sikerült.'
  }
}

watch(planId, () => {
  setDefaultCycle()
  loadNodes()
})

const nodeAvailable = computed(() => node.value?.available === true)
const free = computed(() => cycleAvailable(cycle.value) && Number(priceFor(cycle.value)) === 0)
const canCheckout = computed(() =>
  !loading.value && !submitting.value && plan.value && cycleAvailable(cycle.value) && nodeAvailable.value &&
  (free.value || options.value.providers.includes(provider.value))
)

async function checkout() {
  if (!canCheckout.value) return
  submitting.value = true
  error.value = ''
  try {
    const response = await api.post('/api/billing/checkout', {
      planId: plan.value.id,
      billingCycle: cycle.value,
      autoRenew: !free.value && autoRenew.value,
      provider: free.value ? '' : provider.value,
      server: serverRequest.value
    })
    sessionStorage.removeItem(checkoutRequestKey)
    if (response.data.checkoutUrl) {
      window.location.assign(response.data.checkoutUrl)
      return
    }
    router.push({ name: 'BillingComplete', query: { purchaseId: response.data.purchaseId } })
  } catch (failure) {
    error.value = failure?.msg || failure?.message || 'A vásárlás nem indítható el.'
  } finally {
    submitting.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="billing-checkout">
    <h1><icon name="payment" /> Csomag vásárlása</h1>
    <loader v-if="loading" />
    <template v-else>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <template v-if="!error">
        <label>Csomag<select v-model="planId"><option v-for="item in plans" :key="item.id" :value="String(item.id)">{{ item.name }} · {{ item.cpuCapacityMilli / 1000 }} CPU · {{ item.memoryCapacityMB }} MB RAM</option></select></label>
        <label>Időszak<select v-model="cycle"><option v-if="plan?.allowOneTime" value="once">Egyszeri · {{ formatPrice(plan.oneTimePriceMinor) }}<template v-if="plan.oneTimeDurationDays"> · {{ plan.oneTimeDurationDays }} nap</template></option><option v-if="plan?.allowMonthly" value="month">Havi · {{ formatPrice(plan.monthlyPriceMinor) }}</option><option v-if="plan?.allowYearly" value="year">Éves · {{ formatPrice(plan.yearlyPriceMinor) }}</option></select></label>
        <label v-if="cycle !== 'once' && !free" class="check"><input v-model="autoRenew" type="checkbox"> Automatikus megújítás</label>
        <label v-if="!free">Fizetési szolgáltató<select v-model="provider"><option v-for="item in options.providers" :key="item" :value="item">{{ item }}</option></select></label>
        <div class="node-status" :data-available="nodeAvailable">
          <strong>Node: {{ node?.name || 'nincs kiválasztva' }}</strong>
          <span v-if="nodeAvailable">Szabad: {{ node.availableCpuCapacityMilli / 1000 }} CPU · {{ node.availableMemoryCapacityMB }} MB RAM</span>
          <span v-else>Ez a node megtelt vagy nem kérdezhető le. Lépj vissza és válassz másikat.</span>
        </div>
        <p v-if="serverRequest?.name" class="hint">Létrehozandó szerver: <strong>{{ serverRequest.name }}</strong></p>
        <div class="actions"><btn @click="router.back()">Vissza</btn><btn color="primary" :disabled="!canCheckout" @click="checkout"><icon :name="submitting ? 'loading' : 'payment'" :spin="submitting" /> {{ free ? 'Ingyenes csomag igénylése' : 'Fizetés folytatása' }}</btn></div>
      </template>
    </template>
  </div>
</template>

<style scoped lang="scss">
.billing-checkout { max-width: 620px; display: grid; gap: 16px; }
h1 { display: flex; align-items: center; gap: 10px; }
label { display: flex; flex-direction: column; gap: 5px; color: var(--color-text-secondary); }
label.check { flex-direction: row; align-items: center; }
input, select { padding: 9px; border: 1px solid var(--color-background); border-radius: 5px; background: var(--color-background-secondary); color: var(--color-text); }
.node-status { display: grid; gap: 4px; padding: 12px; background: var(--color-background-secondary); border-left: 3px solid var(--color-error); }
.node-status[data-available="true"] { border-color: var(--color-success); }
.node-status span, .hint { color: var(--color-text-secondary); }
.error { color: var(--color-error); }
.actions { display: flex; justify-content: space-between; gap: 8px; }
</style>
