<script setup>
import { onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Icon from '@/components/ui/Icon.vue'
import Overlay from '@/components/ui/Overlay.vue'
import TextField from '@/components/ui/TextField.vue'
import { serverIconName } from '@/utils/serverIcon'
import Status from './Status.vue'
import Controls from './Controls.vue'

const props = defineProps({
  server: { type: Object, required: true }
})

const { t } = useI18n()
const edit = ref(false)
const name = ref(props.server.name)
const iconName = ref(serverIconName(props.server))
const stopListening = props.server.on('metadataUpdated', () => {
  iconName.value = serverIconName(props.server)
})

onUnmounted(stopListening)

async function updateName() {
  await props.server.updateName(name.value)
  edit.value = false
}
</script>

<template>
  <h1 class="server-header">
    <Status :server="server" />
    <icon class="server-configured-icon" :name="iconName" />
    <span class="name">
      {{ server.name }}
    </span>
    <span v-if="server.tags" class="subline">{{ server.tags }}</span>
    <btn v-if="server.hasScope('server.name.edit')" class="rename" variant="icon" :tooltip="t('servers.EditName')" @click="edit = !edit"><icon name="edit" /></btn>
    <controls :server="server" />
  </h1>
  <overlay v-model="edit" :title="t('servers.EditName')" closable class="server-name">
    <text-field v-model="name" />
    <btn color="primary" @click="updateName()"><icon name="save" />{{ t('common.Save') }}</btn>
  </overlay>
</template>

<style scoped>
.server-configured-icon { flex: 0 0 auto; }
</style>
