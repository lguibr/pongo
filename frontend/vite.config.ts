// vite.config.ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // Proxy WebSocket connections on /subscribe to backend
      '/subscribe': {
        target: 'ws://localhost:3001',
        ws: true,
      },
    },
  },
});
