<script setup>
import { ref, computed, watch, inject, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const props = defineProps({
  kind: { type: String, default: 'web' }
})

const api = inject('api')
const toast = inject('toast')
const router = useRouter()
const { t } = useI18n()

const engine = ref('mariadb')
const botLanguage = ref('node')
const storageMode = ref('files')
const template = ref(null)
const loading = ref(true)
const loadError = ref('')
const nodes = ref([])
const nodeId = ref(null)
const name = ref('')
const version = ref('')
const memory = ref(props.kind === 'gibbon' ? 1024 : 512)
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
const pmaDatabase = ref('app')
const pmaUser = ref('app')
const pmaPassword = ref('')
const smbPassword = ref(randomPassword())

const isPma = computed(() => props.kind === 'db' && engine.value === 'phpmyadmin')

const isDb = computed(() => props.kind === 'db')
const isBot = computed(() => props.kind === 'discordbot')
const isWeb = computed(() => props.kind === 'web')
const isJellyfin = computed(() => props.kind === 'jellyfin')
const isStaticduomusic = computed(() => props.kind === 'staticduomusic')
const isStorage = computed(() => props.kind === 'storage')
const isGibbon = computed(() => props.kind === 'gibbon')
const isSamba = computed(() => props.kind === 'samba')
const selectedNode = computed(() => nodes.value.find(node => node.value === nodeId.value))
const templateKind = computed(() => (isBot.value ? (botLanguage.value === 'python' ? 'discordbot-python' : 'discordbot') : isDb.value ? engine.value : isJellyfin.value ? 'jellyfin' : isStaticduomusic.value ? 'staticduomusic' : isStorage.value ? (storageMode.value === 'nextcloud' ? 'storage-nextcloud' : 'storage-files') : isGibbon.value ? 'gibbon' : isSamba.value ? 'storage-samba' : 'web'))
const versions = computed(() => {
  const versionKey = isDb.value ? 'version' : isWeb.value ? 'php_version' : ''
  return (template.value?.data?.[versionKey]?.options || []).map(o => o.value)
})
const identifier = /^[A-Za-z0-9_]{1,32}$/
const botToken = ref('')

function errorMessage(failure) {
  return failure?.msg || failure?.message || 'Ismeretlen hiba történt.'
}

const canSubmit = computed(() =>
  /^[\x20-\x7e]+$/.test(name.value.trim()) &&
  nodeId.value !== null &&
  Number.isInteger(Number(memory.value)) && Number(memory.value) >= 128 &&
  Number.isFinite(Number(cpu.value)) && Number(cpu.value) > 0 &&
  (!isSamba.value || (selectedNode.value?.privateHost && !['127.0.0.1', '::1', '0.0.0.0', '::'].includes(selectedNode.value.privateHost))) &&
  (isBot.value ? botToken.value.trim().length > 0 : isSamba.value ? smbPassword.value.length >= 12 : !isDb.value || (isPma.value
    ? pmaHost.value.trim() !== '' && Number.isInteger(Number(pmaPort.value)) && Number(pmaPort.value) > 0 && Number(pmaPort.value) < 65536 &&
      pmaDatabase.value.trim() !== '' && pmaUser.value.trim() !== '' && pmaPassword.value.length > 0
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
  loadError.value = ''
  try {
    template.value = (await api.get(`/api/hosting/templates/${templateKind.value}`)).data
    if (!isPma.value && !isBot.value && (isDb.value || isWeb.value)) version.value = template.value.data[isDb.value ? 'version' : 'php_version'].value
  } catch (failure) {
    loadError.value = errorMessage(failure)
  }
}

async function loadSetup() {
  loading.value = true
  loadError.value = ''
  try {
    nodes.value = (await api.node.list()).map(n => ({ value: n.id, label: n.name, privateHost: n.privateHost }))
    if (nodes.value.length === 1) nodeId.value = nodes.value[0].value
    await loadTemplate()
  } catch (failure) {
    loadError.value = errorMessage(failure)
  } finally {
    loading.value = false
  }
}

onMounted(loadSetup)

watch(engine, () => {
  if (!loading.value) loadTemplate()
})

watch(botLanguage, () => {
  if (!loading.value) loadTemplate()
})

watch(storageMode, () => {
  if (!loading.value) loadTemplate()
})

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
    if (isGibbon.value) {
      const created = await createGibbon(self)
      toast.success(t('servers.services.gibbon.created'))
      router.push({ name: 'ServerView', params: { id: created.gibbonId }, query: { created: true } })
      return
    }

    const data = JSON.parse(JSON.stringify(template.value.data))
    // Private databases only listen on the node's loopback address.
    if (data.ip) data.ip.value = isDb.value ? (publicAccess.value ? '0.0.0.0' : '127.0.0.1') : isSamba.value ? selectedNode.value.privateHost : '0.0.0.0'
    if (isBot.value) {
      data.token.value = botToken.value.trim()
    } else if (isSamba.value) {
      data.smb_password.value = smbPassword.value
    } else if (isPma.value) {
      data.pma_host.value = pmaHost.value.trim()
      data.pma_port.value = Number(pmaPort.value)
      data.pma_database.value = pmaDatabase.value.trim()
      data.pma_user.value = pmaUser.value.trim()
      data.pma_password.value = pmaPassword.value
    } else if (isDb.value) {
      data.version.value = version.value
      data.db_name.value = dbName.value
      data.db_user.value = dbUser.value
      data.db_password.value = dbPassword.value
      if (data.root_password) data.root_password.value = rootPassword.value
    } else if (isWeb.value) {
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
  } catch (failure) {
    error.value = errorMessage(failure)
  } finally {
    creating.value = false
  }
}

async function createGibbon(self) {
  const node = nodes.value.find(item => item.value === nodeId.value)
  if (!node?.privateHost || ['127.0.0.1', '::1', '0.0.0.0', '::'].includes(node.privateHost)) {
    throw new Error(t('servers.services.gibbon.privateHostRequired'))
  }

  const dbTemplate = (await api.get('/api/hosting/templates/mariadb')).data
  const dbData = JSON.parse(JSON.stringify(dbTemplate.data))
  dbData.ip.value = node.privateHost
  dbData.db_name.value = 'gibbon'
  dbData.db_user.value = 'gibbon'
  dbData.db_password.value = randomPassword()
  dbData.root_password.value = randomPassword()
  dbData.version.value = '10.11'
  const dbEnvironment = JSON.parse(JSON.stringify(dbTemplate.environment))
  dbEnvironment.hostConfig = { Memory: 512 * 1024 * 1024, MemorySwap: 512 * 1024 * 1024, NanoCpus: 1e9, PidsLimit: 512 }

  let databaseServerId = ''
Z  let gibbonServerId = ''
  try {
    databaseServerId = await api.server.create({
      ...dbTemplate,
      name: `${name.value.trim().slice(0, 31)} DB`,
      node: nodeId.value,
      environment: dbEnvironment,
      users: [self.username],
      data: dbData
    })
    await api.server.start(databaseServerId, true)
    const databaseServer = await api.server.get(databaseServerId, false)
    if (!databaseServer.port) throw new Error(t('servers.services.gibbon.databasePortMissing'))

    const data = JSON.parse(JSON.stringify(template.value.data))
    data.db_host.value = node.privateHost
    data.db_port.value = databaseServer.port
    data.db_name.value = 'gibbon'
    data.db_user.value = 'gibbon'
    data.db_password.value = dbData.db_password.value
    const environment = JSON.parse(JSON.stringify(template.value.environment))
    const memoryBytes = Math.floor(Number(memory.value)) * 1024 * 1024
    environment.hostConfig = { Memory: memoryBytes, MemorySwap: memoryBytes, NanoCpus: Math.round(Number(cpu.value) * 1e9), PidsLimit: 512 }

    gibbonServerId = await api.server.create({
      ...template.value,
      name: name.value.trim(),
      node: nodeId.value,
      environment,
      users: [self.username],
      data
    })
    await api.put(`/api/servers/${gibbonServerId}/database`, {
      databaseServerId,
      engine: 'mariadb',
      accessMode: 'private',
      host: node.privateHost,
      port: databaseServer.port,
      databaseName: 'gibbon',
      username: 'gibbon',
      password: dbData.db_password.value,
      variableMapping: {
        host: 'DB_HOST',
        port: 'DB_PORT',
        database: 'DB_NAME',
        username: 'DB_USERNAME',
        password: 'DB_PASSWORD'
      }
    })
    return { gibbonId: gibbonServerId, databaseServerId }
  } catch (failure) {
    if (gibbonServerId) {
      try {
        await api.server.delete(gibbonServerId)
      } catch {
        throw new Error(`${errorMessage(failure)} (${t('servers.services.gibbon.serverCleanup', { id: gibbonServerId })})`)
      }
    }
    if (databaseServerId) {
      try {
        await api.server.delete(databaseServerId)
      } catch {
        throw new Error(`${errorMessage(failure)} (${t('servers.services.gibbon.databaseCleanup', { id: databaseServerId })})`)
      }
    }
    throw failure
  }
}
</script>

<template>
  <div class="hosting-create">
    <h1><icon name="server" /> {{ isBot ? 'Új Discord bot' : isDb ? 'Új adatbázis' : isJellyfin ? t('servers.services.jellyfin.create') : isStaticduomusic ? t('servers.services.staticduomusic.create') : isStorage ? t('servers.services.storage.create') : isGibbon ? t('servers.services.gibbon.create') : isSamba ? t('servers.services.smb.create') : 'Új webtárhely' }}</h1>
    <loader v-if="loading || (!template && !loadError)" />
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
      <label v-if="isDb">Adatbázis-motor
        <select v-model="engine"><option value="mariadb">MariaDB (MySQL-kompatibilis)</option><option value="postgres">PostgreSQL</option><option value="phpmyadmin">phpMyAdmin (webes MariaDB/MySQL kezelő)</option></select>
      </label>
      <label v-if="isStorage">{{ t('servers.services.storage.kind') }}
        <select v-model="storageMode"><option value="files">{{ t('servers.services.storage.files') }}</option><option value="nextcloud">Nextcloud</option></select>
      </label>
      <template v-if="isBot">
        <label>Programozási nyelv
          <select v-model="botLanguage"><option value="node">Node.js</option><option value="python">Python</option></select>
        </label>
        <label>Discord bot token<input v-model="botToken" type="password" autocomplete="new-password" required></label>
        <p class="hint">A token privát szerveradatként tárolódik, és DISCORD_TOKEN környezeti változóként jut el a bothoz. Fájlokat a Fájlok vagy SFTP felületen tölthetsz fel.</p>
      </template>
      <template v-if="isSamba">
        <p class="hint">{{ t('servers.services.smb.createHint') }}</p>
        <label>{{ t('servers.services.smb.username') }}<input value="puffer" disabled></label>
        <label>{{ t('servers.services.smb.password') }}<input v-model="smbPassword" type="password" minlength="12" autocomplete="new-password" required></label>
      </template>
      <label v-if="!isPma && !isBot && (isDb || isWeb)">{{ isDb ? 'Verzió' : 'PHP verzió' }}
        <select v-model="version"><option v-for="v in versions" :key="v" :value="v">{{ v }}</option></select>
      </label>
      <p v-if="isJellyfin" class="hint">{{ t('servers.services.jellyfin.createHint') }}</p>
      <p v-if="isStaticduomusic" class="hint">{{ t('servers.services.staticduomusic.createHint') }}</p>
      <p v-if="isStorage && storageMode === 'files'" class="hint">{{ t('servers.services.storage.filesHint') }}</p>
      <p v-if="isStorage && storageMode === 'nextcloud'" class="hint">{{ t('servers.services.storage.nextcloudHint') }}</p>
      <p v-if="isGibbon" class="hint">{{ t('servers.services.gibbon.createHint') }}</p>
      <label>Memóriakorlát (MB)<input v-model.number="memory" type="number" min="128" step="128"></label>
      <label>CPU korlát (mag)<input v-model.number="cpu" type="number" min="0.25" step="0.25"></label>
      <template v-if="isPma">
        <label>Adatbázis-szerver címe<input v-model="pmaHost" maxlength="253"></label>
        <label>Adatbázis-szerver portja<input v-model.number="pmaPort" type="number" min="1" max="65535"></label>
        <label>Adatbázis neve<input v-model="pmaDatabase" maxlength="64" required></label>
        <label>Adatbázis-felhasználó<input v-model="pmaUser" maxlength="128" autocomplete="username" required></label>
        <label>Adatbázis-jelszó<input v-model="pmaPassword" type="password" autocomplete="new-password" required></label>
        <p class="hint">A phpMyAdmin a megadott adatbázis-fiókkal csatlakozik. A felhasználónév és jelszó privát szerveradatként tárolódik. A phpMyAdmin host hálózaton fut, így a node loopback címén lévő adatbázisokat is eléri; a webfelület minden interfészen hallgat.</p>
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
      <label v-if="isDb && !isPma" class="check"><input v-model="publicAccess" type="checkbox"> Nyilvános elérés (kikapcsolva csak a node saját gépéről, 127.0.0.1-en érhető el)</label>
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
