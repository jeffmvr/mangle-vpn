import { fileURLToPath, URL } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";

// The build goes straight into the Go server's embed directory and is
// compiled into the binary. The server answers index.html at "/" and serves
// everything under /static from the build's static directory, so the build
// maps onto the web root one for one. Note that `base` must stay "/" —
// setting it to "/static/" as well would emit /static/static/… and nothing
// would load.
export default defineConfig({
  plugins: [vue(), tailwindcss()],

  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },

  build: {
    outDir: "../internal/webui/dist",
    // Assets land in dist/static so the Go server's /static handler finds
    // them, while index.html stays at the root for the "/" route.
    assetsDir: "static",
    emptyOutDir: true,
  },

  server: {
    port: 5173,
    // Everything the Go server renders itself is proxied to it, so
    // `npm run dev` runs against a real backend rather than a mock. Vite
    // keeps "/" for the application shell.
    //
    // The target is plain HTTP because the backend marks its session
    // cookie Secure when it serves TLS, and a browser will not send a
    // Secure cookie to http://localhost. Run the backend with -insecure.
    proxy: Object.fromEntries(
      ["/api", "/login", "/logout", "/password", "/mfa", "/install", "/oauth", "/import", "/logo"].map((path) => [
        path,
        { target: "http://127.0.0.1:9443", changeOrigin: true },
      ]),
    ),
  },
});
