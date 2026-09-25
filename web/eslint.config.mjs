import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

export default defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    // The 3D rooms build three.js objects once and change them every frame.
    // That is how WebGL is driven, outside what React renders, so the
    // compiler's purity rules don't apply there.
    files: ["src/walk/**", "src/components/HeroReveal.tsx"],
    rules: {
      "react-hooks/immutability": "off",
      "react-hooks/purity": "off",
      "react-hooks/refs": "off",
    },
  },
  globalIgnores([".next/**", "out/**", "build/**", "next-env.d.ts"]),
]);
