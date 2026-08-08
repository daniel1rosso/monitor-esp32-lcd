import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    {
      name: 'cloudflare-rocket-loader-opt-out',
      enforce: 'post',
      transformIndexHtml: {
        order: 'post',
        handler: (html) => html.replace('<script type="module"', '<script data-cfasync="false" type="module"'),
      },
    },
  ],
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
  },
})
