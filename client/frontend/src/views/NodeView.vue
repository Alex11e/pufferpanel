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
const isLocalNode = ref(false)
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
const nodeLoaded = ref(false)
const loadError = ref('')
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
  try {
    const node = await api.node.get(route.params.id)
    isLocalNode.value = node.isLocal
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
    withPrivateHost.value = !(node.publicHost === node.privateHost && node.publicPort === node.privatePort)
    const [allocationResult, deploymentResult] = await Promise.allSettled([
      api.node.allocations(route.params.id),
      api.node.deployment(route.params.id)
    ])
    if (allocationResult.status === 'fulfilled') allocations.value = allocationResult.value
    if (deploymentResult.status === 'fulfilled') deploymentData = deploymentResult.value
    if (route.query.created) {
      deploymentOpen.value = true
    }
    fetchFeatures()
  } catch (error) {
    loadError.value = error.message || String(error)
  } finally {
    nodeLoaded.value = true
  }
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
	if (isLocalNode.value) return validRange()
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
  if (isLocalNode.value) {
    await api.node.update(route.params.id, {
      portRangeStart: portValue(portRangeStart.value),
      portRangeEnd: portValue(portRangeEnd.value),
      firewallEnabled: firewallEnabled.value,
      subdomainBase: subdomainBase.value
    })
    toast.success(t('nodes.Updated'))
    return
  }
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
  <loader v-if="!nodeLoaded" />
  <p v-else-if="loadError" class="load-error" role="alert">{{ loadError }}</p>
  <div v-else class="nodeview">
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
    <div class="edit">
      <template v-if="!isLocalNode">
      <text-field v-model="name" class="name" :label="t('common.Name')" />
      <text-field v-model="publicHost" class="public-host" :label="t('nodes.PublicHost')" />
      <text-field v-model="publicPort" class="public-port" :label="t('nodes.PublicPort')" type="number" />
      <toggle v-model="withPrivateHost" class="private-toggle" :label="t('nodes.WithPrivateAddress')" :hint="t('nodes.WithPrivateAddressHint')" />
      <text-field v-if="withPrivateHost" v-model="privateHost" class="private-host" :label="t('nodes.PrivateHost')" />
      <text-field v-if="withPrivateHost" v-model="privatePort" class="private-port" :label="t('nodes.PrivatePort')" type="number" />
      <text-field v-model="sftpPort" class="sftp-port" :label="t('nodes.SftpPort')" type="number" />
      </template>
      <p v-else class="local-node-hint" v-text="t('nodes.LocalNodeEdit')" />
      <h3 class="port-allocation-title" v-text="t('nodes.PortAllocation')" />
      <text-field v-model="portRangeStart" class="port-range-start" :label="t('nodes.PortRangeStart')" type="number" />
      <text-field v-model="portRangeEnd" class="port-range-end" :label="t('nodes.PortRangeEnd')" type="number" />
      <toggle v-model="firewallEnabled" class="local-firewall" :label="t('nodes.FirewallEnabled')" :hint="t('nodes.FirewallHint')" />
      <text-field v-model="subdomainBase" class="subdomain-base" :label="t('nodes.SubdomainBase')" :hint="t('nodes.SubdomainHint')" />
      <div class="allocations">
        <div class="allocation-entry">
          <text-field v-model="allocationServerId" :label="t('nodes.ServerId')" />
          <btn :disabled="!allocationServerId" @click="addAllocation()"><icon name="plus" />{{ t('nodes.AddPort') }}</btn>
        </div>
        <div class="allocation-list">
          <div v-for="allocation in allocations" :key="allocation.id" class="allocation-row">
            <span>{{ allocation.port }} · {{ allocation.protocols }} · {{ allocation.serverId }}</span>
            <btn variant="icon" :title="t('nodes.ReleasePort')" @click="releaseAllocation(allocation)"><icon name="remove" /></btn>
          </div>
        </div>
        <div class="allocation-summary" v-text="allocationSummary()" />
      </div>
      <btn class="node-submit" :disabled="!canSubmit()" color="primary" @click="submit()"><icon name="save" />{{ t('nodes.Update') }}</btn>
      <template v-if="!isLocalNode">
        <btn color="error" @click="deleteNode()"><icon name="remove" />{{ t('nodes.Delete') }}</btn>
        <btn @click="deploymentOpen = true" v-text="t('nodes.Deploy')" />
      </template>
    </div>
    <overlay v-model="deploymentOpen" closable :title="t('nodes.Deploy')" @close="closeDeploy()">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div v-html="markdown(t(`nodes.deploy.Step${currentStep}`, { config: getDeployConfig() }))" />
      <btn v-if="currentStep < 5" @click="currentStep += 1" v-text="t('common.Next')" />
      <btn v-else @click="closeDeploy()" v-text="t('common.Close')" />
    </overlay>
  </div>
</template>

<style scoped>
.nodeview .edit .local-node-hint,
.nodeview .edit .port-allocation-title,
.nodeview .edit .allocations,
.nodeview .edit .node-submit { grid-column: 1 / -1; }
.nodeview .edit .local-node-hint { margin: 0; color: var(--color-text-secondary); }
.nodeview .edit .port-allocation-title { margin: .75rem 0 0; }
.nodeview .edit .port-range-start,
.nodeview .edit .port-range-end,
.nodeview .edit .local-firewall,
.nodeview .edit .subdomain-base { grid-column: span 6; min-width: 0; }
.nodeview .edit .allocations { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; }
.allocation-entry { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: end; gap: .75rem; min-width: 0; }
.allocation-list { display: grid; gap: .5rem; min-width: 0; }
.allocation-row { display: flex; align-items: center; justify-content: space-between; gap: .75rem; min-width: 0; padding: .45rem .75rem; border-radius: .35rem; background: var(--color-background-secondary); }
.allocation-row span { min-width: 0; overflow-wrap: anywhere; }
.allocation-summary { color: var(--color-text-secondary); text-align: right; }
.node-submit { justify-self: start; }

@media (max-width: 700px) {
  .nodeview .edit .port-range-start,
  .nodeview .edit .port-range-end,
  .nodeview .edit .local-firewall,
  .nodeview .edit .subdomain-base { grid-column: 1 / -1; }
  .allocation-entry { grid-template-columns: minmax(0, 1fr); }
  .allocation-entry :deep(button) { justify-self: start; }
}
</style>
