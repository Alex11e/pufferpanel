<script setup>
import { ref, inject, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { updateLocale, locales } from '@/plugins/i18n'
import Btn from '@/components/ui/Btn.vue'
import Dropdown from '@/components/ui/Dropdown.vue'
import Icon from '@/components/ui/Icon.vue'
import Loader from '@/components/ui/Loader.vue'
import ThemeSetting from '@/components/ui/ThemeSetting.vue'

const { t, locale } = useI18n()
const toast = inject('toast')
const themeApi = inject('theme')

const loading = ref(false)
const saving = ref(false)
const themeReady = ref(false)
const theme = ref(themeApi.getActiveTheme())
const themeSettings = ref({})
const selectedLocale = ref(locale.value)

onMounted(async () => {
  loading.value = true
  try {
    themeSettings.value = await themeApi.getThemeSettings()
    themeReady.value = true
  } catch (error) {
    console.error('Could not load theme settings', error)
    toast.error(t('users.ThemeLoadFailed'))
  } finally {
    loading.value = false
  }
})

async function themeChanged() {
  loading.value = true
  themeReady.value = false
  try {
    themeSettings.value = await themeApi.getThemeSettings(theme.value)
    themeReady.value = true
  } catch (error) {
    console.error('Could not load selected theme', error)
    toast.error(t('users.ThemeLoadFailed'))
    theme.value = themeApi.getActiveTheme()
  } finally {
    loading.value = false
  }
}

async function savePreferences() {
  if (saving.value) return
  const settings = {}
  Object.keys(themeSettings.value).map(key => {
    settings[key] = themeSettings.value[key].current
  })

  saving.value = true
  try {
    if (locale.value !== selectedLocale.value) {
      await updateLocale(selectedLocale.value)
    }
    await themeApi.setTheme(theme.value, settings)
    toast.success(t('users.PreferencesUpdated'))
  } catch (error) {
    console.error('Could not save theme preferences', error)
    toast.error(t('users.ThemeSaveFailed'))
  } finally {
    saving.value = false
  }
}

function updateThemeSetting(name, newSetting) {
  themeSettings.value[name] = newSetting
}
</script>

<template>
  <div v-if="loading" class="preferences loading"><loader /></div>
  <div v-else class="preferences">
    <h1 v-text="t('users.Preferences')" />
    <dropdown v-model="selectedLocale" class="locale-select" :options="locales" :label="t('common.Language')" :hint="`[${t('common.HelpTranslate')}](https://translate.pufferpanel.com)`">
      <template #singlelabel="{ value }">
        <div class="multiselect-single-label">
          <span :data-locale="value.value" /> {{ value.label }}
        </div>
      </template>

      <template #option="{ option }">
        <span :data-locale="option.value" /> {{ option.label }}
      </template>
    </dropdown>
    <dropdown v-model="theme" :options="$theme.getThemes()" :label="t('common.theme.Theme')" @change="themeChanged()" />
    <theme-setting v-for="(setting, name) in themeSettings" :key="name" :model-value="setting" @update:modelValue="updateThemeSetting(name, $event)" />
    <btn color="primary" :disabled="saving || !themeReady" @click="savePreferences()"><icon name="save" />{{ t('users.SavePreferences') }}</btn>
  </div>
</template>
