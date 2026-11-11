<script setup>
import { ref, inject, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

const getHotkeys = inject('hotkeys')
const settings = inject('hotkeySettings')
const { t } = useI18n()
const route = useRoute()
const ctxOverrides = {
  'TemplateView': 'Template',
  'TemplateCreate': 'Template'
}
const hotkeys = ref(currentHotkeys())
const editing = ref(null)
const value = ref('')
const error = ref('')

function edit(group, key) {
  editing.value = { group, key }
  value.value = settings.effective(group, key)
  error.value = ''
}

function save() {
  try {
    settings.save(editing.value.group, editing.value.key, value.value.trim())
    editing.value = null
    error.value = ''
  } catch (e) {
    error.value = t(e.message === 'conflict' ? 'hotkeys.Conflict' : e.message === 'invalid' ? 'hotkeys.Invalid' : 'hotkeys.SaveError')
  }
}

function reset(group, key) {
  try {
    settings.reset(group, key)
    editing.value = null
    error.value = ''
  } catch (e) {
    error.value = t(e.message === 'conflict' ? 'hotkeys.Conflict' : 'hotkeys.SaveError')
  }
}

function getContext() {
  return ctxOverrides[route.name] || route.name
}

function currentHotkeys() {
  const h = getHotkeys()
  const res = {
    global: (h.root || []).flat().filter(e => e !== '?' && e !== 'Shift+?' && e !== 'Escape'),
    contextual: (h[route.name] || [])
      .flat()
      .filter(e => e !== 'Escape')
      .filter(e => e !== 'Control+a')
      .filter(e => !/^. \d \d$/.test(e))
      .sort(),
    context: getContext()
  }

  return res
}

watch(
  () => route.name,
  async () => {
    setTimeout(() => {
      hotkeys.value = currentHotkeys()
    }, 500)
  }
)

onMounted(() => {
  hotkeys.value = currentHotkeys()
})
</script>

<template>
  <p>{{ t('hotkeys.CustomHelp') }}</p>
  <p v-if="error" role="alert">{{ error }}</p>
  <div class="hotkey-list">
    <div v-if="hotkeys.contextual.length > 0" class="contextual">
      <h3 v-text="t(`hotkeys.${hotkeys.context}.Title`)" />
      <div v-for="keys in hotkeys.contextual" :key="keys" class="hotkey">
        <span class="keys">
          <span v-if="!settings.effective(route.name, keys)" class="key">{{ t('hotkeys.Disabled') }}</span>
          <span v-for="key in settings.effective(route.name, keys).split(' ').filter(Boolean)" :key="key" class="key" v-text="key" />
        </span>
        <span class="description" v-text="t(`hotkeys.${hotkeys.context}.${keys}`)" />
        <button type="button" @click="edit(route.name, keys)">{{ t('hotkeys.Edit') }}</button>
        <button type="button" @click="reset(route.name, keys)">{{ t('hotkeys.Reset') }}</button>
      </div>
    </div>
    <div class="global">
      <h3 v-text="t('hotkeys.Global.Title')" />
      <div v-for="keys in hotkeys.global" :key="keys" class="hotkey">
        <span class="keys">
          <span v-if="!settings.effective('root', keys)" class="key">{{ t('hotkeys.Disabled') }}</span>
          <span v-for="key in settings.effective('root', keys).split(' ').filter(Boolean)" :key="key" class="key" v-text="key" />
        </span>
        <span class="description" v-text="t(`hotkeys.Global.${keys}`)" />
        <button type="button" @click="edit('root', keys)">{{ t('hotkeys.Edit') }}</button>
        <button type="button" @click="reset('root', keys)">{{ t('hotkeys.Reset') }}</button>
      </div>
    </div>
  </div>
  <form v-if="editing" class="hotkey-editor" @submit.prevent="save">
    <label for="hotkey-value">{{ t('hotkeys.NewShortcut') }} ({{ editing.key }})</label>
    <input id="hotkey-value" v-model="value" autocomplete="off">
    <button type="submit">{{ t('hotkeys.Save') }}</button>
    <button type="button" @click="editing = null">{{ t('hotkeys.Cancel') }}</button>
  </form>
</template>

<style scoped>
.hotkey { flex-wrap: wrap; gap: .4rem; align-items: center; }
.hotkey .description { height: auto; flex: 1 1 10rem; }
.hotkey-editor { display: flex; flex-wrap: wrap; gap: .5rem; margin-top: 1rem; }
</style>
