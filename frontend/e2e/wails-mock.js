/**
 * 在页面加载前注入 Wails 的假后端。
 * 前端所有数据都走 window.go.main.App.XXX()，这里用 Proxy 兜底，
 * 只给冒烟测试真正用到的几个方法提供数据。
 */

export const CONNECTIONS = [
  { id: 'conn-dev', name: 'dev', host: '127.0.0.1', port: 6379, db: 0, group: '' },
  { id: 'conn-prod', name: 'prod-backup', host: '10.0.0.5', port: 6380, db: 0, group: '' },
]

export async function installWailsMock(page, options = {}) {
  await page.addInitScript(
    ({ connections, keyCount, tabsEnabled }) => {
      const settings = {
        key_scan_count: 200,
        hash_load_count: 20,
        list_load_count: 20,
        set_load_count: 20,
        zset_load_count: 20,
        stream_load_count: 20,
        search_history_limit: 10,
        key_display_mode: 'tree',
        font_size_level: 'small',
        watermark_enabled: false,
        watermark_text: '',
        watermark_size: 16,
        watermark_angle: -22,
        watermark_opacity: 12,
        watermark_density: 3,
        language: 'zh',
        local_api_enabled: false,
        local_api_port: 64201,
        proxy_enabled: false,
        proxy_url: '',
        last_seen_version: '1.0.0',
        onboarding_done: true,
        connection_tabs_enabled: tabsEnabled,
        command_history: [],
      }

      const keys = Array.from({ length: keyCount }, (_, index) => ({
        name: 'user:' + (index % 60) + ':field:' + index,
        type: ['string', 'hash', 'list', 'set', 'zset'][index % 5],
        ttl: index % 7 === 0 ? 3600 : -1,
      }))

      const impl = {
        ListConnections: () => connections,
        GetSettings: () => ({ ...settings }),
        SaveSettings: () => ({ success: true }),
        GetAppVersion: () => '1.0.0',
        GetReleaseNotes: () => ({ found: false, body: '', url: '' }),
        MarkVersionSeen: () => ({ success: true }),
        CompleteOnboarding: () => ({ success: true }),
        GetCommandHistory: () => ['GET user:1:field:1', 'INFO memory'],
        PushCommandHistory: () => ({ success: true }),
        ClearCommandHistory: () => ({ success: true }),
        GetDatabases: () => 16,
        GetConnectionDB: () => 0,
        Connect: () => ({ success: true, init_db: 0 }),
        Disconnect: () => ({ success: true }),
        IsConnected: () => true,
        PingConnection: () => ({ success: true }),
        SelectDB: () => ({ success: true }),
        DbSize: () => 5000,
        ScanKeys: (connId, pattern, count) => ({
          keys: keys.slice(0, Math.min(count || 200, keys.length)),
          next_cursor: 0,
          has_more: false,
        }),
        GetValue: () => ({
          key: '',
          type: 'string',
          string_val: JSON.stringify(
            Object.fromEntries(
              Array.from({ length: 3000 }, (_, i) => ['field_' + String(i).padStart(4, '0'), 'value-' + i + '-' + 'x'.repeat(20)])
            )
          ),
        }),
        GetConnectionOverview: () => ({
          host: '127.0.0.1',
          port: 6379,
          used_memory: '1.2 MB',
          current_db: 0,
          total_keys: 5000,
          uptime_human: '2 天',
        }),
        ExecuteRedisCommand: () => ({ success: true, output: 'PONG' }),
        ExecuteRedisCommandForce: () => ({ success: true, output: 'PONG' }),
        GetLocalAPIInfo: () => ({ running: false, port: 0, token: '', error: '' }),
        GetDiagnostics: () => ({ connections: [] }),
        GetUpdateInfo: () => ({ need_update: false, latest: '1.0.0' }),
        CheckLatestRelease: () => ({ need_update: false, latest: '1.0.0' }),
      }

      window.go = {
        main: {
          App: new Proxy(
            {},
            {
              get: (_target, method) => (...args) =>
                Promise.resolve(typeof impl[method] === 'function' ? impl[method](...args) : { success: true }),
            }
          ),
        },
      }
      // runtime 里的方法一律当空实现，避免未绑定就抛错
      window.runtime = new Proxy(
        {},
        {
          get: () => () => {},
        }
      )
    },
    {
      connections: options.connections || CONNECTIONS,
      keyCount: options.keyCount || 3000,
      tabsEnabled: options.tabs === true,
    }
  )
}
