<script setup>
import { ref, computed, watch, inject, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const props = defineProps({
  kind: { type: String, default: 'web' }
})

const api = inject('api')
const toast = inject('toast')
const router = useRouter()

const engine = ref('mariadb')
const template = ref(null)
const nodes = ref([])
const nodeId = ref(null)
const name = ref('')
const version = ref('')
const memory = ref(512)
const cpu = ref(1)
const dbName = ref('app')
const dbUser = ref('app')
const dbPassword = ref('')
const rootPassword = ref('')
const creating = ref(false)
const error = ref('')
const publicAccess = ref(props.kind === 'web')
const pmaHost = ref('127.0.0.1')
const pmaPort = ref(3306)

const isPma = computed(() => props.kind === 'db' && engine.value === 'phpmyadmin')

const isDb = computed(() => props.kind === 'db')
const templateKind = computed(() => (isDb.value ? engine.value : 'web'))
const versions = computed(() => (template.value?.data?.[isDb.value ? 'version' : 'php_version']?.options || []).map(o => o.value))
const identifier = /^[A-Za-z0-9_]{1,32}$/

const canSubmit = computed(() =>
  /^[\x20-\x7e]+$/.test(name.value.trim()) &&
  nodeId.value !== null &&
  Number(memory.value) >= 128 &&
  Number(cpu.value) > 0 &&
  (!isDb.value || (isPma.value
    ? pmaHost.value.trim() !== '' && Number(pmaPort.value) > 0 && Number(pmaPort.value) < 65536
    : identifier.test(dbName.value) && identifier.test(dbUser.value) &&
      dbPassword.value.length >= 8 && (engine.value !== 'mariadb' || rootPassword.value.length >= 8)
  ))
)

function randomPassword() {
  const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789'
  const bytes = crypto.getRandomValues(new Uint8Array(20))
  return Array.from(bytes, b => alphabet[b % alphabet.length]).join('')
}

async function loadTemplate() {
  template.value = null
  template.value = (await api.get(`/api/hosting/templates/${templateKind.value}`)).data
  if (!isPma.value) version.value = template.value.data[isDb.value ? 'version' : 'php_version'].value
}

onMounted(async () => {
  nodes.value = (await api.node.list()).map(n => ({ value: n.id, label: n.name }))
  if (nodes.value.length === 1) nodeId.value = nodes.value[0].value
  await loadTemplate()
})

watch(engine, loadTemplate)

async function create() {
  if (!canSubmit.value || creating.value) return
  creating.value = true
  error.value = ''
  try {
    const features = await api.node.features(nodeId.value)
    if ((features.features || []).indexOf('docker') < 0) {
      error.value = 'A kiválasztott node nem támogatja a Dockert, ehhez erre van szükség.'
      return
    }
    const self = await api.self.get()
    const data = JSON.parse(JSON.stringify(template.value.data))
    // Private databases only listen on the node's loopback address.
    if (data.ip) data.ip.value = publicAccess.value ? '0.0.0.0' : '127.0.0.1'
    if (isPma.value) {
      data.pma_host.value = pmaHost.value.trim()
      data.pma_port.value = Number(pmaPort.value)
    } else if (isDb.value) {
      data.version.value = version.value
      data.db_name.value = dbName.value
      data.db_user.value = dbUser.value
      data.db_password.value = dbPassword.value
      if (data.root_password) data.root_password.value = rootPassword.value
    } else {
      data.php_version.value = version.value
    }

    const environment = JSON.parse(JSON.stringify(template.value.environment))
    const bytes = Math.floor(Number(memory.value)) * 1024 * 1024
    // Docker enforces these limits; disk usage is not limited.
    environment.hostConfig = { Memory: bytes, MemorySwap: bytes, NanoCpus: Math.round(Number(cpu.value) * 1e9), PidsLimit: 512 }

    const request = { ...template.value, name: name.value.trim(), node: nodeId.value, environment, users: [self.username], data }
    const id = await api.server.create(request)
    toast.success('Létrehozva.')
    router.push({ name: 'ServerView', params: { id }, query: { created: true } })
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div class="hosting-create">
    <h1><icon name="server" /> {{ isDb ? 'Új adatbázis' : 'Új webtárhely' }}</h1>
    <loader v-if="!template" />
    <form v-else @submit.prevent="create">
      <label>Név<input v-model="name" maxlength="40" required></label>
      <label>Node
        <select v-model="nodeId" required>
          <option :value="null" disabled>Válassz node-ot</option>
          <option v-for="n in nodes" :key="n.value" :value="n.value">{{ n.label }}</option>
        </select>
      </label>
      <label v-if="isDb">Adatbázis-motor
        <select v-model="engine"><option value="mariadb">MariaDB (MySQL-kompatibilis)</option><option value="postgres">PostgreSQL</option><option value="phpmyadmin">phpMyAdmin (webes MariaDB/MySQL kezelő)</option></select>
      </label>
      <label v-if="!isPma">{{ isDb ? 'Verzió' : 'PHP verzió' }}
        <select v-model="version"><option v-for="v in versions" :key="v" :value="v">{{ v }}</option></select>
      </label>
      <label>Memóriakorlát (MB)<input v-model.number="memory" type="number" min="128" step="128"></label>
      <label>CPU korlát (mag)<input v-model.number="cpu" type="number" min="0.25" step="0.25"></label>
      <template v-if="isPma">
        <label>Adatbázis-szerver címe<input v-model="pmaHost" maxlength="253"></label>
        <label>Adatbázis-szerver portja<input v-model.number="pmaPort" type="number" min="1" max="65535"></label>
        <p class="hint">A bejelentkezéshez az adatbázis saját felhasználóneve és jelszava kell. A phpMyAdmin host hálózaton fut, így a node loopback címén lévő (privát) adatbázisokat is eléri; a webfelület minden interfészen hallgat.</p>
      </template>
      <template v-else-if="isDb">
        <label>Adatbázis neve<input v-model="dbName" maxlength="32" pattern="[A-Za-z0-9_]+"></label>
        <label>Felhasználó<input v-model="dbUser" maxlength="32" pattern="[A-Za-z0-9_]+"></label>
        <label>Jelszó (min. 8 karakter)
          <span class="row"><input v-model="dbPassword" type="text" minlength="8" autocomplete="off"><btn type="button" @click="dbPassword = randomPassword()">Generál</btn></span>
        </label>
        <label v-if="engine === 'mariadb'">Root jelszó (min. 8 karakter)
          <span class="row"><input v-model="rootPassword" type="text" minlength="8" autocomplete="off"><btn type="button" @click="rootPassword = randomPassword()">Generál</btn></span>
        </label>
        <p class="hint">A név, felhasználó és jelszavak csak az első indításkor érvényesülnek; utólag SQL-ből módosíthatók.</p>
      </template>
      <label v-if="!isPma" class="check"><input v-model="publicAccess" type="checkbox"> Nyilvános elérés (kikapcsolva csak a node saját gépéről, 127.0.0.1-en érhető el)</label>
      <div v-if="error" class="error">{{ error }}</div>
      <btn color="primary" type="submit" :disabled="!canSubmit || creating"><icon :name="creating ? 'loading' : 'plus'" :spin="creating" /> Létrehozás</btn>
    </form>
  </div>
</template>

<style scoped lang="scss">
.hosting-create { max-width: 560px; }
h1 { display: flex; align-items: center; gap: 10px; }
form { display: flex; flex-direction: column; gap: 14px; }
label { display: flex; flex-direction: column; gap: 4px; color: var(--color-text-secondary); }
label.check { flex-direction: row; align-items: center; gap: 8px; }
.row { display: flex; gap: 8px; }
.row input { flex: 1; }
input, select { padding: 9px; border: 1px solid var(--color-background); border-radius: 5px; background: var(--color-background-secondary); color: var(--color-text); }
.error { color: var(--color-error); }
.hint { color: var(--color-text-secondary); margin: 0; }
</style>
