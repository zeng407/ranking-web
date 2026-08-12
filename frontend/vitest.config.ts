import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
    // A working localStorage/sessionStorage. Node's own is experimental and, on Node 25
    // without a valid --localstorage-file, present but with every method undefined.
    // See src/test/webStorage.ts.
    setupFiles: ['./src/test/webStorage.ts'],
    coverage: {
      reporter: ['text', 'json-summary'],
    },
  },
})
