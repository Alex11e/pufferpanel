<script setup>
import { inject, onMounted, ref } from 'vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const toast = inject('toast')
const loading = ref(true)
const error = ref('')
const plans = ref([])
const nodes = ref([])
const editingId = ref(null)
const granting = ref(false)
const plan = ref(emptyPlan())
const priceInputs = ref({ oneTime: '0', monthly: '0', yearly: '0' })
const grant = ref({ userId: '', planId: '', nodeId: 0, billingCycle: 'once' })

function emptyPlan() {
  return {
    name: '', description: '', active: true, currency: 'HUF',
    oneTimePriceMinor: 0, monthlyPriceMinor: 0, yearlyPriceMinor: 0,
    oneTimeDurationDays: 0, cpuCapacityMilli: 1000, memoryCapacityMB: 1024, maxServers: 1
  }
}

function decimals(currency = plan.value.currency) {
  return currency === 'HUF' ? 0 : 2
}

function fromMinor(amount, currency) {
  return (Number(amount || 0) / (10 ** decimals(currency))).toFixed(decimals(currency))
}

function toMinor(amount, currency) {
  return Math.round(Number(amount || 0) * (10 ** decimals(currency)))
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [planResponse, nodeResponse] = await Promise.all([
      api.get('/api/billing/admin/plans'),
      api.node.list()
    ])
    plans.value = planResponse.data || []
    nodes.value = nodeResponse || []
  } catch (failure) {
    error.value = failure?.msg || failure?.message || 'Nem sikerült betölteni a csomagokat.'
  } finally {
    loading.value = false
  }
}

function editPlan(item) {
  editingId.value = item.id
  plan.value = { ...item }
  priceInputs.value = {
    oneTime: fromMinor(item.oneTimePriceMinor, item.currency),
    monthly: fromMinor(item.monthlyPriceMinor, item.currency),
    yearly: fromMinor(item.yearlyPriceMinor, item.currency)
  }
}

function resetForm() {
  editingId.value = null
  plan.value = emptyPlan()
  priceInputs.value = { oneTime: '0', monthly: '0', yearly: '0' }
}

async function savePlan() {
  const payload = {
    ...plan.value,
    oneTimePriceMinor: toMinor(priceInputs.value.oneTime, plan.value.currency),
    monthlyPriceMinor: toMinor(priceInputs.value.monthly, plan.value.currency),
    yearlyPriceMinor: toMinor(priceInputs.value.yearly, plan.value.currency),
    cpuCapacityMilli: Math.round(Number(plan.value.cpuCapacityMilli) * 1000),
    memoryCapacityMB: Math.floor(Number(plan.value.memoryCapacityMB)),
    maxServers: Math.floor(Number(plan.value.maxServers)),
    oneTimeDurationDays: Math.floor(Number(plan.value.oneTimeDurationDays))
  }
  try {
    if (editingId.value) await api.put(`/api/billing/admin/plans/${editingId.value}`, payload)
    else await api.post('/api/billing/admin/plans', payload)
    toast.success('Csomag elmentve.')
    resetForm()
    await load()
  } catch (failure) {
    error.value = failure?.msg || failure?.message || 'Nem sikerült elmenteni a csomagot.'
  }
}

async function archivePlan(item) {
  try {
    await api.delete(`/api/billing/admin/plans/${item.id}`)
    toast.success('Csomag archiválva.')
    await load()
  } catch (failure) {
    error.value = failure?.msg || failure?.message || 'Nem sikerült archiválni a csomagot.'
  }
}

async function grantPlan() {
  if (!grant.value.userId || !grant.value.planId || grant.value.nodeId === '') return
  granting.value = true
  error.value = ''
  try {
    await api.post('/api/billing/admin/purchases/grant', {
      ...grant.value,
      userId: Number(grant.value.userId),
      planId: Number(grant.value.planId),
      nodeId: Number(grant.value.nodeId)
    })
    toast.success('A csomag díjmentesen kiosztva.')
    grant.value.userId = ''
  } catch (failure) {
    error.value = failure?.msg || failure?.message || 'Nem sikerült kiosztani a csomagot.'
  } finally {
    granting.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="billing-admin">
    <div class="heading">
      <div><h1><icon name="payment" /> Csomagok és jogosultságok</h1><p>Árak, erőforrás-keretek és díjmentes kiosztások.</p></div>
      <btn :disabled="loading" @click="load"><icon name="reload" /> Frissítés</btn>
    </div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <loader v-if="loading" />
    <template v-else>
      <section>
        <h2>{{ editingId ? 'Csomag szerkesztése' : 'Új csomag' }}</h2>
        <form class="plan-form" @submit.prevent="savePlan">
          <label>Név<input v-model.trim="plan.name" maxlength="100" required></label>
          <label>Leírás<textarea v-model="plan.description" maxlength="2000" rows="2" /></label>
          <label>Pénznem<select v-model="plan.currency"><option>HUF</option><option>EUR</option><option>USD</option></select></label>
          <label>Egyszeri ár ({{ plan.currency }})<input v-model="priceInputs.oneTime" type="number" min="0" step="0.01"></label>
          <label>Havi ár ({{ plan.currency }})<input v-model="priceInputs.monthly" type="number" min="0" step="0.01"></label>
          <label>Éves ár ({{ plan.currency }})<input v-model="priceInputs.yearly" type="number" min="0" step="0.01"></label>
          <label>Egyszeri csomag érvényessége (nap, 0 = korlátlan)<input v-model.number="plan.oneTimeDurationDays" type="number" min="0" step="1"></label>
          <label>CPU keret (mag)<input v-model.number="plan.cpuCapacityMilli" type="number" min="0.1" step="0.1" required></label>
          <label>Memóriakeret (MB)<input v-model.number="plan.memoryCapacityMB" type="number" min="128" step="128" required></label>
          <label>Szerverek száma<input v-model.number="plan.maxServers" type="number" min="1" step="1" required></label>
          <label class="check"><input v-model="plan.active" type="checkbox"> Aktív, vásárolható csomag</label>
          <div class="form-actions"><btn color="primary" type="submit"><icon name="save" /> Mentés</btn><btn v-if="editingId" type="button" @click="resetForm">Mégse</btn></div>
        </form>
      </section>
      <section>
        <h2>Díjmentes admin kiosztás</h2>
        <div class="grant-form">
          <label>Felhasználó ID<input v-model="grant.userId" type="number" min="1" required></label>
          <label>Csomag<select v-model.number="grant.planId" required><option value="">Válassz csomagot</option><option v-for="item in plans" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
          <label>Node<select v-model.number="grant.nodeId"><option v-for="node in nodes" :key="node.id" :value="node.id">{{ node.name }}</option></select></label>
          <label>Időszak<select v-model="grant.billingCycle"><option value="once">Egyszeri</option><option value="month">Havi</option><option value="year">Éves</option></select></label>
          <btn color="primary" :disabled="granting || !grant.userId || !grant.planId" @click="grantPlan"><icon :name="granting ? 'loading' : 'plus'" :spin="granting" /> Kiosztás fizetés nélkül</btn>
        </div>
      </section>
      <section>
        <h2>Elérhető csomagok</h2>
        <p v-if="plans.length === 0" class="empty">Még nincs csomag.</p>
        <div v-for="item in plans" :key="item.id" class="plan-row">
          <div class="plan-details">
            <strong>{{ item.name }} <small>#{{ item.id }} · {{ item.active ? 'aktív' : 'archivált' }}</small></strong>
            <span>{{ item.currency }} {{ fromMinor(item.oneTimePriceMinor, item.currency) }} egyszeri · {{ fromMinor(item.monthlyPriceMinor, item.currency) }}/hó · {{ fromMinor(item.yearlyPriceMinor, item.currency) }}/év</span>
            <small>{{ item.cpuCapacityMilli / 1000 }} CPU · {{ item.memoryCapacityMB }} MB RAM · {{ item.maxServers }} szerver</small>
          </div>
          <div class="row-actions"><btn variant="icon" tooltip="Szerkesztés" @click="editPlan(item)"><icon name="edit" /></btn><btn variant="icon" tooltip="Archiválás" :disabled="!item.active" @click="archivePlan(item)"><icon name="remove" /></btn></div>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped lang="scss">
.billing-admin { max-width: 1100px; margin: 0 auto; }
.heading { display:flex; align-items:center; justify-content:space-between; gap:16px; margin-bottom:20px; }
.heading h1 { display:flex; align-items:center; gap:10px; margin-bottom:4px; }
.heading p, small { color:var(--color-text-secondary); }
section { margin-top:16px; padding:16px; border:1px solid var(--color-background); border-radius:8px; background:var(--color-background-secondary); }
section h2 { margin-top:0; }
.plan-form, .grant-form { display:grid; grid-template-columns:repeat(auto-fit,minmax(190px,1fr)); gap:12px; }
label { display:flex; flex-direction:column; gap:4px; color:var(--color-text-secondary); }
label.check { flex-direction:row; align-items:center; }
input, select, textarea { min-width:0; padding:9px; border:1px solid var(--color-background); border-radius:5px; background:var(--color-background); color:var(--color-text); }
.form-actions { display:flex; align-items:end; gap:8px; }
.plan-row { display:flex; align-items:center; justify-content:space-between; gap:12px; padding:12px 0; border-bottom:1px solid var(--color-background); }
.plan-row:last-child { border-bottom:0; }
.plan-details { display:grid; gap:4px; min-width:0; }
.plan-details span { color:var(--color-text-secondary); overflow-wrap:anywhere; }
.row-actions { display:flex; gap:4px; }
.error { color:var(--color-error); }
.empty { color:var(--color-text-secondary); }
@media (max-width:700px) { .heading { align-items:flex-start; flex-direction:column; } .plan-row { align-items:flex-start; } }
</style>
