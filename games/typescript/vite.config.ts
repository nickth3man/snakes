import { defineConfig } from "vite";

export default defineConfig(({ mode }) => ({
  // Deployed under https://<user>.github.io/snakes/typescript/
  base: mode === "production" ? "/snakes/typescript/" : "/",
  server: {
    open: true,
  },
}));
