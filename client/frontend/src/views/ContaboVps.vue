<script setup>
import { computed, inject, onMounted, onUnmounted, ref, watch } from 'vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const events = inject('events')
const instances = ref([])
const images = ref([])
const firewalls = ref([])
const selectedId = ref('')
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const reinstallImage = ref('')
const reinstallUser = ref('root')
const sshSecretIds = ref('')
const rootPasswordSecret = ref('')
const userData = ref('')
const customImage = ref({ name: '', description: '', url: '', osType: 'Linux', version: '' })
const firewallName = ref('')
const firewallDescription = ref('')
const firewallStatus = ref('inactive')
const firewallRules = ref(`{
  "inbound": []
}`)
const firewallAssignments = ref({})
const instanceFilter = ref('')
const sshOutput = ref('')
const sshStatus = ref('Nincs kapcsolat')
let sshSocket = null
let sshDecoder = new TextDecoder()
const selectedInstance = computed(() => instances.value.find(instance => String(instance.instanceId) === selectedId.value))
const filteredInstances = computed(() => instances.value.filter(instance => {
  const label = `${instance.displayName || ''} ${instance.instanceId || ''} ${instance.name || ''} ${instance.ipConfig?.v4?.ip || ''}`.toLowerCase()
  return label.includes(instanceFilter.value.trim().toLowerCase())
}))

function unwrap(response) {
  const body = response?.data ?? response
  return body?.data ?? body
}

function errorMessage(failure) {
  return failure?.msg || failure?.message || failure?.response?.error?.msg || 'Ismeretlen hiba történt.'
}

function instanceIP(instance) {
  const ipConfig = instance?.ipConfig
  return ipConfig?.v4?.ip || ipConfig?.v4?.address || instance?.ipAddress || instance?.ip || ''
}

function imageLabel(image) {
  return `${image.name || image.imageId} · ${image.osType || 'OS'} ${image.version || ''}${image.standardImage ? ' · standard' : ' · custom'}`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [instanceResponse, imageResponse, firewallResponse] = await Promise.all([
      api.contabo.instances(),
      api.contabo.images(),
      api.contabo.firewalls()
    ])
    instances.value = unwrap(instanceResponse) || []
    images.value = unwrap(imageResponse) || []
    firewalls.value = unwrap(firewallResponse) || []
    firewallAssignments.value = Object.fromEntries(firewalls.value.map(firewall => [
      firewall.firewallId,
      JSON.stringify(firewall.rules || { inbound: [] }, null, 2)
    ]))
    if (!selectedId.value && instances.value.length) selectedId.value = String(instances.value[0].instanceId)
  } catch (failure) {
    error.value = errorMessage(failure)
  } finally {
    loading.value = false
  }
}

async function runAction(action) {
  if (!selectedInstance.value) return
  error.value = ''
  notice.value = ''
  saving.value = true
  try {
    await api.contabo.action(selectedInstance.value.instanceId, action)
    notice.value = 'A műveletet a Contabo elfogadta.'
    await load()
  } catch (failure) {
    error.value = errorMessage(failure)
  } finally {
    saving.value = false
  }
}

function confirmAction(action, message, label) {
  events.emit('confirm', message, {
    text: label,
    icon: 'apply',
    color: 'error',
    action: () => runAction(action)
  })
}

function sshCommand() {
  const ip = instanceIP(selectedInstance.value)
  if (!ip) return ''
  const user = selectedInstance.value.defaultUser || 'root'
  return `ssh ${user}@${ip}`
}

async function copySSHCommand() {
  const command = sshCommand()
  if (!command) return
  try {
    await navigator.clipboard.writeText(command)
    notice.value = 'Az SSH parancs a vágólapra került.'
  } catch {
    error.value = 'A vágólap nem érhető el ebben a böngészőben.'
  }
}

function confirmReinstall() {
  if (!selectedInstance.value || !reinstallImage.value) return
  events.emit('confirm', 'Az újratelepítés törli a VPS jelenlegi lemezének teljes tartalmát. Folytatod?', {
    text: 'Lemez törlése és telepítés',
    icon: 'apply',
    color: 'error',
    action: reinstall
  })
}

async function reinstall() {
  const sshKeys = sshSecretIds.value.split(',').map(value => Number(value.trim())).filter(Number.isInteger)
  const payload = {
    imageId: reinstallImage.value,
    defaultUser: reinstallUser.value,
    userData: userData.value,
    sshKeys,
    confirmWipe: true
  }
  if (rootPasswordSecret.value) payload.rootPassword = Number(rootPasswordSecret.value)
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    await api.contabo.reinstall(selectedInstance.value.instanceId, payload)
    notice.value = 'Az újratelepítést a Contabo elfogadta.'
    await load()
  } catch (failure) {
    error.value = errorMessage(failure)
  } finally {
    saving.value = false
  }
}

async function addCustomImage() {
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    await api.contabo.createImage(customImage.value)
    notice.value = 'A custom image letöltése elindult a Contabo-nál.'
    customImage.value = { name: '', description: '', url: '', osType: 'Linux', version: '' }
    await load()
  } catch (failure) {
    error.value = errorMessage(failure)
  } finally {
    saving.value = false
  }
}

async function createFirewall() {
  saving.value = true
  error.value = ''
  try {
    let rules
    try {
      rules = JSON.parse(firewallRules.value)
    } catch {
      error.value = 'A tűzfalszabály JSON formátuma hibás.'
      return
    }
    await api.contabo.createFirewall({
      name: firewallName.value.trim(),
      description: firewallDescription.value.trim(),
      status: firewallStatus.value,
      rules
    })
    firewallName.value = ''
    firewallDescription.value = ''
    notice.value = 'A tűzfal létrejött.'
    await load()
  } catch (failure) {
    error.value = errorMessage(failure)
  } finally {
    saving.value = false
  }
}

async function saveRules(firewall) {
  let rules
  try {
    rules = JSON.parse(firewallAssignments.value[firewall.firewallId] || '{}')
  } catch {
    error.value = 'A tűzfalszabály JSON formátuma hibás.'
    return
  }
  error.value = ''
  try {
    await api.contabo.updateFirewallRules(firewall.firewallId, rules)
    notice.value = `A(z) ${firewall.name} tűzfal szabályai frissültek.`
    await load()
  } catch (failure) {
    error.value = errorMessage(failure)
  }
}

async function assignFirewall(firewall) {
  if (!selectedInstance.value) return
  error.value = ''
  try {
    await api.contabo.assignFirewall(firewall.firewallId, selectedInstance.value.instanceId)
    notice.value = 'A tűzfal hozzá lett rendelve a VPS-hez.'
    await load()
  } catch (failure) {
    error.value = errorMessage(failure)
  }
}

async function buyFirewallAddon() {
  if (!selectedInstance.value) return
  events.emit('confirm', 'A Contabo firewall kiegészítő fizetős lehet. A rendelést a szolgáltató számlázza. Biztosan folytatod?', {
    text: 'Fizetős kiegészítő megrendelése',
    icon: 'apply',
    color: 'error',
    action: async () => {
      saving.value = true
      error.value = ''
      try {
        await api.contabo.enableFirewallAddon(selectedInstance.value.instanceId)
        notice.value = 'A firewall kiegészítés kérését a Contabo elfogadta.'
      } catch (failure) {
        error.value = errorMessage(failure)
      } finally {
        saving.value = false
      }
    }
  })
}

onMounted(load)
onUnmounted(closeSSH)
watch(selectedId, closeSSH)

function connectSSH() {
  if (!selectedInstance.value || sshSocket) return
  const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws'
  sshStatus.value = 'Kapcsolódás...'
  sshOutput.value = ''
  sshDecoder = new TextDecoder()
  sshSocket = new WebSocket(`${scheme}://${window.location.host}/api/contabo/instances/${selectedInstance.value.instanceId}/ssh`)
  sshSocket.binaryType = 'arraybuffer'
  sshSocket.addEventListener('open', () => { sshStatus.value = 'Kapcsolódva' })
  sshSocket.addEventListener('message', event => {
    sshOutput.value += typeof event.data === 'string' ? event.data : sshDecoder.decode(event.data, { stream: true })
    if (sshOutput.value.length > 200000) sshOutput.value = sshOutput.value.slice(-180000)
  })
  sshSocket.addEventListener('error', () => { sshStatus.value = 'Kapcsolati hiba' })
  sshSocket.addEventListener('close', () => {
    sshSocket = null
    sshStatus.value = 'Lezárva'
  })
}

function closeSSH() {
  if (sshSocket) {
    const socket = sshSocket
    sshSocket = null
    socket.close()
  }
}

function sendSSHKey(event) {
  if (!sshSocket || sshSocket.readyState !== WebSocket.OPEN) return
  let data = ''
  if (event.ctrlKey && event.key.toLowerCase() === 'c') data = '\u0003'
  else if (event.key === 'Enter') data = '\r'
  else if (event.key === 'Backspace') data = '\u007f'
  else if (event.key === 'Tab') data = '\t'
  else if (event.key === 'ArrowUp') data = '\u001b[A'
  else if (event.key === 'ArrowDown') data = '\u001b[B'
  else if (event.key === 'ArrowRight') data = '\u001b[C'
  else if (event.key === 'ArrowLeft') data = '\u001b[D'
  else if (event.key.length === 1 && !event.altKey && !event.ctrlKey && !event.metaKey) data = event.key
  if (data) {
    event.preventDefault()
    sshSocket.send(data)
  }
}

function pasteSSH(event) {
  if (!sshSocket || sshSocket.readyState !== WebSocket.OPEN) return
  event.preventDefault()
  sshSocket.send(event.clipboardData.getData('text'))
}
</script>

<template>
  <main class="contabo-page">
    <header class="heading">
      <div>
        <h1><icon name="node" /> Contabo VPS</h1>
        <p class="hint">Külső VPS-ek, rendszerek és tűzfalak kezelése</p>
      </div>
      <btn variant="icon" tooltip="Frissítés" :disabled="loading" @click="load"><icon name="reload" /></btn>
    </header>

    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice" role="status">{{ notice }}</p>
    <loader v-if="loading" />
    <template v-else>
      <section class="instances-section">
        <div class="section-heading">
          <h2>VPS-ek</h2>
          <input v-model="instanceFilter" type="search" placeholder="Név, azonosító vagy IP" aria-label="VPS keresése">
        </div>
        <p v-if="!instances.length" class="empty">Nem található VPS a Contabo-fiókban.</p>
        <div v-else class="instance-layout">
          <nav class="instance-list" aria-label="VPS lista">
            <button v-for="instance in filteredInstances" :key="instance.instanceId" :class="['instance-row', { selected: selectedId === String(instance.instanceId) }]" @click="selectedId = String(instance.instanceId)">
              <strong>{{ instance.displayName || instance.name || `VPS ${instance.instanceId}` }}</strong>
              <span>#{{ instance.instanceId }} · {{ instance.status || 'ismeretlen' }}</span>
              <span>{{ instanceIP(instance) || 'IP betöltés alatt' }}</span>
            </button>
          </nav>

          <section v-if="selectedInstance" class="instance-detail" aria-label="Kiválasztott VPS">
            <h2>{{ selectedInstance.displayName || selectedInstance.name || `VPS ${selectedInstance.instanceId}` }}</h2>
            <dl>
              <div><dt>Azonosító</dt><dd>{{ selectedInstance.instanceId }}</dd></div>
              <div><dt>Állapot</dt><dd>{{ selectedInstance.status || 'ismeretlen' }}</dd></div>
              <div><dt>IP-cím</dt><dd>{{ instanceIP(selectedInstance) || 'Nem elérhető' }}</dd></div>
              <div><dt>Régió</dt><dd>{{ selectedInstance.region || '-' }}</dd></div>
              <div><dt>Termék</dt><dd>{{ selectedInstance.productName || selectedInstance.productId || '-' }}</dd></div>
            </dl>
            <div class="actions">
              <btn v-if="selectedInstance.status !== 'running'" color="primary" :disabled="saving" @click="runAction('start')"><icon name="play" /> Indítás</btn>
              <btn :disabled="saving" @click="runAction('restart')"><icon name="reload" /> Újraindítás</btn>
              <btn :disabled="saving" @click="runAction('shutdown')"><icon name="stop" /> Leállítás</btn>
              <btn color="error" :disabled="saving" @click="confirmAction('stop', 'A kényszerített leállítás adatvesztést okozhat. Folytatod?', 'Kényszerített leállítás')"><icon name="stop" /> Kényszerített leállítás</btn>
            </div>
            <div class="ssh-line">
              <label>SSH kapcsolat<input :value="sshCommand()" readonly placeholder="Nincs kiosztott nyilvános IP"></label>
              <btn variant="icon" tooltip="SSH parancs másolása" :disabled="!sshCommand()" @click="copySSHCommand"><icon name="copy" /></btn>
            </div>
            <div class="terminal">
              <div class="terminal-heading"><h3>SSH terminál</h3><span>{{ sshStatus }}</span></div>
              <pre aria-live="polite">{{ sshOutput }}</pre>
              <textarea aria-label="SSH terminál beviteli mező" :disabled="sshStatus !== 'Kapcsolódva'" rows="2" placeholder="Írd be a parancsot…" @keydown="sendSSHKey" @paste="pasteSSH"></textarea>
              <div class="actions">
                <btn v-if="sshStatus !== 'Kapcsolódva' && sshStatus !== 'Kapcsolódás...'" color="primary" :disabled="saving || !instanceIP(selectedInstance)" @click="connectSSH"><icon name="play" /> Kapcsolódás</btn>
                <btn v-else :disabled="sshStatus === 'Kapcsolódás...'" @click="closeSSH"><icon name="stop" /> Leválasztás</btn>
              </div>
              <p class="hint">A kapcsolat a panel szerveréről indul. A host kulcs ellenőrzése kötelező.</p>
            </div>
          </section>
        </div>
      </section>

      <section v-if="selectedInstance" class="section">
        <h2>Rendszer telepítése</h2>
        <p class="warning">Az újratelepítés törli a VPS lemezén lévő adatokat. SSH-kulcs és jelszó helyett Contabo Secret ID-ket használj.</p>
        <div class="form-grid">
          <label>OS image<select v-model="reinstallImage" required><option value="" disabled>Válassz image-et</option><option v-for="image in images" :key="image.imageId" :value="image.imageId">{{ imageLabel(image) }}</option></select></label>
          <label>Alapértelmezett SSH-felhasználó<select v-model="reinstallUser"><option value="root">root</option><option value="admin">admin</option><option value="administrator">administrator (Windows)</option></select></label>
          <label>SSH secret ID-k, vesszővel elválasztva<input v-model="sshSecretIds" inputmode="numeric" placeholder="pl. 1234, 5678"></label>
          <label>Jelszó secret ID<input v-model="rootPasswordSecret" inputmode="numeric" placeholder="Opcionális"></label>
        </div>
        <label class="wide-label">Cloud-init user-data<textarea v-model="userData" rows="5" placeholder="#cloud-config"></textarea></label>
        <btn color="error" :disabled="saving || !reinstallImage" @click="confirmReinstall"><icon name="apply" /> Újratelepítés</btn>
      </section>

      <section class="section">
        <h2>Saját OS image hozzáadása</h2>
        <p class="hint">A Contabo közvetlen HTTPS URL-ről tölt le `.iso` vagy `.qcow2` image-et.</p>
        <form class="form-grid" @submit.prevent="addCustomImage">
          <label>Név<input v-model="customImage.name" required maxlength="255"></label>
          <label>Verzió<input v-model="customImage.version" required maxlength="80"></label>
          <label>Image URL<input v-model="customImage.url" type="url" required placeholder="https://.../image.iso"></label>
          <label>Operációs rendszer<select v-model="customImage.osType"><option>Linux</option><option>Windows</option></select></label>
          <label class="wide-label">Leírás<input v-model="customImage.description" maxlength="255"></label>
          <btn color="primary" type="submit" :disabled="saving"><icon name="plus" /> Image regisztrálása</btn>
        </form>
      </section>

      <section class="section">
        <div class="section-heading"><h2>Contabo tűzfalak</h2><btn variant="icon" tooltip="Fizetős firewall kiegészítés" :disabled="saving || !selectedInstance" @click="buyFirewallAddon"><icon name="plus" /></btn></div>
        <p class="warning">A Contabo tűzfal VPS-enként fizetős kiegészítő lehet; aktiválás előtt külön megerősítést kérünk.</p>
        <form class="form-grid" @submit.prevent="createFirewall">
          <label>Név<input v-model="firewallName" required maxlength="255"></label>
          <label>Állapot<select v-model="firewallStatus"><option value="inactive">Inaktív</option><option value="active">Aktív</option></select></label>
          <label class="wide-label">Leírás<input v-model="firewallDescription" maxlength="255"></label>
          <label class="wide-label">Szabályok JSON<textarea v-model="firewallRules" rows="5" spellcheck="false"></textarea></label>
          <btn color="primary" type="submit" :disabled="saving || !firewallName.trim()"><icon name="plus" /> Tűzfal létrehozása</btn>
        </form>
        <p v-if="!firewalls.length" class="empty">Nincs Contabo tűzfal.</p>
        <div v-for="firewall in firewalls" :key="firewall.firewallId" class="firewall-row">
          <div class="firewall-heading"><strong>{{ firewall.name }}</strong><span>{{ firewall.status }}</span></div>
          <label>Inbound szabályok JSON<textarea v-model="firewallAssignments[firewall.firewallId]" rows="5" spellcheck="false" :placeholder="JSON.stringify(firewall.rules || { inbound: [] }, null, 2)"></textarea></label>
          <div class="actions">
            <btn :disabled="saving || !selectedInstance" @click="assignFirewall(firewall)"><icon name="plus" /> Hozzárendelés a kiválasztott VPS-hez</btn>
            <btn :disabled="saving" @click="saveRules(firewall)"><icon name="save" /> Szabályok mentése</btn>
          </div>
        </div>
      </section>
    </template>
  </main>
</template>

<style scoped lang="scss">
.contabo-page { max-width: 1100px; }
.heading, .section-heading, .firewall-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.heading h1 { display: flex; align-items: center; gap: 10px; margin-bottom: 4px; }
.heading h1, h2 { margin-top: 0; }
.hint, .empty { color: var(--color-text-secondary); }
.error, .warning { color: var(--color-error); }
.notice { color: var(--color-success); }
.instances-section, .section { margin-top: 28px; padding-top: 20px; border-top: 1px solid var(--color-background); }
.section-heading input, input, select, textarea { min-width: 0; padding: 9px; border: 1px solid var(--color-background); border-radius: 5px; background: var(--color-background-secondary); color: var(--color-text); }
.section-heading input { flex: 1 1 220px; max-width: 360px; }
.instance-layout { display: grid; grid-template-columns: minmax(210px, 0.8fr) minmax(0, 1.6fr); gap: 20px; }
.instance-list { display: flex; flex-direction: column; gap: 4px; }
.instance-row { display: flex; flex-direction: column; align-items: flex-start; gap: 4px; width: 100%; padding: 11px; border: 1px solid transparent; border-radius: 5px; background: var(--color-background-secondary); color: var(--color-text); text-align: left; cursor: pointer; }
.instance-row span, dd { color: var(--color-text-secondary); overflow-wrap: anywhere; }
.instance-row.selected { border-color: var(--color-primary); }
.instance-detail { min-width: 0; }
dl { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; }
dl div { min-width: 0; }
dt { color: var(--color-text-secondary); font-size: 0.9rem; }
dd { margin: 3px 0 0; }
.actions { display: flex; flex-wrap: wrap; gap: 8px; margin: 12px 0; }
.ssh-line { display: flex; align-items: end; gap: 8px; }
.ssh-line label { flex: 1; }
.terminal { margin-top: 16px; }
.terminal-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.terminal-heading h3 { margin: 0; }
.terminal pre { min-height: 260px; max-height: 420px; overflow: auto; margin: 10px 0; padding: 12px; border-radius: 5px; background: #111; color: #f3f3f3; white-space: pre-wrap; overflow-wrap: anywhere; font: 13px/1.5 monospace; }
.terminal textarea { font: 13px/1.5 monospace; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin: 14px 0; }
label { display: flex; flex-direction: column; gap: 5px; color: var(--color-text-secondary); }
input, select, textarea { width: 100%; }
textarea { resize: vertical; font-family: monospace; }
.wide-label { grid-column: 1 / -1; }
.firewall-row { margin-top: 16px; padding-top: 14px; border-top: 1px solid var(--color-background); }
.firewall-row textarea { margin-top: 8px; }
@media (max-width: 720px) {
  .instance-layout, .form-grid { grid-template-columns: 1fr; }
  .wide-label { grid-column: auto; }
}
</style>