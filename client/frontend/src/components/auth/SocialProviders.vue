<script setup>
import { inject, onMounted, ref } from 'vue'
import Icon from '@/components/ui/Icon.vue'

const api = inject('api')
const providers = ref([])

onMounted(async () => {
  try {
    const response = await api.get('/auth/social/providers')
    providers.value = response.data || []
  } catch {
    providers.value = []
  }
})
</script>

<template>
  <div v-if="providers.length" class="social-providers">
    <p>Vagy folytasd ezzel:</p>
    <a v-for="provider in providers" :key="provider.key" :href="`/auth/social/${encodeURIComponent(provider.key)}`">
      <icon name="account" />
      {{ provider.name }}
    </a>
  </div>
</template>

<style scoped>
.social-providers { display: grid; gap: 8px; margin-top: 16px; }
.social-providers p { text-align: center; color: var(--color-text-secondary); }
.social-providers a { display: flex; justify-content: center; align-items: center; gap: 8px; padding: 10px 12px; color: var(--color-text); text-decoration: none; border: 1px solid var(--color-background-secondary); border-radius: 5px; }
.social-providers a:hover { border-color: var(--color-primary); }
.social-error { color: var(--color-error); }
</style>