<script setup>
import { ref, defineAsyncComponent, inject, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

import ServerHeader from '../server/Header.vue'

import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import Tab from '@/components/ui/Tab.vue'
import Tabs from '@/components/ui/Tabs.vue'

const load = loader => defineAsyncComponent({ loader, loadingComponent: Loader })
const VpsOverview = load(() => import('../server/VpsOverview.vue'))
const Console = load(() => import('../server/Console.vue'))
const Stats = load(() => import('../server/Stats.vue'))
const Files = load(() => import('../server/Files.vue'))
const Settings = load(() => import('../server/Settings.vue'))
const Users = load(() => import('../server/Users.vue'))
const Sftp = load(() => import('../server/Sftp.vue'))
const Network = load(() => import('../server/Network.vue'))
const Backup = load(() => import('../server/Backup.vue'))
const Admin = load(() => import('../server/Admin.vue'))
const Activity = load(() => import('../server/Activity.vue'))
const Database = load(() => import('../server/Database.vue'))

const { t } = useI18n()
const events = inject('events')
const route = useRoute()
const router = useRouter()
const http = ref(false)
const httpWarnDismissed = ref(false)
let httpCount = 2

const props = defineProps({
  server: { type: Object, required: true }
})

let task = null
onMounted(() => {
  if (route.query.created && props.server.hasScope('server.install')) {
    events.emit(
      'confirm',
      {
        title: t('servers.InstallPrompt'),
        body: t('servers.InstallPromptBody')
      },
      {
        text: t('servers.Install'),
        icon: 'install',
        action: () => {
          props.server.install()
        }
      },
      {
        color: 'neutral'
      }
    )
    router.push({ query: {}, hash: route.hash })
  }

  task = props.server.startTask(() => {
    // avoid flicker of the info alert on a flaky connection
    if (props.server.needsPolling() && httpCount < 3) httpCount += 1
    if (!props.server.needsPolling() && httpCount > 0) httpCount -= 1
    if (httpCount === 3) http.value = true
    if (httpCount === 0) http.value = false
  }, 5000)
})

onUnmounted(() => {
  if (task) props.server.stopTask(task)
})
</script>

<template>
  <div :class="http ? 'http-fallback' : ''">
    <server-header :server="server" />

    <tabs anchors>
      <tab id="vps" title="VPS" icon="server" hotkey="t v">
        <vps-overview :server="server" />
      </tab>
      <tab
        v-if="server.hasScope('server.console') || server.hasScope('server.console.send')"
        id="console"
        title="QEMU konzol"
        icon="console"
        hotkey="t c"
      >
        <div v-if="http && !httpWarnDismissed" class="alert info">
          <span v-text="t('servers.SocketWarnConsole')" />
          <btn variant="icon" @click="httpWarnDismissed = true"><icon name="close" /></btn>
        </div>
        <Console :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.stats')" id="stats" :title="t('servers.Statistics')" icon="stats" hotkey="t i">
        <stats :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.files.view')" id="files" title="Fájlok és ISO-k" icon="files" hotkey="t f">
        <files :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.view')" id="network" :title="t('servers.Network')" icon="server" hotkey="t n">
        <Network :server="server" />
      </tab>
      <tab
        v-if="server.hasScope('server.data.view') || server.hasScope('server.flags.view')"
        id="settings"
        :title="t('servers.Settings')"
        icon="settings"
        hotkey="t s"
      >
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
      <tab v-if="server.hasScope('server.admin')" id="database" :title="t('servers.database.title')" icon="database">
        <Database :server="server" />
      </tab>
      <tab v-if="server.hasScope('server.view')" id="activity" :title="t('servers.Activity')" icon="stats" hotkey="t y">
        <activity :server="server" />
      </tab>
      <tab
        v-if="server.hasScope('server.definition.view') || server.hasScope('server.delete')"
        id="admin"
        :title="t('servers.Admin')"
        icon="admin"
        hotkey="t a"
      >
        <admin :server="server" />
      </tab>
    </tabs>
  </div>
</template>
