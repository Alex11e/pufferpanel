<script setup>
import { defineAsyncComponent, inject, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

import ServerHeader from './Header.vue'
import HostingOverview from './HostingOverview.vue'

import Tab from '@/components/ui/Tab.vue'
import Tabs from '@/components/ui/Tabs.vue'
import Loader from '@/components/ui/Loader.vue'

const load = loader => defineAsyncComponent({ loader, loadingComponent: Loader })
const Console = load(() => import('./Console.vue'))
const Stats = load(() => import('./Stats.vue'))
const Files = load(() => import('./Files.vue'))
const Settings = load(() => import('./Settings.vue'))
const Users = load(() => import('./Users.vue'))
const Sftp = load(() => import('./Sftp.vue'))
const Network = load(() => import('./Network.vue'))
const Backup = load(() => import('./Backup.vue'))
const Admin = load(() => import('./Admin.vue'))
const Activity = load(() => import('./Activity.vue'))
const Database = load(() => import('./Database.vue'))

const props = defineProps({
  server: { type: Object, required: true },
  mode: { type: String, default: 'web' }
})

const { t } = useI18n()
const events = inject('events')
const route = useRoute()
const router = useRouter()

let task = null
onMounted(() => {
  if (route.query.created && props.server.hasScope('server.install')) {
    events.emit(
      'confirm',
      { title: t('servers.InstallPrompt'), body: t('servers.InstallPromptBody') },
      { text: t('servers.Install'), icon: 'install', action: () => props.server.install() },
      { color: 'neutral' }
    )
    router.push({ query: {}, hash: route.hash })
  }
  task = props.server.startTask(() => {}, 5000)
})

onUnmounted(() => {
  if (task) props.server.stopTask(task)
})
</script>

<template>
  <div>
    <server-header :server="server" />

    <tabs anchors>
      <tab id="overview" :title="mode === 'web' ? 'Webtárhely' : 'Adatbázis'" icon="server" hotkey="t v">
        <hosting-overview :server="server" :mode="mode" />
      </tab>
      <tab v-if="server.hasScope('server.admin')" id="database" :title="t('servers.database.title')" icon="database">
        <Database :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.console') || server.hasScope('server.console.send')" id="console" :title="t('servers.Console')" icon="console" hotkey="t c">
        <Console :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.stats')" id="stats" :title="t('servers.Statistics')" icon="stats" hotkey="t i">
        <stats :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.files.view')" id="files" :title="t('servers.Files')" icon="files" hotkey="t f">
        <files :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.view')" id="network" :title="t('servers.Network')" icon="server" hotkey="t n">
        <Network :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.data.view') || server.hasScope('server.flags.view')" id="settings" :title="t('servers.Settings')" icon="settings" hotkey="t s">
        <settings :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.users.view')" id="users" :title="t('users.Users')" icon="users" hotkey="t u">
        <users :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.sftp')" id="sftp" :title="t('servers.SFTPInfo')" icon="sftp" hotkey="t 6">
        <sftp :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.backup.view')" id="backups" :title="t('backup.Backup')" icon="backup" hotkey="t 7">
        <backup :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.view')" id="activity" :title="t('servers.Activity')" icon="stats" hotkey="t y">
        <activity :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.definition.view') || server.hasScope('server.delete')" id="admin" :title="t('servers.Admin')" icon="admin" hotkey="t a">
        <admin :server="server" />
      </tab>
    </tabs>
  </div>
</template>
