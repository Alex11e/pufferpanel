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
const nodeId = ref(null)
const name = ref('')
const ram = ref(2048)
const disk = ref(20)
const uefi = ref(true)
const kvm = ref(false)
const vncPassword = ref('')
const vncDisplay = ref(1)
const iso = ref('netboot.xyz.iso')
const creating = ref(false)
const error = ref('')

const canSubmit = computed(() =>
  /^[\x20-\x7e]+$/.test(name.value.trim()) &&
  nodeId.value !== null &&
  Number(ram.value) >= 256 &&
  Number(disk.value) >= 1 &&
  vncPassword.value.length >= 1 && vncPassword.value.length <= 8 &&
  Number.isInteger(Number(vncDisplay.value)) && Number(vncDisplay.value) >= 0
)

onMounted(async () => {
  template.value = (await api.get('/api/vps/template')).data
  nodes.value = (await api.node.list()).map(n => ({ value: n.id, label: n.name }))
  if (nodes.value.length === 1) nodeId.value = nodes.value[0].value
})

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
      vnc_password: vncPassword.value, vnc_display: Number(vncDisplay.value), iso_file: iso.value.trim()
    }
    for (const key in values) data[key].value = values[key]

    const environment = JSON.parse(JSON.stringify(template.value.environment))
    // KVM only works when the container can see the host device.
    if (kvm.value) environment.hostConfig = { Devices: [{ PathOnHost: '/dev/kvm', PathInContainer: '/dev/kvm', CgroupPermissions: 'rwm' }] }

    const request = { ...template.value, name: name.value.trim(), node: nodeId.value, environment, users: [self.username], data }
    const id = await api.server.create(request)
    toast.success('A VPS létrejött.')
    router.push({ name: 'ServerView', params: { id }, query: { created: true } })
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div class="vps-create">
    <h1><icon name="server" /> Új VPS</h1>
    <loader v-if="!template" />
    <form v-else @submit.prevent="create">
      <label>Név<input v-model="name" maxlength="40" required></label>
      <label>Node
        <select v-model="nodeId" required>
          <option :value="null" disabled>Válassz node-ot</option>
          <option v-for="n in nodes" :key="n.value" :value="n.value">{{ n.label }}</option>
        </select>
      </label>
      <label>Memória (MB)<input v-model.number="ram" type="number" min="256" step="256"></label>
      <label>Lemez (GB)<input v-model.number="disk" type="number" min="1"></label>
      <label>Boot ISO<input v-model="iso" placeholder="üres = lemezről indul"></label>
      <label>VNC jelszó (max. 8 karakter)<input v-model="vncPassword" type="password" maxlength="8" autocomplete="new-password" required></label>
      <label>VNC kijelző (port: 5900 + szám, node-onként egyedi)<input v-model.number="vncDisplay" type="number" min="0"></label>
      <label class="check"><input v-model="uefi" type="checkbox"> UEFI indítás</label>
      <label class="check"><input v-model="kvm" type="checkbox"> KVM gyorsítás (a node-nak elérhetővé kell tennie a /dev/kvm eszközt)</label>
      <div v-if="error" class="error">{{ error }}</div>
      <btn color="primary" type="submit" :disabled="!canSubmit || creating"><icon :name="creating ? 'loading' : 'plus'" :spin="creating" /> Létrehozás</btn>
    </form>
  </div>
</template>

<style scoped lang="scss">
.vps-create { max-width: 560px; }
h1 { display: flex; align-items: center; gap: 10px; }
form { display: flex; flex-direction: column; gap: 14px; }
label { display: flex; flex-direction: column; gap: 4px; color: var(--color-text-secondary); }
label.check { flex-direction: row; align-items: center; gap: 8px; }
input:not([type="checkbox"]), select { padding: 9px; border: 1px solid var(--color-background); border-radius: 5px; background: var(--color-background-secondary); color: var(--color-text); }
.error { color: var(--color-error); }
</style>
