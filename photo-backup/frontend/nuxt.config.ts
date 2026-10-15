// WebKit บน macOS อาจบล็อก HMR ที่ใช้ wails.localhost
// จึงบังคับให้ Vite เชื่อมต่อผ่าน localhost ตามแนวทางในบทเรียน
function forceHmrLocalhost() {
  return {
    name: "wails-force-hmr-localhost",
    apply: "serve" as const,
    configureServer(server: any) {
      const BufferClass: any = (globalThis as any).Buffer;

      server.middlewares.stack.unshift({
        route: "",
        handle(req: any, res: any, next: any) {
          const url: string = req.url || "";
          if (!url.includes("@vite/client")) return next();

          delete req.headers["if-none-match"];
          delete req.headers["if-modified-since"];

          const originalEnd = res.end.bind(res);
          const chunks: any[] = [];

          res.write = function (chunk: any, ...rest: any[]) {
            if (chunk) chunks.push(BufferClass.from(chunk));
            const callback = rest.find((value) => typeof value === "function");
            if (callback) callback();
            return true;
          };

          res.end = function (chunk: any, ...rest: any[]) {
            if (typeof chunk === "function") chunk = undefined;
            if (chunk) chunks.push(BufferClass.from(chunk));

            let body: string = BufferClass.concat(chunks).toString("utf8");
            body = body.replace(
              "null || importMetaUrl.hostname",
              '"localhost"',
            );
            res.setHeader("content-length", BufferClass.byteLength(body));

            const callback = rest.find((value) => typeof value === "function");
            return originalEnd(body, callback);
          };

          next();
        },
      });
    },
  };
}

export default defineNuxtConfig({
  compatibilityDate: "2025-07-15",
  devtools: { enabled: true },
  ssr: false,
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
