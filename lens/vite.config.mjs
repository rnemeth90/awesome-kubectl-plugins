// Builds the extension's two entry points, one per run: the renderer with
// `vite build`, the main entry with `vite build --mode main`. Modelled on the
// official Freelens extension example.
import { builtinModules } from "node:module";
import react from "@vitejs/plugin-react";
import { defaultServerConditions, defaultServerMainFields, defineConfig } from "vite";
import { hostModules } from "./build/vite-plugin-host-modules.mjs";

const entryPoints = {
  renderer: { entry: "src/renderer/index.tsx", emptyOutDir: true, external: [], platform: "browser" },
  main: {
    entry: "src/main/index.ts",
    emptyOutDir: false,
    external: ["electron", /^electron\//, /^node:/, ...builtinModules],
    platform: "node",
  },
};

export default defineConfig(({ mode }) => {
  const name = mode === "main" ? "main" : "renderer";
  const entryPoint = entryPoints[name];

  return {
    plugins: [hostModules(name), react()],
    ...(name === "renderer"
      ? { define: { "process.env.NODE_ENV": JSON.stringify("production") } }
      : {
          resolve: { conditions: [...defaultServerConditions], mainFields: [...defaultServerMainFields] },
          define: { __dirname: "import.meta.dirname", __filename: "import.meta.filename" },
        }),
    build: {
      target: "esnext",
      minify: false,
      sourcemap: true,
      emptyOutDir: entryPoint.emptyOutDir,
      lib: { entry: entryPoint.entry, formats: ["es"], fileName: () => `${name}.js`, cssFileName: name },
      rolldownOptions: { external: entryPoint.external, platform: entryPoint.platform },
    },
  };
});
