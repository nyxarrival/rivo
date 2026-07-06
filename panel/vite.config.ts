import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const apiBaseURLs = {
  development: 'http://127.0.0.1:8080',
  // 正式版如果和 Master 同域部署可留空；如果前后端分离部署，填正式 Master 地址。
  production: ''
}

export default defineConfig(({ command }) => {
  const isBuild = command === 'build'

  return {
    base: './',
    plugins: [vue()],
    define: {
      __RIVO_API_BASE_URL__: JSON.stringify(isBuild ? apiBaseURLs.production : '')
    },
    server: {
      host: '0.0.0.0',
      port: 5173,
      strictPort: true,
      proxy: {
        '/api': apiBaseURLs.development,
        '/healthz': apiBaseURLs.development
      }
    }
  }
})
