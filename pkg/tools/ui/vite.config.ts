import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { viteSingleFile } from 'vite-plugin-singlefile'
import { resolve, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  plugins: [react(), viteSingleFile()],
  build: {
    rollupOptions: {
      input: {
        timeserieschart: resolve(__dirname, 'timeserieschart/index.html')
      }
    }
  }
})
