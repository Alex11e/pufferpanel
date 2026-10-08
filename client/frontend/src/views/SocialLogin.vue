<script setup>
import { inject, onMounted, ref } from 'vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'

const api = inject('api')
const events = inject('events')
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const providers = ref([])
const redirectUrl = ref('')
const providerForms = ref({})
const settings = ref({ allowRegistration: false, allowEmailLinking: false, allowAccountLinking: false, redirectBaseUrl: '' })
const draft = ref({ key: '', name: '', kind: 'google', clientId: '', clientSecret: '', issuerUrl: '', enabled: false })

async function load() {
  loading.value = true
  error.value = ''
  try {
    const response = await api.get('/api/admin/social-login')
    providers.value = response.data.providers || []
    redirectUrl.value = response.data.redirectUrl || ''
    settings.value = {
      allowRegistration: response.data.allowRegistration,
      allowEmailLinking: response.data.allowEmailLinking,
      allowAccountLinking: response.data.allowAccountLinking,
      redirectBaseUrl: response.data.redirectBaseUrl || ''
    }
    providerForms.value = Object.fromEntries(providers.value.map(provider => [provider.key, { ...provider, clientSecret: '' }]))
  } catch (failure) {
    error.value = failure?.msg || failure?.message || 'A külső bejelentkezési beállítások betöltése nem sikerült.'
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  saving.value = true
  error.value = ''
  try {
    await api.put('/api/admin/social-login/settings', settings.value)
  } catch (failure) {
    error.value = failure?.msg || failure?.message || 'A policy-beállítások mentése nem sikerült.'
  } finally {
    saving.value = false
  }
}

async function saveProvider(key, form) {
  saving.value = true
  error.value = ''
  try {
    await api.put(`/api/admin/social-login/providers/${encodeURIComponent(key)}`, form)
    await load()
  } catch (failure) {
    error.value = failure?.msg || failure?.message || 'A provider mentése nem sikerült.'
  } finally {
    saving.value = false
  }
}

function deleteProvider(provider) {
  events.emit('confirm', `Törlöd a(z) ${provider.name} bejelentkezést? A kapcsolt fiókok is leválnak.`, {
    text: 'Törlés', icon: 'remove', color: 'error', action: async () => {
      error.value = ''
      try {
        await api.delete(`/api/admin/social-login/providers/${encodeURIComponent(provider.key)}`)
        await load()
      } catch (failure) {
        error.value = failure?.msg || failure?.message || 'A provider törlése nem sikerült.'
      }
    }
  })
}

async function createProvider() {
  const key = draft.value.key.trim().toLowerCase()
  await saveProvider(key, { ...draft.value, key: undefined })
  if (!error.value) draft.value = { key: '', name: '', kind: 'google', clientId: '', clientSecret: '', issuerUrl: '', enabled: false }
}

onMounted(load)
</script>

<template>
  <div class="social-admin">
    <header><div><h1><icon name="account" /> Külső bejelentkezések</h1><p>Google, Discord, GitHub és OpenID Connect providerek kezelése.</p></div><btn :disabled="loading" @click="load"><icon :name="loading ? 'loading' : 'reload'" :spin="loading" /> Frissítés</btn></header>
    <loader v-if="loading" />
    <template v-else>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <section>
        <h2>Fiók policy-k</h2>
        <label><input v-model="settings.allowRegistration" type="checkbox"> Új fiókok létrehozása külső belépéssel</label>
        <label><input v-model="settings.allowEmailLinking" type="checkbox"> Meglévő fiók automatikus összekapcsolása ellenőrzött e-mail alapján</label>
        <label><input v-model="settings.allowAccountLinking" type="checkbox"> Felhasználók provider fiókot kapcsolhatnak vagy választhatnak le</label>
        <label class="callback-setting">OAuth callback base URL<input v-model="settings.redirectBaseUrl" type="url" placeholder="Üresen a panel Master URL-jét használja"></label>
        <p class="callback-preview">Provider callback URL: <code>{{ redirectUrl }}</code></p>
        <btn color="primary" :disabled="saving" @click="saveSettings"><icon name="save" /> Policy-k mentése</btn>
      </section>
      <section>
        <h2>Providerek</h2>
        <p>OAuth callback URL: <code>{{ redirectUrl }}</code></p>
        <p v-if="!providers.length" class="muted">Még nincs beállított provider.</p>
        <article v-for="provider in providers" :key="provider.key">
          <div class="provider-heading"><h3>{{ provider.name }}</h3><code>{{ provider.key }} · {{ provider.kind }}</code></div>
          <div class="fields">
            <label>Név<input v-model="providerForms[provider.key].name" maxlength="100"></label>
            <label>Client ID<input v-model="providerForms[provider.key].clientId" maxlength="255"></label>
            <label>Új client secret<input v-model="providerForms[provider.key].clientSecret" type="password" autocomplete="new-password" :placeholder="provider.hasSecret ? 'Beállítva; üresen hagyva változatlan' : 'Kötelező'"></label>
            <label v-if="provider.kind === 'oidc'">Issuer URL<input v-model="providerForms[provider.key].issuerUrl" type="url" placeholder="https://id.example.com"></label>
            <label class="toggle"><input v-model="providerForms[provider.key].enabled" type="checkbox"> Engedélyezve</label>
          </div>
          <div class="actions"><btn color="primary" :disabled="saving" @click="saveProvider(provider.key, providerForms[provider.key])"><icon name="save" /> Mentés</btn><btn color="error" :disabled="saving" @click="deleteProvider(provider)"><icon name="remove" /> Törlés</btn></div>
        </article>
      </section>
      <section>
        <h2>Provider hozzáadása</h2>
        <div class="fields">
          <label>Azonosító<input v-model="draft.key" maxlength="40" placeholder="pl. google"></label>
          <label>Név<input v-model="draft.name" maxlength="100" placeholder="Google"></label>
          <label>Típus<select v-model="draft.kind"><option value="google">Google</option><option value="discord">Discord</option><option value="github">GitHub</option><option value="oidc">OpenID Connect</option></select></label>
          <label>Client ID<input v-model="draft.clientId" maxlength="255"></label>
          <label>Client secret<input v-model="draft.clientSecret" type="password" autocomplete="new-password"></label>
          <label v-if="draft.kind === 'oidc'">Issuer URL<input v-model="draft.issuerUrl" type="url" placeholder="https://id.example.com"></label>
          <label class="toggle"><input v-model="draft.enabled" type="checkbox"> Engedélyezve mentés után</label>
        </div>
        <btn color="primary" :disabled="saving || !draft.key.trim() || !draft.name.trim() || !draft.clientId || !draft.clientSecret" @click="createProvider"><icon name="plus" /> Hozzáadás</btn>
      </section>
    </template>
  </div>
</template>

<style scoped lang="scss">
.social-admin header, .provider-heading, .actions { display:flex; align-items:center; justify-content:space-between; gap:12px; }
.social-admin header p, .muted, code { color:var(--color-text-secondary); }
section { margin-top:18px; padding:16px; background:var(--color-background-secondary); border-radius:6px; }
section h2 { margin-top:0; }
section > label { display:flex; align-items:center; gap:9px; margin:12px 0; }
.callback-setting { align-items:flex-start; flex-direction:column; }
.callback-setting input { width:min(100%, 560px); padding:9px; color:var(--color-text); background:var(--color-background); border:1px solid var(--color-background); border-radius:5px; }
.callback-preview { color:var(--color-text-secondary); overflow-wrap:anywhere; }
article { padding:14px 0; border-top:1px solid var(--color-background); }
.provider-heading h3 { margin:0; }
.fields { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:12px; margin:14px 0; }
.fields label { display:grid; gap:6px; }
.fields input:not([type=checkbox]), .fields select { min-width:0; padding:9px; color:var(--color-text); background:var(--color-background); border:1px solid var(--color-background); border-radius:5px; }
.fields .toggle { display:flex; align-items:center; gap:8px; }
.actions { justify-content:flex-start; }
.error { color:var(--color-error); }
@media(max-width:700px) { .social-admin header { align-items:flex-start; flex-direction:column; } .fields { grid-template-columns:1fr; } }
</style>