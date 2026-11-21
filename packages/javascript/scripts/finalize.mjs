import { writeFileSync, copyFileSync, mkdirSync } from "node:fs";

mkdirSync("dist", { recursive: true });
copyFileSync("dist/esm/index.d.ts", "dist/index.d.ts");
writeFileSync("dist/cjs/package.json", JSON.stringify({ type: "commonjs" }) + "\n");
