<script setup>
import { computed, ref, onMounted, onUnmounted, inject, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import ContextMenu from '@/components/ui/ContextMenu.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import Overlay from '@/components/ui/Overlay.vue'
import Editor, { skipDownload } from './files/Editor.vue'
import FileTree from './files/FileTree.vue'
import { deleteFiles, retainSelection, selectedPaths, sortFiles, validItemName, visibleFiles as filterVisibleFiles } from './files/fileView.mjs'
import Upload from './files/Upload.vue'
import TextField from '@/components/ui/TextField.vue'
import Toggle from '@/components/ui/Toggle.vue'

const { t } = useI18n()
const events = inject('events')
const toast = inject('toast')

const props = defineProps({
  server: { type: Object, required: true }
})

const allowDirectoryUpload = 'webkitdirectory' in document.createElement('input')
const canEdit = props.server.hasScope('server.files.edit')

const fileEls = ref([])
const files = ref(null)
const file = ref(null)
const fileSizeWarn = ref(false)
const fileSizeWarnSubject = ref(null)
const currentPath = ref([])
const editorOpen = ref(false)
const loading = ref(false)
const saving = ref(false)
const createFileOpen = ref(false)
const createFolderOpen = ref(false)
const archiveSelectedOpen = ref(false)
const newItemName = ref('')
const createError = ref('')
const creating = ref(false)
const viewMode = ref('list')
const treeGeneration = ref(0)
const fileError = ref('')
const search = ref('')
let fileRequest = 0
let openRequest = 0
const selection = computed(() => {
  return (files.value || []).filter(f => f.isSelected)
})
const visibleFiles = computed(() => filterVisibleFiles(files.value || [], search.value))

let task
let unbindEvent
onMounted(() => {
  unbindEvent = props.server.on('status', () => {
    if (selection.value.length === 0) refresh()
  })

  task = props.server.startTask(() => {
    if (selection.value.length === 0) refresh()
  }, 5 * 60 * 1000)
})

onUnmounted(async () => {
  fileRequest++
  openRequest++
  if (unbindEvent) unbindEvent()
  if (task) props.server.stopTask(task)
})

watch(currentPath, () => {
  files.value = null
  refresh()
}, { deep: true, flush: 'sync' })

async function refresh(manual = false) {
  const request = ++fileRequest
  const path = getCurrentPath()
  fileError.value = ''
  const previous = files.value || []
  if (manual) files.value = null // cause visual feedback on manual refresh
  try {
    const res = await props.server.getFile(path)
    if (request === fileRequest) files.value = retainSelection(res, files.value || previous).sort(sortFiles)
  } catch {
    if (request === fileRequest) {
      files.value = null
      fileError.value = t('files.LoadFailed')
    }
  }
  if (request === fileRequest) treeGeneration.value++
}

function getCurrentPath() {
  return currentPath.value.map(e => e.name).join('/')
}

async function openFile(f, overrideWarn = false, explicitPath = null) {
  if (f.isFile) {
    const path = explicitPath || getCurrentPath() + `/${f.name}`
    const request = ++openRequest
    loading.value = false
    if (!skipDownload(f) && !overrideWarn && f.size > 30 * Math.pow(2, 20)) {
      fileSizeWarnSubject.value = { file: f, path }
      fileSizeWarn.value = true
      return
    }

    fileSizeWarn.value = false
    loading.value = true
    try {
      const content = skipDownload(f) ? null : await props.server.getFile(path, true)
      if (request !== openRequest) return
      file.value = { ...f, path, content, originalContent: content, url: props.server.getFileUrl(path) }
      editorOpen.value = viewMode.value === 'list'
    } catch (error) {
      if (request === openRequest) {
        console.error('Could not open file', error)
        toast.error(t('files.OpenFailed'))
      }
    } finally {
      if (request === openRequest) loading.value = false
    }
  } else {
    if (f.name === '..') {
      currentPath.value.pop()
    } else {
      currentPath.value.push(f)
    }
  }
}

async function saveFile({close}) {
  if (!canEdit || saving.value || !file.value) return
  const savedFile = file.value
  const savedContent = savedFile.content
  const request = openRequest
  saving.value = true
  try {
    await props.server.uploadFile(savedFile.path, savedContent)
    toast.success(t('files.Saved'))
    if (request !== openRequest || !file.value || file.value.path !== savedFile.path) return
    file.value.originalContent = savedContent
    if (close && file.value.content === savedContent) {
      editorOpen.value = false
      file.value = null
    }
    await refresh()
  } catch (error) {
    console.error('Could not save file', error)
    toast.error(t('files.SaveFailed'))
  } finally {
    saving.value = false
  }
}

function openTreeFile(entry, path) {
  if (!confirmDiscard()) return
  currentPath.value = path.split('/').slice(0, -1).map(name => ({ name }))
  openFile(entry, false, path)
}

function confirmDiscard() {
  return !file.value || file.value.content === file.value.originalContent || window.confirm(t('files.DiscardChanges'))
}

function closeTreeFile() {
  if (!confirmDiscard()) return
  openRequest++
  file.value = null
  editorOpen.value = false
}

function changeView(mode) {
  if (viewMode.value === mode || !confirmDiscard()) return
  openRequest++
  loading.value = false
  if (files.value) deselectAll()
  viewMode.value = mode
  file.value = null
  editorOpen.value = false
}

function getIcon(file) {
  if (!file.isFile) return 'folder'
  if (!file.extension) return 'file'
  return 'file-' + file.extension.substring(1)
}

function deleteFile(file) {
  const path = `${getCurrentPath()}/${file.name}`
  events.emit(
    'confirm',
    t('files.ConfirmDelete', { name: file.name }),
    {
      text: t('files.Delete'),
      icon: 'remove',
      color: 'error',
      action: async () => {
        await props.server.deleteFile(path)
        await refresh()
      }
    },
    {
      color: 'primary'
    }
  )
}

const numFormat = new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 })
function formatFileSize(size) {
  if (!size) return '0 B'
  if (size < Math.pow(2, 10)) return numFormat.format(size) + ' B'
  if (size < Math.pow(2, 20)) return numFormat.format(size / Math.pow(2, 10)) + ' KiB'
  if (size < Math.pow(2, 30)) return numFormat.format(size / Math.pow(2, 20)) + ' MiB'
  if (size < Math.pow(2, 40)) return numFormat.format(size / Math.pow(2, 30)) + ' GiB'
  return numFormat.format(size / Math.pow(2, 40)) + ' TiB'
}

function startCreateFile() {
  newItemName.value = ''
  createError.value = ''
  createFileOpen.value = true
}

function startCreateFolder() {
  newItemName.value = ''
  createError.value = ''
  createFolderOpen.value = true
}

async function createFile() {
  if (!validItemName(newItemName.value) || creating.value) return
  if (!confirmDiscard()) return
  const name = newItemName.value
  const path = `${getCurrentPath()}/${name}`
  creating.value = true
  createError.value = ''
  try {
    await props.server.uploadFile(path, '', undefined, true)
    createFileOpen.value = false
    newItemName.value = ''
    await openFile({ name, size: 0, isFile: true }, false, path)
    await refresh()
  } catch (error) {
    console.error('Could not create file', error)
    createError.value = t(error.code === 'FILE_EXISTS' ? 'files.FileExists' : error.code === 'CREATE_UNAVAILABLE' ? 'files.CreateUnavailable' : 'files.CreateFailed')
  } finally {
    creating.value = false
  }
}

async function createFolder() {
  if (!validItemName(newItemName.value) || creating.value) return
  await props.server.createFolder(`${getCurrentPath()}/${newItemName.value}`)
  const folder = { name: newItemName.value, isFile: false }
  createFolderOpen.value = false
  newItemName.value = ''
  await openFile(folder)
  await refresh()
}

const archiveExtensions = [
  '.7z',
  '.bz2',
  '.gz',
  '.lz',
  '.lzma',
  '.rar',
  '.tar',
  '.tgz',
  '.xz',
  '.zip',
  '.zipx'
]

function isArchive (file) {
  const filename = file.name.toLowerCase()
  for (let i = 0; i < archiveExtensions.length; i++) {
    if (filename.endsWith(archiveExtensions[i])) return true
  }
  return false
}

async function extract(file) {
  loading.value = true
  try {
    let dest = getCurrentPath()
    if (!dest.startsWith('/')) dest = '/' + dest
    await props.server.extractFile(`${getCurrentPath()}/${file.name}`, dest)
    refresh()
  } finally {
    loading.value = false
  }
}

async function makeArchiveName(fileName, directory) {
  let destination = `${directory}/${fileName}.zip`
  for (let i = 2; await props.server.fileExists(destination); i++) {
    destination = `${directory}/${fileName} (${i}).zip`
  }
  return destination
}

async function archiveCurrentDirectory() {
  loading.value = true
  try {
    const directory = getCurrentPath()
    const item = currentPath.value[currentPath.value.length - 1];
    let lastPathEntry = props.server.id
    if (item !== undefined) {
      lastPathEntry = currentPath.value[currentPath.value.length - 1].name
    }

    await props.server.archiveFile(
      await makeArchiveName(lastPathEntry, directory),
      directory
    )
  } finally {
    setTimeout(() => {
      refresh()
      loading.value = false
    }, 500)
  }
}

async function archive(file) {
  loading.value = true
  try {
    const directory = getCurrentPath()
    await props.server.archiveFile(
      await makeArchiveName(file.name, directory),
      `${directory}/${file.name}`
    )
  } finally {
    setTimeout(() => {
      refresh()
      loading.value = false
    }, 500)
  }
}

function download(file) {
  const a = document.createElement('a')
  a.href = props.server.getFileUrl(getCurrentPath() + '/' + file.name)
  a.download = a.href.split('/').pop()
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

function fileListHotkey() {
  if (fileEls.value[0]) fileEls.value[0].focus()
}

function trackFileEl(index) {
  return (el) => fileEls.value[index] = el
}

function contextActionsForFile(file) {
  const actions = []
  if (file.name !== '..') {
    actions.push({
      icon: file.isSelected ? 'deselect' :'select',
      label: t(file.isSelected ? 'files.Deselect' : 'files.Select'),
      hotkey: 's',
      action: () => file.isSelected = !file.isSelected
    })
  }
  if (canEdit && file.name !== '..' && !file.isFile) {
    actions.push({
      icon: 'archive',
      label: t('files.Archive'),
      hotkey: 'a',
      action: () => archive(file)
    })
  }
  if (canEdit && file.isFile && isArchive(file)) {
    actions.push({
      icon: 'extract',
      label: t('files.Extract'),
      hotkey: 'e',
      action: () => extract(file)
    })
  }
  if (file.isFile) {
    actions.push({
      icon: 'download',
      label: t('files.Download'),
      hotkey: 'd',
      action: () => download(file)
    })
  }
  if (canEdit && file.name !== '..') {
    actions.push({
      icon: 'remove',
      label: t('files.Delete'),
      hotkey: 'Delete',
      class: 'action-delete',
      action: () => deleteFile(file)
    })
  }
  return actions
}

async function archiveSelected() {
  const archiveName = newItemName.value
  const directory = getCurrentPath()
  const paths = selectedPaths(directory, selection.value)
  if (!paths.length) {
    archiveSelectedOpen.value = false
    toast.error(t('files.NoSelection'))
    return
  }
  newItemName.value = ''
  archiveSelectedOpen.value = false
  loading.value = true
  try {
    await props.server.archiveFile(
      await makeArchiveName(archiveName, directory),
      paths
    )
  } finally {
    setTimeout(async () => {
      await refresh()
      loading.value = false
    }, 500)
  }
}

function deleteSelected() {
  const paths = selectedPaths(getCurrentPath(), selection.value)
  if (!paths.length) return
  events.emit(
    'confirm',
    t('files.ConfirmDeleteSelected', undefined, paths.length),
    {
      text: t('files.Delete'),
      icon: 'remove',
      color: 'error',
      action: async () => {
        loading.value = true
        try {
          const failures = await deleteFiles(props.server, paths)
          if (failures.length) {
            console.error('Some files could not be deleted', failures)
            toast.error(t('files.DeletePartialFailure', { count: failures.length }))
          }
        } finally {
          await refresh()
          loading.value = false
        }
      }
    },
    {
      color: 'primary'
    }
  )
}

function deselectAll() {
  files.value = files.value.map(f => {
    f.isSelected = false
    return f
  })
}

function selectAll() {
  if (files.value === null) return
  const visible = new Set(visibleFiles.value)
  files.value = files.value.map(f => {
    if (visible.has(f)) f.isSelected = f.name !== '..'
    return f
  })
}
</script>

<template>
  <div class="file-manager">
    <div class="header">
      <h2 v-text="t('servers.Files')" />
      <div class="file-view-controls" role="group" :aria-label="t('files.ViewMode')">
        <btn :color="viewMode === 'list' ? 'primary' : undefined" :aria-pressed="viewMode === 'list'" @click="changeView('list')">{{ t('files.ListView') }}</btn>
        <btn :color="viewMode === 'tree' ? 'primary' : undefined" :aria-pressed="viewMode === 'tree'" @click="changeView('tree')">{{ t('files.TreeView') }}</btn>
      </div>
      <h3>
        <a @click="currentPath = []"><icon name="server-root" /></a>
        <span v-for="segment, index in currentPath" :key="index">
          <icon name="path-separator" />
          <a @click="currentPath.splice(index + 1)" v-text="segment.name" />
        </span>
      </h3>
      <span class="spacer" />
      <span v-if="selection.length === 0" class="controls">
        <btn v-if="canEdit" v-hotkey="'f a'" variant="icon" :tooltip="t('files.ArchiveCurrent')" @click="archiveCurrentDirectory()"><icon name="archive" /></btn>
        <upload v-if="canEdit" :path="getCurrentPath()" :server="server" hotkey="f u" @uploaded="refresh()" />
        <upload v-if="canEdit && allowDirectoryUpload" :path="getCurrentPath()" :server="server" folder hotkey="f d" @uploaded="refresh()" />
        <btn v-if="canEdit" v-hotkey="'f c f'" variant="icon" :tooltip="t('files.CreateFile')" @click="startCreateFile()"><icon name="file-create" /></btn>
        <btn v-if="canEdit" v-hotkey="'f c d'" variant="icon" :tooltip="t('files.CreateFolder')" @click="startCreateFolder()"><icon name="folder-create" /></btn>
        <btn v-hotkey="'f r'" variant="icon" :tooltip="t('files.Refresh')" @click="refresh(true)"><icon name="reload" /></btn>
      </span>
      <span v-else class="controls">
        <span class="selection-count" v-text="t('files.Selected', undefined, selection.length)" />
        <btn v-if="canEdit" v-hotkey="'f s a'" variant="icon" :tooltip="t('files.ArchiveSelected')" @click="archiveSelectedOpen = true"><icon name="archive" /></btn>
        <btn v-if="canEdit" v-hotkey="'f s d'" variant="icon" :tooltip="t('files.DeleteSelected', undefined, selection.length)" @click="deleteSelected()"><icon name="remove" /></btn>
        <btn v-hotkey="'Escape'" variant="icon" :tooltip="t('files.DeselectAll')" @click="deselectAll()"><icon name="close" /></btn>
      </span>
    </div>
    <div v-if="fileError" class="alert error" role="alert">
      {{ fileError }} <btn @click="refresh(true)">{{ t('files.Refresh') }}</btn>
    </div>
    <div v-if="viewMode === 'tree'" class="tree-layout">
      <aside class="tree-sidebar" :aria-label="t('files.TreeView')">
        <FileTree :server="server" :generation="treeGeneration" root @open="openTreeFile" />
      </aside>
      <section class="tree-editor">
        <editor v-if="file" :key="file.path" v-model="file" :read-only="!canEdit" :saving="saving" @save="saveFile($event)" @close="closeTreeFile" />
        <p v-else class="tree-placeholder">{{ t('files.SelectFromTree') }}</p>
      </section>
    </div>
    <div v-else v-hotkey="['f l', 'Control+a']" class="file-list" @hotkey="keys => keys === 'Control+a' ? selectAll() : fileListHotkey()">
      <label class="file-search">
        <span>{{ t('files.Search') }}</span>
        <input v-model="search" type="search" :placeholder="t('files.SearchPlaceholder')">
      </label>
      <loader v-if="!Array.isArray(files) && !fileError" />
      <!-- eslint-disable-next-line vue/no-template-shadow -->
      <a v-for="(file, index) in visibleFiles" v-else :key="file.name" :ref="trackFileEl(index)" tabindex="0" :class="['file', file.isSelected ? 'selected' : '']" @click="openFile(file)" @keydown.enter="openFile(file)" @keydown.space="file.isSelected = !file.isSelected">
        <icon class="file-icon" :name="getIcon(file)" />
        <div class="details">
          <div class="name">{{ file.name }}</div>
          <div v-if="file.isFile" class="size">{{ formatFileSize(file.size) }}</div>
        </div>
        <toggle v-if="file.name !== '..'" v-model="file.isSelected" class="file-select" @click.stop="" />
        <context-menu :title="file.name" :actions="contextActionsForFile(file)">
          <template #title>
            <li class="context-title">
              <span class="name" v-text="file.name" />
              <span v-if="file.isFile" class="size" v-text="formatFileSize(file.size)" />
            </li>
          </template>

          <template #activator="contextMenu">
            <btn v-if="contextMenu.canOpen" tabindex="-1" variant="icon" @click.stop="contextMenu.onClick">
              <icon name="menu" />
            </btn>
          </template>
        </context-menu>
      </a>
    </div>
    <overlay v-model="fileSizeWarn" closable :title="t('files.OpenLargeFile')">
      <btn color="error" @click="fileSizeWarn = false"><icon name="close" />{{ t('common.Cancel') }}</btn>
      <btn color="primary" @click="openFile(fileSizeWarnSubject.file, true, fileSizeWarnSubject.path)"><icon name="check" />{{ t('files.OpenAnyways') }}</btn>
    </overlay>
    <overlay v-model="createFileOpen" closable :title="t('files.CreateFile')">
      <text-field v-model="newItemName" :error="createError || (newItemName && !validItemName(newItemName) ? t('files.InvalidName') : '')" />
      <btn color="primary" :disabled="!validItemName(newItemName) || creating" @click="createFile()"><icon name="check" />{{ t('files.CreateFile') }}</btn>
    </overlay>
    <overlay v-model="createFolderOpen" closable :title="t('files.CreateFolder')">
      <text-field v-model="newItemName" :error="newItemName && !validItemName(newItemName) ? t('files.InvalidName') : ''" />
      <btn color="primary" :disabled="!validItemName(newItemName)" @click="createFolder()"><icon name="check" />{{ t('files.CreateFolder') }}</btn>
    </overlay>
    <overlay v-model="archiveSelectedOpen" closable :title="t('files.ArchiveSelectedName')">
      <text-field v-model="newItemName" :error="newItemName && !validItemName(newItemName) ? t('files.InvalidName') : ''" />
      <btn color="primary" :disabled="!validItemName(newItemName)" @click="archiveSelected()"><icon name="check" />{{ t('files.ArchiveSelected') }}</btn>
    </overlay>
    <overlay v-model="loading" class="loader-overlay">
      <loader />
    </overlay>
    <overlay v-model="editorOpen" class="editor">
      <editor v-if="file" v-model="file" :read-only="!canEdit" :saving="saving" @save="saveFile($event)" @close="closeTreeFile" />
    </overlay>
  </div>
</template>

<style scoped>
.file-view-controls { display: flex; gap: .35rem; }
.tree-layout { display: grid; grid-template-columns: minmax(180px, 250px) minmax(0, 1fr); gap: .75rem; min-height: 28rem; }
.tree-sidebar, .tree-editor { padding: .75rem; border: 1px solid var(--color-background); border-radius: .5rem; background: var(--color-background-secondary); overflow: auto; }
.tree-editor { min-width: 0; }
.tree-placeholder { color: var(--color-text-secondary); text-align: center; margin: 8rem 1rem; }
.file-search { display: flex; align-items: center; gap: .75rem; margin: .75rem 0; }
.file-search input { min-width: 0; flex: 1; padding: .5rem; border: 1px solid var(--color-background); border-radius: .35rem; background: var(--color-background-secondary); color: inherit; }
.tree-editor :deep(.file-editor) { width: 100%; }
@media (max-width: 700px) {
  .tree-layout { grid-template-columns: 1fr; }
  .tree-sidebar { max-height: 18rem; }
}
</style>
