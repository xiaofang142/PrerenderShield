import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      // 共享包直连源码，免构建链
      '@prerender/utils': path.resolve(__dirname, '../../packages/utils/src/main.ts'),
      '@prerender/design-tokens': path.resolve(__dirname, '../../packages/design-tokens/src/main.ts'),
    },
  },
  build: {
    chunkSizeWarningLimit: 800,
    rollupOptions: {
      output: {
        manualChunks: {
          react: ['react', 'react-dom', 'react-router-dom'],
          antd: ['antd'],
          icons: ['@ant-design/icons'],
          echarts: ['echarts'],
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:9598',
        changeOrigin: true
      },
      '/ws': {
        target: 'http://localhost:9598',
        changeOrigin: true,
        ws: true
      }
    }
  }
})
