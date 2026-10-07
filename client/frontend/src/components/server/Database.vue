<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const props = defineProps({ server: { type: Object, required: true } })
const api = inject('api')
const toast = inject('toast')
const { t } = useI18n()

const loading = ref(true)
const saving = ref(false)
const error = ref('')
const loadError = ref('')
const linked = ref(null)
const connection = ref(null)
const nodeContext = ref(null)
const showPassword = ref(false)
const editing = ref(false)
const startFailed = ref(false)
const databaseServerNeedsStart = ref(false)
const form = ref({
  databaseServerId: '',
  engine: 'mariadb',
  accessMode: 'private',
  databaseName: 'app',
  username: 'app',
  password: randomPassword(),
  rootPassword: randomPassword(),
  host: '',
  port: 3306,
  variableMapping: {
    host: 'DB_HOST',
    port: 'DB_PORT',
    database: 'DB_DATABASE',
    username: 'DB_USERNAME',
    password: 'DB_PASSWORD'
  }
})

const canManage = computed(() => props.server.hasScope('server.admin'))
const canCreateServer = computed(() => api.auth.hasScope('server.create'))
const parentNode = computed(() => nodeContext.value)
const externalMode = computed(() => form.value.accessMode === 'external')

function randomPassword() {
  const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%'
  const bytes = crypto.getRandomValues(new Uint8Array(24))
  return Array.from(bytes, byte => alphabet[byte % alphabet.length]).join('')
}

function failureMessage(failure) {
  return failure?.msg || failure?.message || t('servers.database.failed')
}

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const databaseState = await props.server.getDatabase()
    linked.value = databaseState.database
    nodeContext.value = databaseState.node
    if (linked.value) {
      const definition = await props.server.getDefinition()
      const variables = definition.execution?.environmentVars || {}
      const mapping = linked.value.variableMapping || {}
      connection.value = {
        ...linked.value,
        password: variables[mapping.password] || ''
      }
      form.value = {
        ...form.value,
        databaseServerId: linked.value.databaseServerId || '',
        engine: linked.value.engine,
        accessMode: linked.value.accessMode,
        databaseName: linked.value.databaseName,
        username: linked.value.username,
        password: variables[mapping.password] || '',
        host: linked.value.host,
        port: linked.value.port,
        variableMapping: { ...form.value.variableMapping, ...mapping }
      }
    }
  } catch (failure) {
    loadError.value = failureMessage(failure)
  } finally {
    loading.value = false
  }
}

function setDataValue(data, name, value) {
  if (!data[name]) throw new Error(`Missing ${name} database setting`)
  data[name].value = value
}

async function createDatabaseServer() {
  const node = parentNode.value
  if (!node) throw new Error(t('servers.database.nodeUnavailable'))
  if (!node.privateHost || ['127.0.0.1', '::1', '0.0.0.0', '::'].includes(node.privateHost)) {
    throw new Error(t('servers.database.privateHostRequired'))
  }

  const features = await api.node.features(node.id)
  if (!(features.features || []).includes('docker')) throw new Error(t('servers.database.dockerRequired'))

  const template = (await api.get(`/api/hosting/templates/${form.value.engine}`)).data
  const self = await api.self.get()
  const data = JSON.parse(JSON.stringify(template.data))
  setDataValue(data, 'ip', form.value.accessMode === 'public' ? '0.0.0.0' : node.privateHost)
  setDataValue(data, 'db_name', form.value.databaseName.trim())
  setDataValue(data, 'db_user', form.value.username.trim())
  setDataValue(data, 'db_password', form.value.password)
  if (data.root_password) setDataValue(data, 'root_password', form.value.rootPassword)

  const environment = JSON.parse(JSON.stringify(template.environment))
  const memoryBytes = 512 * 1024 * 1024
  environment.hostConfig = { Memory: memoryBytes, MemorySwap: memoryBytes, NanoCpus: 1e9, PidsLimit: 512 }
  const request = {
    ...template,
    name: `${props.server.name.slice(0, 31)} database`,
    node: node.id,
    environment,
    users: [self.username],
    data
  }
  return await api.server.create(request)
}

async function save() {
  if (saving.value || !canManage.value) return
  saving.value = true
  error.value = ''
  startFailed.value = false
  let databaseServerId = ''
  let createdDatabaseServerId = ''
  try {
    const status = await props.server.getStatus()
    if (status !== 'offline') throw new Error(t('servers.database.stopServerFirst'))

    let host = form.value.host.trim()
    let port = Number(form.value.port)
    if (!externalMode.value) {
      databaseServerId = form.value.databaseServerId
      if (!databaseServerId) {
        if (!canCreateServer.value) throw new Error(t('servers.database.createPermissionRequired'))
        createdDatabaseServerId = await createDatabaseServer()
        databaseServerId = createdDatabaseServerId
        form.value.databaseServerId = databaseServerId
        databaseServerNeedsStart.value = true
      }
      host = parentNode.value.privateHost
      port = 0
    } else if (!host || !Number.isInteger(port) || port < 1 || port > 65535) {
      throw new Error(t('servers.database.invalidExternalAddress'))
    }

    linked.value = await props.server.saveDatabase({
      databaseServerId: externalMode.value ? '' : databaseServerId,
      engine: form.value.engine,
      accessMode: form.value.accessMode,
      host,
      port,
      databaseName: form.value.databaseName.trim(),
      username: form.value.username.trim(),
      password: form.value.password,
      variableMapping: form.value.variableMapping
    })

    if (databaseServerNeedsStart.value && databaseServerId) {
      try {
        await api.server.start(databaseServerId)
        databaseServerNeedsStart.value = false
      } catch {
        startFailed.value = true
      }
    }
    editing.value = false
    await load()
    toast.success(t('servers.database.saved'))
  } catch (failure) {
    error.value = failureMessage(failure)
    if (createdDatabaseServerId) error.value += ` (${t('servers.database.createdServerId')}: ${createdDatabaseServerId})`
  } finally {
    saving.value = false
  }
}

async function detach() {
  if (saving.value) return
  saving.value = true
  error.value = ''
  try {
    const status = await props.server.getStatus()
    if (status !== 'offline') throw new Error(t('servers.database.stopServerFirst'))
    await props.server.deleteDatabase()
    linked.value = null
    connection.value = null
    editing.value = false
    toast.success(t('servers.database.detached'))
  } catch (failure) {
    error.value = failureMessage(failure)
  } finally {
    saving.value = false
  }
}

async function retryStart() {
  if (saving.value || !linked.value?.databaseServerId) return
  saving.value = true
  error.value = ''
  try {
    await api.server.start(linked.value.databaseServerId)
    databaseServerNeedsStart.value = false
    startFailed.value = false
    toast.success(t('servers.database.saved'))
  } catch (failure) {
    error.value = failureMessage(failure)
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="database-manager">
    <h2 v-text="t('servers.database.title')" />
    <loader v-if="loading" />
    <p v-else-if="loadError" class="error" role="alert">{{ loadError }}</p>
    <template v-else>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <template v-if="linked && connection && !editing">
      <dl class="connection-details">
        <dt>{{ t('servers.database.engine') }}</dt><dd>{{ connection.engine }}</dd>
        <dt>{{ t('servers.database.host') }}</dt><dd>{{ connection.host }}:{{ connection.port }}</dd>
        <dt>{{ t('servers.database.databaseName') }}</dt><dd>{{ connection.databaseName }}</dd>
        <dt>{{ t('servers.database.username') }}</dt><dd>{{ connection.username }}</dd>
        <dt>{{ t('servers.database.password') }}</dt>
        <dd class="secret-value">
          <code>{{ showPassword ? connection.password : '************' }}</code>
          <btn variant="icon" :tooltip="t(showPassword ? 'servers.database.hidePassword' : 'servers.database.showPassword')" @click="showPassword = !showPassword">
            <icon :name="showPassword ? 'eye-off' : 'eye'" />
          </btn>
        </dd>
        <dt>{{ t('servers.database.accessMode') }}</dt><dd>{{ t(`servers.database.modes.${connection.accessMode}`) }}</dd>
        <dt v-if="linked.databaseServerId">{{ t('servers.database.databaseServer') }}</dt>
        <dd v-if="linked.databaseServerId"><router-link :to="{ name: 'ServerView', params: { id: linked.databaseServerId } }">{{ t('servers.database.openServer') }}</router-link></dd>
        <dt>{{ t('servers.database.environmentVariables') }}</dt>
        <dd class="mapping-list">
          <span v-for="(name, role) in connection.variableMapping" :key="role"><code>{{ name }}</code>: {{ t(`servers.database.roles.${role}`) }}</span>
        </dd>
      </dl>
      <div v-if="startFailed" class="start-failed" role="alert">
        <p class="error" v-text="t('servers.database.startFailed')" />
        <btn :disabled="saving" @click="retryStart"><icon name="play" />{{ t('servers.database.retryStart') }}</btn>
      </div>
      <p v-if="server.type === 'hypervm'" class="hint" v-text="t('servers.database.virtualMachineHint')" />
      <p class="hint" v-text="t('servers.database.detachHint')" />
      <btn :disabled="saving" @click="editing = true"><icon name="edit" />{{ t('servers.database.edit') }}</btn>
      <btn color="error" :disabled="saving" @click="detach"><icon name="link-off" />{{ t('servers.database.detach') }}</btn>
    </template>
    <form v-else @submit.prevent="save">
      <label>{{ t('servers.database.engine') }}
        <select v-model="form.engine" :disabled="!!form.databaseServerId"><option value="mariadb">MariaDB</option><option value="postgres">PostgreSQL</option></select>
      </label>
      <label>{{ t('servers.database.accessMode') }}
        <select v-model="form.accessMode" :disabled="!!form.databaseServerId">
          <option value="private">{{ t('servers.database.modes.private') }}</option>
          <option value="public">{{ t('servers.database.modes.public') }}</option>
          <option value="external">{{ t('servers.database.modes.external') }}</option>
        </select>
      </label>
      <template v-if="externalMode">
        <label>{{ t('servers.database.host') }}<input v-model="form.host" maxlength="253" required></label>
        <label>{{ t('servers.database.port') }}<input v-model.number="form.port" type="number" min="1" max="65535" required></label>
      </template>
      <template v-else>
        <p v-if="!canCreateServer" class="hint" v-text="t('servers.database.createPermissionRequired')" />
      </template>
      <label>{{ t('servers.database.databaseName') }}<input v-model="form.databaseName" :disabled="!!form.databaseServerId" maxlength="32" pattern="[A-Za-z0-9_]+" required></label>
      <label>{{ t('servers.database.username') }}<input v-model="form.username" :disabled="!!form.databaseServerId" maxlength="32" pattern="[A-Za-z0-9_]+" required></label>
      <label>{{ t('servers.database.password') }}
        <span class="password-field"><input v-model="form.password" :disabled="!!form.databaseServerId" type="password" minlength="8" required><btn v-if="!form.databaseServerId" type="button" @click="form.password = randomPassword()"><icon name="refresh" />{{ t('servers.database.generate') }}</btn></span>
      </label>
      <label v-if="!externalMode && form.engine === 'mariadb' && !form.databaseServerId">{{ t('servers.database.rootPassword') }}
        <span class="password-field"><input v-model="form.rootPassword" type="password" minlength="8" required><btn type="button" @click="form.rootPassword = randomPassword()"><icon name="refresh" />{{ t('servers.database.generate') }}</btn></span>
      </label>
      <div class="variable-mapping">
        <h3>{{ t('servers.database.environmentVariables') }}</h3>
        <label>{{ t('servers.database.roles.host') }}<input v-model="form.variableMapping.host" required></label>
        <label>{{ t('servers.database.roles.port') }}<input v-model="form.variableMapping.port" required></label>
        <label>{{ t('servers.database.roles.database') }}<input v-model="form.variableMapping.database" required></label>
        <label>{{ t('servers.database.roles.username') }}<input v-model="form.variableMapping.username" required></label>
        <label>{{ t('servers.database.roles.password') }}<input v-model="form.variableMapping.password" required></label>
      </div>
      <p class="hint" v-text="t('servers.database.privateHint')" />
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <btn color="primary" type="submit" :disabled="saving || (!externalMode && !form.databaseServerId && !canCreateServer)">
        <icon :name="saving ? 'loading' : 'database-plus'" :spin="saving" />{{ t(editing ? 'servers.database.save' : 'servers.database.create') }}
      </btn>
      <btn v-if="editing" type="button" :disabled="saving" @click="editing = false"><icon name="close" />{{ t('common.Cancel') }}</btn>
    </form>
    </template>
  </section>
</template>

<style scoped>
.database-manager { max-width: 720px; }
form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
label { display: grid; gap: 5px; min-width: 0; color: var(--color-text-secondary); }
input, select { width: 100%; min-width: 0; padding: 9px; border: 1px solid var(--color-background); border-radius: 5px; background: var(--color-background-secondary); color: var(--color-text); }
.variable-mapping, .hint, .error, form > button { grid-column: 1 / -1; }
.variable-mapping { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px 12px; }
.variable-mapping h3 { grid-column: 1 / -1; }
.hint { color: var(--color-text-secondary); }
.error { color: var(--color-error); }
.password-field { display: flex; gap: 8px; min-width: 0; }
.password-field input { flex: 1; }
.connection-details { display: grid; grid-template-columns: minmax(130px, 1fr) minmax(0, 2fr); gap: 8px 16px; }
.connection-details dt { color: var(--color-text-secondary); }
.connection-details dd { min-width: 0; margin: 0; overflow-wrap: anywhere; }
.secret-value { display: flex; align-items: center; gap: 8px; }
.mapping-list { display: grid; gap: 4px; }
@media (max-width: 600px) {
  form, .variable-mapping { grid-template-columns: minmax(0, 1fr); }
  .connection-details { grid-template-columns: minmax(0, 1fr); gap: 4px; }
  .connection-details dd { margin-bottom: 8px; }
}
</style>