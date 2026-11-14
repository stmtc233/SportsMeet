import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "SportsMeet",
  identifier: "io.github.stmtc233.sportsmeet",
  version: "0.1.0-beta.3",
  // `mygo dev` runs devCommand and loads devUrl; `mygo build` runs
  // buildCommand and embeds frontendDist into the app.
  devUrl: "http://localhost:5173",
  devCommand: "bun run dev:web",
  buildCommand: "bun run build:web",
  frontendDist: "dist",
  // resources/brand holds design sources; the app only needs icon.png.
  resourcesExclude: ["brand"],
  // Signed updates from the GitHub releases; the secret key is the
  // MYGO_UPDATER_PRIVATE_KEY secret of the repository.
  updates: {
    publicKey: "xQEIqRRBjy9HC/FSMbtubdq4t5SpajjyDWxlyQYHg/I=",
    github: "stmtc233/SportsMeet",
  },
  bindings: "src/mygo.ts",
  out: "build",
});
