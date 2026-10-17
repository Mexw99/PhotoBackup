import { forceHmrLocalhost } from "./config/forceHmrLocalhost";

export default defineNuxtConfig({
  compatibilityDate: "2025-07-15",
  // Nuxt DevTools ใช้ RPC ข้าม origin ซึ่งไม่จำเป็นใน Wails WebView
  // และทำให้ WebKit บน macOS แสดงหน้าขาวพร้อม warning cross-origin
  devtools: { enabled: false },
  ssr: false,
  css: ["~/assets/css/main.css"],
  $production: {
    nitro: {
      output: {
        publicDir: "../frontend/dist",
      },
    },
  },
  router: {
    options: {
      hashMode: true,
    },
  },
  vite: {
    plugins: [forceHmrLocalhost()],
  },
});
