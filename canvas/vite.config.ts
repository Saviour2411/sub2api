import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const canvasDir = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
    base: "/canvas-app/",
    plugins: [react()],
    resolve: { alias: { "@": resolve(canvasDir, "src") } },
    build: {
        outDir: "../backend/internal/web/dist/canvas-app",
        emptyOutDir: true,
    },
    server: {
        host: "0.0.0.0",
        port: 3001,
        proxy: {
            "/v1": "http://localhost:8080",
            "/v1beta": "http://localhost:8080",
        },
    },
});
