// SPDX-License-Identifier: AGPL-3.0-or-later

// Bundles entry.mjs into internal/web/assets/prosemirror.min.js. Run with
// `just vendor-prosemirror`; the versions are the exact pins in package.json
// and package-lock.json, and REUSE.toml records them beside the file.
import { build } from "esbuild";
import { readFileSync } from "node:fs";

const lock = JSON.parse(readFileSync(new URL("./package-lock.json", import.meta.url)));
const bundled = Object.entries(lock.packages)
  .filter(([path, p]) => path.startsWith("node_modules/") && !/esbuild|@types|argparse/.test(path))
  .map(([path, p]) => `${path.slice("node_modules/".length)}@${p.version} (${p.license})`)
  .sort();

await build({
  entryPoints: [new URL("./entry.mjs", import.meta.url).pathname],
  bundle: true,
  minify: true,
  format: "iife",
  globalName: "tixPM",
  target: ["es2019"],
  legalComments: "none",
  // markdown-it only decodes. The package index also pulls in the 23 KB
  // encoder table, which nothing here calls.
  alias: { entities: new URL("./node_modules/entities/lib/esm/decode.js", import.meta.url).pathname },
  charset: "utf8",
  outfile: new URL("../../../internal/web/assets/prosemirror.min.js", import.meta.url).pathname,
  banner: {
    js: "/* ProseMirror and markdown-it for the tix description editor. Built by\n" +
      "   scripts/vendor/prosemirror from: " + bundled.join(", ") + ".\n" +
      "   MIT: Copyright (C) Marijn Haverbeke and others (ProseMirror), Vitaly Puzrin and\n" +
      "   Alex Kocharin (markdown-it, linkify-it, mdurl, uc.micro), Mathias Bynens (punycode.js).\n" +
      "   BSD-2-Clause: Copyright (c) Felix Boehm (entities). See LICENSES/ and REUSE.toml. */",
  },
});
