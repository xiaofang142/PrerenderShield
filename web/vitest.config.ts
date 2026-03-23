import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
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
  },
})
