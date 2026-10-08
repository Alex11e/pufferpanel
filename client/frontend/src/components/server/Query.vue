<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'

const props = defineProps({
  server: { type: Object, required: true }
})

const { t } = useI18n()

const data = ref({})
const queryError = ref(false)

let task = null
async function loadQuery() {
  try {
    const canQuery = await props.server.canQuery({ onError: () => ({ unavailable: true }) })
    if (canQuery === null) {
      queryError.value = true
      return
    }
    if (!canQuery) {
      data.value = {}
      queryError.value = false
      return
    }
    const result = await props.server.getQuery({ onError: () => ({ data: { networkError: true } }) })
    if (result?.networkError) {
      queryError.value = true
      return
    }
    data.value = result || {}
    queryError.value = false
  } catch {
    queryError.value = true
  }
}

onMounted(async () => {
  await loadQuery()
  task = setInterval(loadQuery, 30000)
})

onUnmounted(() => {
  if (task) clearInterval(task)
})
</script>

<template>
  <div class="query">
    <div v-if="queryError" class="alert error" role="alert">
      <span>{{ t('servers.QueryLoadFailed') }}</span>
      <btn variant="icon" :tooltip="t('servers.Retry')" @click="loadQuery"><icon name="reload" /></btn>
    </div>
    <div v-if="data.minecraft" class="minecraft">
      <span class="playerCountText">
        {{ t('servers.NumPlayersOnline', {current: data.minecraft.numPlayers, max: data.minecraft.maxPlayers}) }}
      </span>
      <progress
        class="playerCountBar"
        :value="data.minecraft.numPlayers"
        :max="data.minecraft.maxPlayers"
      />
      <div v-if="(data.minecraft.players || []).length > 0" class="players">
        <div v-for="player in data.minecraft.players || []" :key="player" v-text="player" />
      </div>
    </div>
  </div>
</template>