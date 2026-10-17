import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  // Emit the production build into ../web so the Go binary can embed/serve it
  // for single-service deployment (e.g. Render).
  build: {
    outDir: '../web',
    emptyOutDir: true,
  },
})
