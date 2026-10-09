import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
export default defineConfig({
  plugins: [vue()],
  server: {
    host: true,
    port: 3346,
    strictPort: true,
    proxy: {
      "/api": { target: "http://localhost:8083" },
      "/ws": { target: "ws://localhost:8083", ws: true },
    },
  },
});
