import { defineConfig } from '@playwright/test'

// 默认用系统 Chrome（本机缓存里没有 Playwright 自带的 chromium 时也能跑）；
// CI 里装了 Playwright 的 chromium 后设置 PW_CHANNEL=bundled 即可切换。
const useBundledChromium = process.env.PW_CHANNEL === 'bundled'

export default defineConfig({
  testDir: './e2e',
  timeout: 45_000,
  expect: { timeout: 8_000 },
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['github']] : [['list']],
  use: {
    baseURL: 'http://127.0.0.1:5199',
    viewport: { width: 1440, height: 900 },
    trace: 'retain-on-failure',
    channel: useBundledChromium ? undefined : 'chrome',
  },
  webServer: {
    command: 'npm run dev -- --port 5199 --strictPort',
    url: 'http://127.0.0.1:5199',
    reuseExistingServer: !process.env.CI,
    timeout: 90_000,
  },
})
