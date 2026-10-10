<script setup>
import { ref, inject, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Btn from '@/components/ui/Btn.vue'
import Loader from '@/components/ui/Loader.vue'
import TextField from '@/components/ui/TextField.vue'

const { t } = useI18n()
const api = inject('api')
const toast = inject('toast')

const oldPass = ref('')
const newPass = ref('')
const confirmPass = ref('')
const hasLocalPassword = ref(false)
const loading = ref(true)

onMounted(async () => {
  try {
    hasLocalPassword.value = (await api.self.get()).hasLocalPassword
  } catch {
    hasLocalPassword.value = true
  } finally {
    loading.value = false
  }
})

function isValidPassword(p) {
  return p.length >= 8
}

function canSubmit() {
  return (!hasLocalPassword.value || isValidPassword(oldPass.value)) && isValidPassword(newPass.value) && newPass.value === confirmPass.value
}

async function submit() {
  if (!canSubmit()) return
  if (hasLocalPassword.value) {
    await api.self.changePassword(oldPass.value, newPass.value)
  } else {
    await api.self.setInitialPassword(newPass.value)
    hasLocalPassword.value = true
  }
  oldPass.value = ''
  newPass.value = ''
  confirmPass.value = ''
  toast.success(t('users.PasswordChanged'))
}
</script>

<template>
  <div class="changepassword">
    <h1 v-text="t('users.ChangePassword')" />
    <loader v-if="loading" />
    <form v-else>
      <p v-if="!hasLocalPassword" class="hint" v-text="t('users.SetPasswordHint')" />
      <text-field v-if="hasLocalPassword" v-model="oldPass" icon="lock" type="password" :label="t('users.OldPassword')" />
      <text-field v-model="newPass" icon="lock" type="password" :label="t('users.NewPassword')" />
      <text-field v-model="confirmPass" icon="lock" type="password" :label="t('users.ConfirmPassword')" />
      <btn :disabled="!canSubmit()" color="primary" @click="submit()"><icon name="save" />{{ t(hasLocalPassword ? 'users.ChangePassword' : 'users.SetPassword') }}</btn>
    </form>
  </div>
</template>
