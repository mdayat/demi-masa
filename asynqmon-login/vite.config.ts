import path from "path";
import solidPlugin from "vite-plugin-solid";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [solidPlugin()],
  server: {
    port: 3000,
  },
  build: {
    target: "esnext",
    outDir: ".solid",
  },
  resolve: {
    extensions: [".ts", ".tsx"],
    alias: {
      "@components": path.resolve(__dirname, "./src/components"),
      "@libs": path.resolve(__dirname, "./src/libs"),
      "@utils": path.resolve(__dirname, "./src/utils"),
      "@customTypes": path.resolve(__dirname, "./src/types"),
    },
  },
});
