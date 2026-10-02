<script setup>
import HostingTabs from '../server/HostingTabs.vue'

defineProps({
  server: { type: Object, required: true }
})
</script>

<template>
  <hosting-tabs :server="server" mode="db" />
</template>
