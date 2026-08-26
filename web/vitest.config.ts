import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      // 与 vite.config.ts 保持一致：共享包直连源码
      '@prerender/utils': path.resolve(__dirname, '../../packages/utils/src/main.ts'),
      '@prerender/design-tokens': path.resolve(__dirname, '../../packages/design-tokens/src/main.ts'),
    },
  },
  test: {
    globals: true,
    environment: 'happy-dom',
    include: ['src/**/__tests__/**/*.test.{ts,tsx}'],
    exclude: [
      '**/node_modules/**',
      '**/tests/**',
      '**/dist/**',
    ],
    setupFiles: ['./src/tests/setup.ts'],
    // threads 池：forks 池在本项目存在 worker 收尾内存泄漏（OOM）
    pool: 'threads',
    // vitest 4 默认 worker 内存上限过低，React 组件测试易触发 ERR_WORKER_OUT_OF_MEMORY
    poolOptions: {
      threads: {
        maxThreads: 2,
      },
    },
    testTimeout: 15000,
  },
})
