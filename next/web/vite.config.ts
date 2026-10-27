import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

const api = process.env.WTM_API ?? 'http://127.0.0.1:4777'

// The build lands where next/internal/web embeds it; `make ui` is the only writer.
export default defineConfig({
  plugins: [tanstackRouter({ target: 'react', autoCodeSplitting: true }), react(), tailwindcss()],
  resolve: { alias: { '@': path.resolve(import.meta.dirname, './src') } },
  build: { outDir: '../internal/web/dist', emptyOutDir: true, sourcemap: false },
  test: {
    include: ['src/**/*.test.{ts,tsx}', 'lint/*.test.ts'],
    environment: 'happy-dom',
    setupFiles: ['src/test/setup.ts'],
    // a whole wizard run through the app takes 5 s alone on a busy machine, vitest's default timeout
    testTimeout: 15_000,
  },
  server: {
    // Dev mode: the page comes from Vite, /api from the Go server of `wtm-next ui`.
    proxy: { '/api': { target: api, changeOrigin: true, headers: { origin: api } } },
  },
})
