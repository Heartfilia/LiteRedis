import js from '@eslint/js'
import pluginVue from 'eslint-plugin-vue'
import prettier from 'eslint-config-prettier'

// 浏览器 + Wails 运行时里常见的全局变量
const browserGlobals = {
  window: 'readonly',
  document: 'readonly',
  navigator: 'readonly',
  localStorage: 'readonly',
  sessionStorage: 'readonly',
  console: 'readonly',
  fetch: 'readonly',
  setTimeout: 'readonly',
  clearTimeout: 'readonly',
  setInterval: 'readonly',
  clearInterval: 'readonly',
  requestAnimationFrame: 'readonly',
  cancelAnimationFrame: 'readonly',
  queueMicrotask: 'readonly',
  structuredClone: 'readonly',
  getComputedStyle: 'readonly',
  CustomEvent: 'readonly',
  Event: 'readonly',
  MouseEvent: 'readonly',
  KeyboardEvent: 'readonly',
  HTMLElement: 'readonly',
  Element: 'readonly',
  Node: 'readonly',
  NodeFilter: 'readonly',
  ResizeObserver: 'readonly',
  MutationObserver: 'readonly',
  IntersectionObserver: 'readonly',
  Blob: 'readonly',
  File: 'readonly',
  FileReader: 'readonly',
  URL: 'readonly',
  URLSearchParams: 'readonly',
  ClipboardItem: 'readonly',
  DOMParser: 'readonly',
  Image: 'readonly',
}

export default [
  {
    ignores: ['dist/**', 'node_modules/**', 'wailsjs/**', 'frontend/**'],
  },
  js.configs.recommended,
  ...pluginVue.configs['flat/essential'],
  {
    files: ['**/*.{js,vue}'],
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      globals: browserGlobals,
    },
    rules: {
      // 组件文件名已经是单一职责的中文/英文单词，不再强制 PascalCase + 多词
      'vue/multi-word-component-names': 'off',
      // 允许 catch {} 与空的分支占位（代码里大量「静默失败」的写法）
      'no-empty': ['error', { allowEmptyCatch: true }],
      'no-unused-vars': ['error', { argsIgnorePattern: '^_', caughtErrors: 'none' }],
    },
  },
  {
    files: ['**/*.test.js'],
    languageOptions: {
      globals: {
        ...browserGlobals,
        globalThis: 'writable',
      },
    },
  },
  // 关掉所有和 Prettier 冲突的格式类规则，格式交给 Prettier
  prettier,
]
