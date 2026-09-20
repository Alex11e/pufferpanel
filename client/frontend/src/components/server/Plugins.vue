<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import TextField from '@/components/ui/TextField.vue'

const { t } = useI18n()
const api = inject('api')
const events = inject('events')
const toast = inject('toast')
const props = defineProps({ server: { type: Object, required: true } })

const plugins = ref(null)
const pluginUrl = ref('')
const installing = ref(false)
const uploading = ref(false)
const input = ref(null)
const canEdit = computed(() => props.server.hasScope('server.files.edit'))

onMounted(refresh)

async function refresh() {
  try {
    const files = await props.server.getFile('plugins')
    plugins.value = files.filter(file => file.isFile && file.name.toLowerCase().endsWith('.jar')).sort((a, b) => a.name.localeCompare(b.name))
  } catch (error) {
    plugins.value = []
  }
}

function selectFiles() {
  input.value.click()
}

async function upload(event) {
  const files = Array.from(event.target.files).filter(file => file.name.toLowerCase().endsWith('.jar'))
  event.target.value = ''
  if (files.length === 0) return
  uploading.value = true
  try {
    await props.server.createFolder('plugins')
    for (const file of files) await props.server.uploadFile(`plugins/${file.name}`, file)
    toast.success(t('servers.PluginUploaded'))
    await refresh()
  } finally {
    uploading.value = false
  }
}

async function installFromUrl() {
  if (!pluginUrl.value.trim()) return
  installing.value = true
  try {
    await api.post(`/api/servers/${props.server.id}/plugins/download`, { url: pluginUrl.value.trim() })
    pluginUrl.value = ''
    toast.success(t('servers.PluginInstalled'))
    await refresh()
  } finally {
    installing.value = false
  }
}

function deletePlugin(plugin) {
  events.emit('confirm', t('servers.ConfirmPluginDelete', { name: plugin.name }), {
    text: t('files.Delete'), icon: 'remove', color: 'error', action: async () => {
      await props.server.deleteFile(`plugins/${plugin.name}`)
      toast.success(t('servers.PluginDeleted'))
      await refresh()
    }
  }, { color: 'primary' })
}

async function restart() {
  await props.server.restart()
}
</script>

<template>
  <div class="plugin-manager">
    <div class="group-header">
      <div class="title">
        <h2 v-text="t('servers.Plugins')" />
        <p v-text="t('servers.PluginsHint')" />
      </div>
      <btn variant="icon" :tooltip="t('files.Refresh')" @click="refresh"><icon name="reload" /></btn>
    </div>

    <div v-if="canEdit" class="plugin-actions">
      <btn color="primary" :disabled="uploading" @click="selectFiles">
        <icon :name="uploading ? 'loading' : 'file-upload'" :spin="uploading" /> {{ t('servers.PluginUpload') }}
      </btn>
      <input ref="input" type="file" accept=".jar,application/java-archive" multiple @change="upload" />
      <div class="plugin-url">
        <text-field v-model="pluginUrl" :label="t('servers.PluginUrl')" :hint="t('servers.PluginUrlHint')" type="url" @keyup.enter="installFromUrl" />
        <btn color="primary" :disabled="installing || !pluginUrl.trim()" @click="installFromUrl">
          <icon :name="installing ? 'loading' : 'download'" :spin="installing" /> {{ t('servers.PluginInstall') }}
        </btn>
      </div>
    </div>

    <loader v-if="plugins === null" />
    <div v-else-if="plugins.length === 0" class="alert info" v-text="t('servers.NoPlugins')" />
    <div v-else class="plugin-list">
      <div v-for="plugin in plugins" :key="plugin.name" class="plugin">
        <icon name="file" />
        <span class="name" v-text="plugin.name" />
        <span class="size">{{ Math.ceil(plugin.size / 1024) }} KiB</span>
        <btn v-if="canEdit" variant="icon" :tooltip="t('files.Delete')" @click="deletePlugin(plugin)"><icon name="remove" /></btn>
      </div>
    </div>

    <btn v-if="server.hasScope('server.start') && server.hasScope('server.stop')" variant="text" @click="restart"><icon name="restart" /> {{ t('servers.Restart') }}</btn>
  </div>
</template>

<style scoped>
.plugin-actions, .plugin-url { display: flex; gap: 1rem; align-items: flex-end; flex-wrap: wrap; margin-bottom: 1.25rem; }
.plugin-url { flex: 1; }
.plugin-url :deep(.text-field) { flex: 1; min-width: min(100%, 20rem); }
input[type="file"] { display: none; }
.plugin-list { display: grid; gap: .5rem; margin-bottom: 1rem; }
.plugin { display: flex; align-items: center; gap: .75rem; padding: .8rem 1rem; background: var(--background-secondary); border-radius: .4rem; }
.plugin .name { flex: 1; overflow-wrap: anywhere; }
.plugin .size { color: var(--text-muted); font-size: .85em; }
</style>
