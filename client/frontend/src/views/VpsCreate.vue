<script setup>
import { ref, computed, inject, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const toast = inject('toast')
const router = useRouter()

const template = ref(null)
const nodes = ref([])
const loading = ref(true)
const nodeId = ref(null)
const name = ref('')
const ram = ref(2048)
const disk = ref(20)
const uefi = ref(true)
const kvm = ref(false)
const vncPassword = ref('')
const iso = ref('netboot.xyz.iso')
const creating = ref(false)
const error = ref('')
const loadError = ref('')

const canSubmit = computed(() =>
  /^[\x20-\x7e]+$/.test(name.value.trim()) && name.value.trim().length <= 40 &&
  nodes.value.some(node => node.value === nodeId.value) &&
  Number.isInteger(Number(ram.value)) && Number(ram.value) >= 256 &&
  Number.isInteger(Number(disk.value)) && Number(disk.value) >= 1 &&
  /^[\x20-\x7e]{1,8}$/.test(vncPassword.value)
)

function errorMessage(error) {
  return error?.msg || error?.message || 'Ismeretlen hiba történt.'
}

async function loadSetup() {
  loading.value = true
  loadError.value = ''
  try {
    const [templateResponse, nodeRecords] = await Promise.all([
      api.get('/api/vps/template'),
      api.node.list()
    ])
    template.value = templateResponse.data
    nodes.value = nodeRecords.map(node => ({ value: node.id, label: node.name }))
    if (nodes.value.length === 1) nodeId.value = nodes.value[0].value
  } catch (loadFailure) {
    loadError.value = errorMessage(loadFailure)
  } finally {
    loading.value = false
  }
}

onMounted(loadSetup)

async function create() {
  if (!canSubmit.value || creating.value) return
  creating.value = true
  error.value = ''
  try {
    const features = await api.node.features(nodeId.value)
    if ((features.features || []).indexOf('docker') < 0) {
      error.value = 'A kiválasztott node nem támogatja a Dockert, a VPS-hez erre van szükség.'
      return
    }
    const self = await api.self.get()
    const data = JSON.parse(JSON.stringify(template.value.data))
    const values = {
      ram: Number(ram.value), disk_size: Number(disk.value), use_uefi: uefi.value, use_kvm: kvm.value,
      vnc_password: vncPassword.value, iso_file: iso.value.trim()
    }
    for (const key in values) data[key].value = values[key]

    const environment = JSON.parse(JSON.stringify(template.value.environment))
    // KVM only works when the container can see the host device.
    if (kvm.value) environment.hostConfig = { Devices: [{ PathOnHost: '/dev/kvm', PathInContainer: '/dev/kvm', CgroupPermissions: 'rwm' }] }

    const request = { ...template.value, name: name.value.trim(), node: nodeId.value, environment, users: [self.username], data }
    const id = await api.server.create(request)
    toast.success('A VPS létrejött.')
    router.push({ name: 'ServerView', params: { id }, query: { created: true } })
  } catch (creationError) {
    error.value = errorMessage(creationError)
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div class="vps-create">
    <h1><icon name="server" /> Új VPS</h1>
    <loader v-if="loading" />
    <div v-else-if="loadError" class="error-state" role="alert">
      <p>{{ loadError }}</p>
      <btn @click="loadSetup"><icon name="reload" /> Újrapróbálás</btn>
    </div>
    <form v-else-if="template" @submit.prevent="create">
      <label>Név<input v-model="name" maxlength="40" required></label>
      <label>Node
        <select v-model="nodeId" required>
          <option :value="null" disabled>Válassz node-ot</option>
          <option v-for="n in nodes" :key="n.value" :value="n.value">{{ n.label }}</option>
        </select>
      </label>
      <p v-if="!nodes.length" class="error" role="alert">Nincs elérhető node a VPS létrehozásához.</p>
      <label>Memória (MB)<input v-model.number="ram" type="number" min="256" step="1" required></label>
      <label>Lemez (GB)<input v-model.number="disk" type="number" min="1" step="1" required></label>
      <label>Boot ISO<input v-model="iso" placeholder="üres = lemezről indul"></label>
      <label>VNC jelszó (max. 8 karakter)<input v-model="vncPassword" type="password" maxlength="8" autocomplete="new-password" required></label>
      <p class="hint">A VPS VNC-portját a rendszer automatikusan, node-onként egyedien foglalja le.</p>
      <label class="check"><input v-model="uefi" type="checkbox"> UEFI indítás</label>
      <label class="check"><input v-model="kvm" type="checkbox"> KVM gyorsítás (a node-nak elérhetővé kell tennie a /dev/kvm eszközt)</label>
      <div v-if="error" class="error">{{ error }}</div>
      <btn color="primary" type="submit" :disabled="!canSubmit || creating" @click="create"><icon :name="creating ? 'loading' : 'plus'" :spin="creating" /> Létrehozás</btn>
    </form>
  </div>
</template>

<style scoped lang="scss">
.vps-create { max-width: 560px; }
.error-state { color: var(--color-error); }
h1 { display: flex; align-items: center; gap: 10px; }
form { display: flex; flex-direction: column; gap: 14px; }
label { display: flex; flex-direction: column; gap: 4px; color: var(--color-text-secondary); }
label.check { flex-direction: row; align-items: center; gap: 8px; }
input:not([type="checkbox"]), select { padding: 9px; border: 1px solid var(--color-background); border-radius: 5px; background: var(--color-background-secondary); color: var(--color-text); }
.error { color: var(--color-error); }
.hint { color: var(--color-text-secondary); }
</style>
