<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Loader from '@/components/ui/Loader.vue'

const props = defineProps({ server: { type: Object, required: true } })
const { t, locale } = useI18n()
const records = ref(null)
const intl = new Intl.DateTimeFormat([locale.value.replace('_', '-'), 'en'], { dateStyle: 'medium', timeStyle: 'medium' })

onMounted(async () => { records.value = await props.server.getActivity() })
</script>

<template>
  <div class="activity-log">
    <loader v-if="records === null" />
    <div v-else-if="records.length === 0" class="alert info" v-text="t('servers.NoActivity')" />
    <div v-for="record in records" :key="record.id" class="list-item">
      <div class="title">{{ record.username }} — {{ record.action }}</div>
      <div class="subline">{{ record.details || t('servers.NoDetails') }} · {{ record.ipAddress }} · {{ intl.format(new Date(record.createdAt)) }}</div>
    </div>
  </div>
</template>
