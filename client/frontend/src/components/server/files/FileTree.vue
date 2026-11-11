<script setup>
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/ui/Icon.vue'
import { sortFiles } from './fileView.mjs'

const props = defineProps({
  server: { type: Object, required: true },
  path: { type: String, default: '' },
  label: { type: String, default: '' },
  generation: { type: Number, default: 0 },
  root: { type: Boolean, default: false }
})
const emit = defineEmits(['open'])
const { t } = useI18n()
const expanded = ref(props.root)
const entries = ref(null)
const error = ref('')
let requestId = 0

async function load() {
  const request = ++requestId
  error.value = ''
  try {
    const result = await props.server.getFile(props.path)
    if (request !== requestId) return
    entries.value = result.filter(entry => entry.name !== '..').sort(sortFiles)
  } catch {
    if (request === requestId) {
      entries.value = null
      error.value = t('files.LoadFailed')
    }
  }
}

function toggle() {
  expanded.value = !expanded.value
  if (expanded.value) load()
}

function childPath(name) {
  return props.path ? `${props.path}/${name}` : name
}

watch(() => props.generation, () => {
  if (expanded.value) load()
}, { immediate: true })
</script>

<template>
  <div class="tree-branch">
    <button v-if="!root" type="button" class="tree-entry" :aria-expanded="expanded" @click="toggle">
      <span class="chevron">{{ expanded ? 'v' : '>' }}</span><icon name="folder" /> <span>{{ label }}</span>
    </button>
    <div v-if="expanded" :class="{ children: !root }">
      <p v-if="error" class="tree-error" role="alert">
        {{ error }} <button type="button" @click="load">{{ t('files.Refresh') }}</button>
      </p>
      <p v-else-if="entries === null" class="tree-hint">{{ t('files.Loading') }}</p>
      <p v-else-if="entries.length === 0" class="tree-hint">{{ t('files.EmptyFolder') }}</p>
      <template v-for="entry in entries" :key="entry.name">
        <button v-if="entry.isFile" type="button" class="tree-entry" @click="emit('open', entry, childPath(entry.name))">
          <span class="chevron" /><icon name="file" /> <span>{{ entry.name }}</span>
        </button>
        <FileTree v-else :server="server" :path="childPath(entry.name)" :label="entry.name" :generation="generation" @open="(file, path) => emit('open', file, path)" />
      </template>
    </div>
  </div>
</template>

<style scoped>
.children { padding-inline-start: 1rem; }
.tree-entry { display: flex; align-items: center; gap: .35rem; width: 100%; padding: .3rem .4rem; border: 0; border-radius: .3rem; background: transparent; color: inherit; text-align: start; cursor: pointer; overflow-wrap: anywhere; }
.tree-entry:hover, .tree-entry:focus-visible { background: var(--color-background-secondary); outline: 1px solid var(--color-primary); }
.chevron { width: .8rem; flex-shrink: 0; }
.tree-hint { margin: .4rem; color: var(--color-text-secondary); }
.tree-error { margin: .4rem; color: var(--color-error); }
</style>
