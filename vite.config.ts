import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { createApiRouter } from './src/server/index';
import express from 'express';

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    {
      name: 'api-server-middleware',
      configureServer(server) {
        const app = express();
        app.use(express.json());
        app.use('/api', createApiRouter());
        server.middlewares.use(app);
      },
    },
  ],
  server: {
    port: 5173,
    host: true,
  },
});
