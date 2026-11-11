<script setup>
import { ref, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Overlay from '@/components/ui/Overlay.vue'
import { uploadFiles as uploadFileBatch } from './uploadFiles.mjs'

const { t } = useI18n()
const toast = inject('toast')

const props = defineProps({
  server: { type: Object, required: true },
  folder: { type: Boolean, default: () => false },
  hotkey: { type: String, default: () => undefined },
  path: { type: String, required: true }
})

const emit = defineEmits(['uploaded'])

const input = ref(null)
const uploading = ref(false)
const uploadState = ref(null)

function trigger() {
  input.value.click()
}

async function uploadFiles(event) {
  const inputElement = event.target
  const files = Array.from(inputElement.files)
  if (!files.length || uploading.value) return
  const destination = props.path
  uploadState.value = {
    state: 'preparing',
    current: 0,
    total: files.length,
    files: files.map(file => ({ name: file.webkitRelativePath || file.name, size: file.size, progress: 0 }))
  }
  uploading.value = true
  try {
    await uploadFileBatch(props.server, destination, files, props.folder, state => {
      uploadState.value.state = state
    }, (index, loaded) => {
      uploadState.value.current = index
      uploadState.value.files[index].progress = Math.min(loaded, files[index].size)
    }, path => window.confirm(t('files.ConfirmOverwrite', { path })))
    toast.success(t('files.UploadSuccess'))
  } catch(e) {
    if (e.code !== 'UPLOAD_CANCELLED') console.error('file upload failed', e)
    const key = e.code === 'UPLOAD_CANCELLED' ? 'files.UploadCancelled' : e.code === 'CREATE_UNAVAILABLE' ? 'files.CreateUnavailable' : 'files.UploadFailed'
    toast.error(t(key))
  } finally {
    emit('uploaded')
    uploading.value = false
    uploadState.value = null
    inputElement.value = ''
  }
}
</script>

<template>
  <div>
    <btn v-hotkey="hotkey" variant="icon" :tooltip="t(folder ? 'files.UploadFolder' : 'files.UploadFile')" @click="trigger()">
      <icon :name="folder ? 'folder-upload' : 'file-upload'" />
    </btn>
    <input ref="input" type="file" multiple :webkitdirectory="folder" @change="uploadFiles" />
    <overlay v-model="uploading" class="file-upload-progress" :title="t('files.UploadProgress')">
      <div v-if="uploadState && uploadState.state === 'preparing'">
        <div class="progress">
          <span v-text="t('files.PreparingUpload')" />
          <progress />
        </div>
      </div>
      <div v-if="uploadState && uploadState.state === 'create folders'">
        <div class="progress">
          <span v-text="t('files.CreatingFolders')" />
          <progress />
        </div>
      </div>
      <div v-if="uploadState && uploadState.state === 'upload files'">
        <div class="upload-file-count" v-text="t('files.CurrentlyUploading', { current: uploadState.current + 1, total: uploadState.total })" />
        <div class="upload-file-name" v-text="uploadState.files[uploadState.current].name" />
        <div class="progress">
          <span v-text="t('files.Current')" />
          <progress
            :max="uploadState.files[uploadState.current].size"
            :value="uploadState.files[uploadState.current].progress"
          />
        </div>
        <div class="progress">
          <span v-text="t('files.Total')" />
          <progress
            :max="uploadState.files.reduce((a, b) => a + b.size, 0)"
            :value="uploadState.files.reduce((a, b) => a + b.progress, 0)"
          />
        </div>
      </div>
    </overlay>
  </div>
</template>
