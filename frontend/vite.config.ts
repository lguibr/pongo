// File: frontend/vite.config.ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    // Remove or comment out the proxy configuration
    // proxy: {
    //   // Proxy WebSocket connections on /subscribe to backend
    //   '/subscribe': {
    //     target: 'ws://localhost:3001',
    //     ws: true,
    //   },
    // },
  },
});