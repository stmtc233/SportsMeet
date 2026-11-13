import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "SportsMeet",
  identifier: "io.github.stmtc233.sportsmeet",
  version: "0.1.0",
  // `mygo dev` runs devCommand and loads devUrl; `mygo build` runs
  // buildCommand and embeds frontendDist into the app.
  devUrl: "http://localhost:5173",
  devCommand: "bun run dev:web",
  buildCommand: "bun run build:web",
  frontendDist: "dist",
  // resources/brand holds design sources; the app only needs icon.png.
  resourcesExclude: ["brand"],
  bindings: "src/mygo.ts",
  out: "build",
});
