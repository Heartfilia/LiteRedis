<template>
  <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <template v-if="name === 'auto'">
      <rect x="3" y="4.5" width="18" height="12.5" rx="2.2" stroke="currentColor" stroke-width="1.8" />
      <path d="M8.5 20.6h7" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
    </template>
    <template v-else-if="name === 'green'">
      <path d="M2.8 12S6 6.4 12 6.4 21.2 12 21.2 12 18 17.6 12 17.6 2.8 12 2.8 12z" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" />
      <circle cx="12" cy="12" r="2.6" stroke="currentColor" stroke-width="1.7" />
    </template>
    <template v-else-if="name === 'dark'">
      <path d="M18 14.5A7.5 7.5 0 119.5 6 6 6 0 0018 14.5z" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" />
    </template>
    <template v-else>
      <circle cx="12" cy="12" r="4" stroke="currentColor" stroke-width="1.8" />
      <path
        d="M12 2.5v2.2M12 19.3v2.2M21.5 12h-2.2M4.7 12H2.5M18.7 5.3l-1.6 1.6M6.9 17.1l-1.6 1.6M18.7 18.7l-1.6-1.6M6.9 6.9L5.3 5.3"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
      />
    </template>
  </svg>
</template>

<script setup>
// 主题图标：light 太阳 / green 眼睛 / dark 月亮 / auto 显示器
defineProps({
  name: { type: String, default: 'light' },
})
</script>

<style scoped>
svg {
  width: 14px;
  height: 14px;
  display: block;
}
</style>
