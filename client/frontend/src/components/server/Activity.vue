<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Loader from '@/components/ui/Loader.vue'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'

const props = defineProps({ server: { type: Object, required: true } })
const { t, locale } = useI18n()
const records = ref([])
const loading = ref(true)
const loadError = ref('')
const intl = new Intl.DateTimeFormat([locale.value.replace('_', '-'), 'en'], { dateStyle: 'medium', timeStyle: 'medium' })

async function loadActivity() {
  loading.value = true
  loadError.value = ''
  try {
    records.value = await props.server.getActivity({ onError: () => ({ data: null }) })
    if (!Array.isArray(records.value)) throw new Error()
  } catch {
    records.value = []
    loadError.value = t('servers.ActivityLoadFailed')
  } finally {
    loading.value = false
  }
}

onMounted(loadActivity)
</script>

<template>
  <div class="activity-log">
    <div class="activity-heading">
      <h2 v-text="t('servers.Activity')" />
      <btn variant="icon" :tooltip="t('servers.RefreshActivity')" :disabled="loading" @click="loadActivity"><icon :name="loading ? 'loading' : 'reload'" :spin="loading" /></btn>
    </div>
    <loader v-if="loading" />
    <div v-else-if="loadError" class="alert error" role="alert">
      <span v-text="loadError" />
      <btn @click="loadActivity"><icon name="reload" />{{ t('servers.Retry') }}</btn>
    </div>
    <div v-else-if="records.length === 0" class="alert info" v-text="t('servers.NoActivity')" />
    <div v-for="record in records" v-else :key="record.id" class="list-item">
      <div class="title">{{ record.username }} — {{ t(`servers.activity.${record.action}`) }}</div>
      <div class="subline">{{ record.details || t('servers.NoDetails') }} · {{ record.ipAddress }} · {{ intl.format(new Date(record.createdAt)) }}</div>
    </div>
  </div>
</template>

<style scoped>
.activity-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.activity-heading h2 { margin: 0; }
</style>
