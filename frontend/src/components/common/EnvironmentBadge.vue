<template>
  <span
    v-if="environmentKey !== 'unspecified'"
    class="environment-badge"
    :class="[`environment-${environmentKey}`, `theme-${settingsStore.themeMode || 'light'}`]"
    :title="`${t('environment.label')}: ${environmentLabel}`"
    :aria-label="`${t('environment.label')}: ${environmentLabel}`"
  >
    {{ { development: 'D', staging: 'T', production: 'P' }[environmentKey] }}
  </span>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from '../../i18n/index.js'
import { useSettingsStore } from '../../stores/settings.js'

const props = defineProps({
  environment: { type: String, default: '' },
})
const { t } = useI18n()
const settingsStore = useSettingsStore()
const environmentKey = computed(() => ['development', 'staging', 'production'].includes(props.environment)
  ? props.environment
  : 'unspecified')
const environmentLabel = computed(() => t(`environment.${environmentKey.value}`))
</script>

<style scoped>
.environment-badge {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  justify-content: center;
  width: 15px;
  height: 15px;
  box-sizing: border-box;
  padding: 0;
  border: 1px solid #cbd5e1;
  border-radius: 4px;
  background: #f1f5f9;
  color: #475569;
  font-size: 9px;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
}
.environment-development { background: #ecfdf5; border-color: #a7f3d0; color: #065f46; }
.environment-staging { background: #fffbeb; border-color: #fcd34d; color: #92400e; }
.environment-production { background: #fff1f2; border-color: #fda4af; color: #9f1239; }
.environment-badge.theme-dark { background: #1e293b; border-color: #475569; color: #cbd5e1; }
.environment-development.theme-dark { background: #064e3b; border-color: #059669; color: #a7f3d0; }
.environment-staging.theme-dark { background: #451a03; border-color: #b45309; color: #fde68a; }
.environment-production.theme-dark { background: #4c0519; border-color: #be123c; color: #fecdd3; }
</style>
