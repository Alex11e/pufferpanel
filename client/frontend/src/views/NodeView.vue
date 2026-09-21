<script setup>
import { ref, inject, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import markdown from '@/utils/markdown'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import Overlay from '@/components/ui/Overlay.vue'
import TextField from '@/components/ui/TextField.vue'
import Toggle from '@/components/ui/Toggle.vue'

const api = inject('api')
const toast = inject('toast')
const events = inject('events')
const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const deploymentOpen = ref(false)
let deploymentData = {}
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
const allocations = ref([])
const allocationServerId = ref('')
const currentStep = ref(1)
const featuresFetched = ref(null)
const features = ref({})

function portValue(value) {
  const port = Number(value)
  return Number.isInteger(port) && port >= 1 && port <= 65535 ? port : null
}

function validRange() {
  const start = portValue(portRangeStart.value)
  const end = portValue(portRangeEnd.value)
  return start !== null && end !== null && start <= end
}

onMounted(async () => {
  const node = await api.node.get(route.params.id)
  name.value = node.name
  publicHost.value = node.publicHost
  publicPort.value = node.publicPort
  privateHost.value = node.privateHost
  privatePort.value = node.privatePort
  sftpPort.value = node.sftpPort
  portRangeStart.value = node.portRangeStart || 1000
  portRangeEnd.value = node.portRangeEnd || 8000
  firewallEnabled.value = node.firewallEnabled
  subdomainBase.value = node.subdomainBase
  allocations.value = await api.node.allocations(route.params.id)
  withPrivateHost.value = !(node.publicHost === node.privateHost && node.publicPort === node.privatePort)
  deploymentData = await api.node.deployment(route.params.id)
  if (route.query.created) {
    deploymentOpen.value = true
  }

  fetchFeatures()
})

async function fetchFeatures() {
  featuresFetched.value = null
  features.value = {}
  try {
    const f = await api.node.features(route.params.id)
    features.value.envs = [ ...new Set(f.environments.map(e => e === 'standard' || e === 'tty' ? 'host' : e)) ].map(e => t(`env.${e}.name`))
    features.value.docker = f.features.indexOf('docker') !== -1
    features.value.os = f.os
    features.value.arch = f.arch
    featuresFetched.value = true
  } catch(e) {
    featuresFetched.value = false
  }
}

function canSubmit() {
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

async function submit() {
  if (!canSubmit()) return
  const node = {
    name: name.value,
    publicHost: publicHost.value,
    publicPort: portValue(publicPort.value),
    sftpPort: portValue(sftpPort.value),
    portRangeStart: portValue(portRangeStart.value),
    portRangeEnd: portValue(portRangeEnd.value),
    firewallEnabled: firewallEnabled.value, subdomainBase: subdomainBase.value
  }
  if (withPrivateHost.value) {
    node.privateHost = privateHost.value
    node.privatePort = portValue(privatePort.value)
  } else {
    node.privateHost = publicHost.value
    node.privatePort = portValue(publicPort.value)
  }
  await api.node.update(route.params.id, node)
  toast.success(t('nodes.Updated'))
}

async function addAllocation() {
  if (!allocationServerId.value) return
  const allocation = await api.node.allocatePort(route.params.id, allocationServerId.value)
  allocations.value.push(allocation)
  allocationServerId.value = ''
  toast.success(t('nodes.PortAllocated', { port: allocation.port }))
}

async function releaseAllocation(allocation) {
  await api.node.releasePort(route.params.id, allocation.id)
  allocations.value = allocations.value.filter(item => item.id !== allocation.id)
  toast.success(t('nodes.PortReleased', { port: allocation.port }))
}

function allocationSummary() {
  const total = Number(portRangeEnd.value) - Number(portRangeStart.value) + 1
  const used = allocations.value.length
  return `${used} / ${total} ${t('nodes.PortsUsed')}`
}

async function deleteNode() {
  events.emit(
    'confirm',
    t('nodes.ConfirmDelete', { name: name.value }),
    {
      text: t('nodes.Delete'),
      icon: 'remove',
      color: 'error',
      action: async () => {
        await api.node.delete(route.params.id)
        toast.success(t('nodes.Deleted'))
        router.push({ name: 'NodeList' })
      }
    },
    {
      color: 'primary'
    }
  )
}

function getDeployConfig() {
  const config = {
    logs: '/var/log/pufferpanel',
    web: {
      host: `0.0.0.0:${privatePort.value}`
    },
    token: {
      public: location.origin + '/auth/publickey'
    },
    panel: {
      enable: false
    },
    daemon: {
      auth: {
        url: location.origin + '/oauth2/token',
        ...deploymentData
      },
      data: {
        root: '/var/lib/pufferpanel'
      },
      sftp: {
        host: `0.0.0.0:${sftpPort.value}`
      }
    }
  }
  return JSON.stringify(config, undefined, 2)
}

function closeDeploy() {
  deploymentOpen.value = false
  currentStep.value = 1
  fetchFeatures()
}
</script>

<template>
  <div class="nodeview">
    <h1 v-text="name" />
    <loader v-if="featuresFetched === null" />
    <div v-else-if="featuresFetched === false" class="features">
      <div class="unreachable" v-text="t('nodes.Unreachable')" />
    </div>
    <div v-else class="features">
      <div class="reachable" v-text="t('nodes.Reachable')" />
      <div class="os">
        <span v-text="t('nodes.features.os.label')" />
        <span v-text="t('nodes.features.os.' + features.os)" />
      </div>
      <div class="arch">
        <span v-text="t('nodes.features.arch.label')" />
        <span v-text="t('nodes.features.arch.' + features.arch)" />
      </div>
      <div class="env">
        <span v-text="t('nodes.features.envs')" />
        <span v-text="features.envs.join(', ')" />
      </div>
      <div class="docker">
        <span v-text="t('env.docker.name')" />
        <span v-text="t('nodes.features.docker.' + features.docker)" />
      </div>
    </div>
    <h2 v-text="t('nodes.Edit')" />
    <div v-if="route.params.id > 0" class="edit">
      <text-field v-model="name" class="name" :label="t('common.Name')" />
      <text-field v-model="publicHost" class="public-host" :label="t('nodes.PublicHost')" />
      <text-field v-model="publicPort" class="public-port" :label="t('nodes.PublicPort')" type="number" />
      <toggle v-model="withPrivateHost" class="private-toggle" :label="t('nodes.WithPrivateAddress')" :hint="t('nodes.WithPrivateAddressHint')" />
      <text-field v-if="withPrivateHost" v-model="privateHost" class="private-host" :label="t('nodes.PrivateHost')" />
      <text-field v-if="withPrivateHost" v-model="privatePort" class="private-port" :label="t('nodes.PrivatePort')" type="number" />
      <text-field v-model="sftpPort" class="sftp-port" :label="t('nodes.SftpPort')" type="number" />
      <h3 v-text="t('nodes.PortAllocation')" />
      <text-field v-model="portRangeStart" :label="t('nodes.PortRangeStart')" type="number" />
      <text-field v-model="portRangeEnd" :label="t('nodes.PortRangeEnd')" type="number" />
      <toggle v-model="firewallEnabled" :label="t('nodes.FirewallEnabled')" :hint="t('nodes.FirewallHint')" />
      <text-field v-model="subdomainBase" :label="t('nodes.SubdomainBase')" :hint="t('nodes.SubdomainHint')" />
      <div class="allocations">
        <text-field v-model="allocationServerId" :label="t('nodes.ServerId')" />
        <btn :disabled="!allocationServerId" @click="addAllocation()"><icon name="plus" />{{ t('nodes.AddPort') }}</btn>
        <div v-for="allocation in allocations" :key="allocation.id" class="subline">
          {{ allocation.port }} · {{ allocation.protocols }} · {{ allocation.serverId }}
          <btn variant="icon" :title="t('nodes.ReleasePort')" @click="releaseAllocation(allocation)"><icon name="remove" /></btn>
        </div>
        <div class="subline" v-text="allocationSummary()" />
      </div>
      <btn :disabled="!canSubmit()" color="primary" @click="submit()"><icon name="save" />{{ t('nodes.Update') }}</btn>
      <btn color="error" @click="deleteNode()"><icon name="remove" />{{ t('nodes.Delete') }}</btn>
      <btn @click="deploymentOpen = true" v-text="t('nodes.Deploy')" />
    </div>
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div v-else class="edit" v-html="markdown(t('nodes.LocalNodeEdit'))" />
    <overlay v-model="deploymentOpen" closable :title="t('nodes.Deploy')" @close="closeDeploy()">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div v-html="markdown(t(`nodes.deploy.Step${currentStep}`, { config: getDeployConfig() }))" />
      <btn v-if="currentStep < 5" @click="currentStep += 1" v-text="t('common.Next')" />
      <btn v-else @click="closeDeploy()" v-text="t('common.Close')" />
    </overlay>
  </div>
</template>
