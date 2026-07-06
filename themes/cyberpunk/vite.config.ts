import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const apiBaseURLs = {
  development: 'http://127.0.0.1:8080',
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
    build: {
      outDir: 'dist',
      emptyOutDir: true
    },
    server: {
      host: '0.0.0.0',
      port: 5174,
      strictPort: true,
      proxy: {
        '/api': apiBaseURLs.development,
        '/healthz': apiBaseURLs.development
      }
    }
  }
})
