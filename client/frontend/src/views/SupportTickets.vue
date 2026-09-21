<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const tickets = ref([])
const active = ref(null)
const loading = ref(true)
const form = ref({ subject: '', serverId: '', category: 'technical', priority: 'normal', message: '' })
const reply = ref('')
const error = ref('')
const isAdmin = computed(() => api.auth.hasScope('admin'))
const summary = computed(() => ({ open: tickets.value.filter(ticket => ticket.status === 'open').length, answered: tickets.value.filter(ticket => ticket.status === 'answered').length, urgent: tickets.value.filter(ticket => ticket.priority === 'urgent').length }))

async function load() { loading.value = true; try { const response = await api.get('/api/tickets'); tickets.value = response.data || [] } finally { loading.value = false } }
async function open(ticket) { const response = await api.get(`/api/tickets/${ticket.id}`); active.value = response.data; reply.value = '' }
async function create() { error.value = ''; try { const response = await api.post('/api/tickets', form.value); form.value = { subject: '', serverId: '', category: 'technical', priority: 'normal', message: '' }; await load(); active.value = response.data } catch { error.value = 'A jegy létrehozása nem sikerült.' } }
async function sendReply() { if (!reply.value.trim() || !active.value) return; try { await api.post(`/api/tickets/${active.value.id}/messages`, { body: reply.value }); await open(active.value); await load() } catch { error.value = 'Az üzenet elküldése nem sikerült.' } }
async function closeTicket() { if (!active.value) return; await api.put(`/api/tickets/${active.value.id}/status`, { status: 'closed' }); await open(active.value); await load() }
async function updatePriority() { if (!active.value || !isAdmin.value) return; await api.put(`/api/tickets/${active.value.id}/status`, { status: active.value.status, priority: active.value.priority }); await load() }
onMounted(load)
</script>

<template>
  <div class="support">
    <h1><icon name="help" /> Támogatás</h1>
    <p class="hint">Írj részletes hibajegyet. A jelszavadat és más titkos adatot ne küldd el üzenetben.</p>
    <loader v-if="loading" />
    <template v-else>
      <section class="new-ticket"><h2>Új hibajegy</h2><input v-model="form.subject" maxlength="140" placeholder="Rövid tárgy"><input v-model="form.serverId" maxlength="20" placeholder="Szerver azonosítója (opcionális)"><select v-model="form.category"><option value="technical">Technikai hiba</option><option value="billing">Számlázás</option><option value="account">Fiók</option><option value="other">Egyéb</option></select><select v-model="form.priority"><option value="low">Alacsony</option><option value="normal">Normál</option><option value="high">Magas</option><option value="urgent">Sürgős</option></select><textarea v-model="form.message" maxlength="4000" placeholder="Írd le a problémát, az időpontot és a hibaüzenetet." /><btn color="primary" :disabled="!form.subject.trim() || !form.message.trim()" @click="create">Jegy megnyitása</btn></section>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="ticket-summary"><span><b>{{ summary.open }}</b> nyitott</span><span><b>{{ summary.answered }}</b> megválaszolt</span><span><b>{{ summary.urgent }}</b> sürgős</span></div>
      <div class="layout"><section><h2>{{ isAdmin ? 'Összes hibajegy' : 'Saját hibajegyek' }}</h2><div v-if="!tickets.length" class="hint">Még nincs hibajegy.</div><button v-for="ticket in tickets" :key="ticket.id" :class="['ticket', { selected: active?.id === ticket.id, urgent: ticket.priority === 'urgent' }]" @click="open(ticket)"><strong>#{{ ticket.id }} · {{ ticket.subject }}</strong><small>{{ ticket.status }} · {{ ticket.category }} · {{ ticket.priority }}<template v-if="isAdmin"> · {{ ticket.username }}</template></small></button></section><section v-if="active" class="conversation"><h2>#{{ active.id }} · {{ active.subject }}</h2><p class="hint">Állapot: <b>{{ active.status }}</b><template v-if="active.serverId"> · Szerver: {{ active.serverId }}</template></p><label v-if="isAdmin" class="priority">Prioritás <select v-model="active.priority" @change="updatePriority"><option value="low">Alacsony</option><option value="normal">Normál</option><option value="high">Magas</option><option value="urgent">Sürgős</option></select></label><article v-for="message in active.messages" :key="message.id"><strong>{{ message.username }}</strong><small>{{ new Date(message.createdAt).toLocaleString() }}</small><p>{{ message.body }}</p></article><div v-if="active.status !== 'closed'" class="reply"><textarea v-model="reply" maxlength="4000" placeholder="Válasz írása" /><btn color="primary" :disabled="!reply.trim()" @click="sendReply">Válasz küldése</btn><btn color="error" @click="closeTicket">Jegy lezárása</btn></div></section></div>
    </template>
  </div>
</template>

<style scoped lang="scss">
.support h1 { display:flex; gap:10px; align-items:center; } .hint, small { color:var(--color-text-secondary); } section, .error { margin-top:16px; padding:16px; border-radius:8px; background:var(--color-background-secondary); } .new-ticket { display:grid; grid-template-columns:1fr 1fr auto; gap:9px; } input, textarea, select { padding:9px; color:var(--color-text); background:var(--color-background); border:1px solid var(--color-background); border-radius:5px; } textarea { grid-column:1 / -1; min-height:85px; resize:vertical; } .ticket-summary { display:flex; gap:10px; flex-wrap:wrap; margin-top:16px; } .ticket-summary span { padding:8px 12px; border-radius:99px; background:var(--color-background-secondary); } .layout { display:grid; grid-template-columns:minmax(250px, .7fr) minmax(0, 1.3fr); gap:16px; } .ticket { display:block; width:100%; padding:11px; text-align:left; color:var(--color-text); background:var(--color-background); border:0; border-radius:5px; margin-top:7px; cursor:pointer; } .ticket.selected { outline:2px solid var(--color-primary); } .ticket.urgent { border-left:3px solid var(--color-error); } .ticket strong, .ticket small { display:block; } article { padding:10px 0; border-bottom:1px solid var(--color-background); } article small { margin-left:8px; } article p { white-space:pre-wrap; margin:6px 0 0; } .priority { display:flex; align-items:center; gap:8px; margin:10px 0; } .reply { display:grid; grid-template-columns:1fr auto auto; gap:8px; margin-top:12px; } .reply textarea { grid-column:1 / -1; } .error { color:var(--color-error); } @media (max-width:700px) { .layout { grid-template-columns:1fr; } .new-ticket { grid-template-columns:1fr; } .reply { grid-template-columns:1fr; } }
</style>
