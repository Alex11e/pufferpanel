<script setup>
import { ref, inject } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import TextField from '@/components/ui/TextField.vue'
import Toggle from '@/components/ui/Toggle.vue'

const api = inject('api')
const toast = inject('toast')
const { t } = useI18n()
const router = useRouter()

const withPrivateHost = ref(false)
const name = ref('')
const publicHost = ref('')
const publicPort = ref('8080')
const privateHost = ref('')
const privatePort = ref('8080')
const sftpPort = ref('5657')
const portRangeStart = ref('1000')
const portRangeEnd = ref('8000')
const firewallEnabled = ref(false)
const subdomainBase = ref('')

function portValue(value) {
  const port = Number(value)
  return Number.isInteger(port) && port >= 1 && port <= 65535 ? port : null
}

function validRange() {
  const start = portValue(portRangeStart.value)
  const end = portValue(portRangeEnd.value)
  return start !== null && end !== null && start <= end
}

function canCreate() {
  if (!name.value) return false
  if (!publicHost.value) return false
  if (portValue(publicPort.value) === null) return false
  if (portValue(sftpPort.value) === null) return false
  if (!validRange()) return false
  if (withPrivateHost.value) {
    if (!privateHost.value) return false
    if (portValue(privatePort.value) === null) return false
  }
  return true
}

async function create() {
  if (!canCreate()) return
  const node = {
    name: name.value,
    publicHost: publicHost.value,
    publicPort: portValue(publicPort.value),
    sftpPort: portValue(sftpPort.value),
    portRangeStart: portValue(portRangeStart.value),
    portRangeEnd: portValue(portRangeEnd.value),
    firewallEnabled: firewallEnabled.value,
    subdomainBase: subdomainBase.value
  }
  if (withPrivateHost.value) {
    node.privateHost = privateHost.value
    node.privatePort = portValue(privatePort.value)
  } else {
    node.privateHost = publicHost.value
    node.privatePort = portValue(publicPort.value)
  }
  const id = await api.node.create(node)
  toast.success(t('nodes.Created'))
  router.push({ name: 'NodeView', params: { id }, query: { created: true } })
}
</script>

<template>
  <div class="nodecreate">
    <h1 v-text="t('nodes.Create')" />
    <text-field v-model="name" autofocus class="name" :label="t('common.Name')" />
    <text-field v-model="publicHost" class="public-host" :label="t('nodes.PublicHost')" />
    <text-field v-model="publicPort" class="public-port" :label="t('nodes.PublicPort')" type="number" />
    <toggle v-model="withPrivateHost" class="private-toggle" :label="t('nodes.WithPrivateAddress')" :hint="t('nodes.WithPrivateAddressHint')" />
    <text-field v-if="withPrivateHost" v-model="privateHost" class="private-host" :label="t('nodes.PrivateHost')" />
    <text-field v-if="withPrivateHost" v-model="privatePort" class="private-port" :label="t('nodes.PrivatePort')" type="number" />
    <text-field v-model="sftpPort" class="sftp-port" :label="t('nodes.SftpPort')" type="number" />
    <h2 v-text="t('nodes.PortAllocation')" />
    <text-field v-model="portRangeStart" :label="t('nodes.PortRangeStart')" type="number" />
    <text-field v-model="portRangeEnd" :label="t('nodes.PortRangeEnd')" type="number" />
    <toggle v-model="firewallEnabled" :label="t('nodes.FirewallEnabled')" :hint="t('nodes.FirewallHint')" />
    <text-field v-model="subdomainBase" :label="t('nodes.SubdomainBase')" :hint="t('nodes.SubdomainHint')" />
    <btn :disabled="!canCreate()" color="primary" @click="create()"><icon name="save" />{{ t('nodes.Create') }}</btn>
  </div>
</template>
