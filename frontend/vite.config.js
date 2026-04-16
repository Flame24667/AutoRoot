import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'path'

export default defineConfig({
    plugins: [react()],
    base: './', // ← CRITICAL: Makes asset paths relative for Electron
    build: {
        outDir: 'dist',
        assetsDir: 'assets',
        emptyOutDir: true,
        rollupOptions: {
        input: {
            main: resolve(__dirname, 'index.html'),
        },
        },
    },
    server: {
        port: 5173,
        strictPort: true,
    },
})