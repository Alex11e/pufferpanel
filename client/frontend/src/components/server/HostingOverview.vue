<script setup>
import { ref, computed, onMounted } from 'vue'
import PowerControls from './PowerControls.vue'

const props = defineProps({
  server: { type: Object, required: true },
  mode: { type: String, default: 'web' }
})

const vars = ref(null)
const reveal = ref(false)

const host = computed(() => (isPrivate.value ? '127.0.0.1' : props.server.node?.publicHost || ''))
const isPrivate = computed(() => value('ip') === '127.0.0.1')
const port = computed(() => value('port', ''))

function value(key, fallback = '') {
  const entry = vars.value?.[key]
  return entry && entry.value !== undefined && entry.value !== '' ? entry.value : fallback
}

const engine = computed(() => value('engine', ''))
const connection = computed(() => {
  const user = encodeURIComponent(value('db_user'))
  const secret = reveal.value ? encodeURIComponent(value('db_password')) : '********'
  const db = encodeURIComponent(value('db_name'))
  const scheme = engine.value === 'postgres' ? 'postgresql' : 'mysql'
  return `${scheme}://${user}:${secret}@${host.value}:${port.value}/${db}`
})

onMounted(async () => {
  if (props.server.hasScope('server.data.view')) {
    const data = await props.server.getData()
    vars.value = data.data || data
  }
})
</script>

<template>
  <div class="hosting-overview">
    <h2>{{ mode === 'web' ? 'Webtárhely' : 'Adatbázis' }}</h2>
    <power-controls :server="server" />

    <section v-if="vars && mode === 'web'" class="card">
      <h3>Weboldal</h3>
      <p>Cím: <a :href="`http://${host}:${port}`" target="_blank" rel="noopener noreferrer">http://{{ host }}:{{ port }}</a></p>
      <p>PHP verzió: {{ value('php_version', '-') }}</p>
      <p class="hint">A weboldal fájljait a Fájlok fülön vagy SFTP-n töltheted fel, a gyökérmappa a nyilvános könyvtár. Ha a PHP nem tud fájlt írni, állíts a mappán írási jogot.</p>
    </section>

    <section v-if="vars && mode === 'db' && engine === 'phpmyadmin'" class="card">
      <h3>phpMyAdmin</h3>
      <p>Webfelület: <a :href="`http://${host}:${port}`" target="_blank" rel="noopener noreferrer">http://{{ host }}:{{ port }}</a></p>
      <p>Cél adatbázis-szerver: <code>{{ value('pma_host') }}:{{ value('pma_port') }}</code></p>
      <p class="hint">Bejelentkezéshez az adatbázis felhasználóneve és jelszava kell. A célszerver a Beállítások fülön módosítható újraindítás után.</p>
    </section>

    <section v-if="vars && mode === 'db' && engine !== 'phpmyadmin'" class="card">
      <h3>Kapcsolat</h3>
      <ul>
        <li>Motor: {{ engine === 'postgres' ? 'PostgreSQL' : 'MariaDB' }} {{ value('version') }}</li>
        <li>Gép: <code>{{ host }}</code> · Port: <code>{{ port }}</code></li>
        <li>Adatbázis: <code>{{ value('db_name') }}</code> · Felhasználó: <code>{{ value('db_user') }}</code></li>
      </ul>
      <p><code class="uri">{{ connection }}</code></p>
      <p v-if="isPrivate" class="hint">Privát adatbázis: csak a node-on futó programok érik el.</p>
      <button type="button" class="link" @click="reveal = !reveal">{{ reveal ? 'Jelszó elrejtése' : 'Jelszó megjelenítése' }}</button>
      <p class="hint">A mentések fájlszintűek: futó adatbázisnál nem garantáltan konzisztensek, ezért érdemes előtte leállítani, vagy SQL-dumpot használni.</p>
    </section>
  </div>
</template>

<style scoped lang="scss">
.card { margin-top: 16px; padding: 16px; border-radius: 8px; background: var(--color-background-secondary); border: 1px solid var(--color-background); }
.hint { color: var(--color-text-secondary); }
.uri { word-break: break-all; }
ul { padding-left: 1.2em; }
.link { background: none; border: 0; padding: 0; color: var(--color-primary); cursor: pointer; }
</style>
