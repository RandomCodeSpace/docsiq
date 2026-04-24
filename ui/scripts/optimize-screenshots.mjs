// Optimise docs/screenshots/*.png in place via sharp.
//
// Targets:
//   - Keep pixel dimensions (don't downscale — we use @2x captures).
//   - Re-encode PNG with compression level 9 and palette where possible.
//   - Refuse to exit 0 if any resulting file is > 500 KB — that's our
//     published budget per screenshot.
//
// Usage:  node ui/scripts/optimize-screenshots.mjs

import { readdir, stat } from "node:fs/promises";
import path from "node:path";
import sharp from "sharp";

const DIR = path.resolve(
  path.dirname(new URL(import.meta.url).pathname),
  "..",
  "..",
  "docs",
  "screenshots",
);
const BUDGET_BYTES = 500 * 1024;

const entries = (await readdir(DIR)).filter((f) => f.endsWith(".png"));
if (entries.length === 0) {
  console.error("no PNGs found in", DIR);
  process.exit(1);
}

let failed = false;
for (const name of entries) {
  const p = path.join(DIR, name);
  const before = (await stat(p)).size;
  const buf = await sharp(p)
    .png({ compressionLevel: 9, palette: true, effort: 10 })
    .toBuffer();
  await sharp(buf).toFile(p);
  const after = (await stat(p)).size;
  const flag = after > BUDGET_BYTES ? "OVER BUDGET" : "ok";
  console.log(
    `${name.padEnd(18)}  ${Math.round(before / 1024)} KB → ${Math.round(after / 1024)} KB  [${flag}]`,
  );
  if (after > BUDGET_BYTES) failed = true;
}

process.exit(failed ? 2 : 0);
