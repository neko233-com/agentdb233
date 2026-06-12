import { mkdir, readdir, readFile, stat, writeFile } from "node:fs/promises";
import path from "node:path";
import { createHash } from "node:crypto";
import { createGzip } from "node:zlib";
import { Readable } from "node:stream";
import { pipeline } from "node:stream/promises";

const skillDir = process.argv[2];
const outDir = process.argv[3] ?? "dist";
if (!skillDir) {
  throw new Error("usage: node scripts/pack-skill.mjs <skill-dir> [out-dir]");
}

const root = path.resolve(skillDir);
const skillName = path.basename(root);
await mkdir(outDir, { recursive: true });
const files = await walk(root);
const payload = {};
for (const file of files) {
  const rel = path.relative(root, file).replaceAll(path.sep, "/");
  payload[rel] = await readFile(file, "utf8");
}
const json = JSON.stringify({ name: skillName, files: payload }, null, 2);
const jsonPath = path.join(outDir, `${skillName}.skill.json`);
const gzPath = `${jsonPath}.gz`;
await writeFile(jsonPath, `${json}\n`);
await pipeline(Readable.from(json), createGzip(), await import("node:fs").then(fs => fs.createWriteStream(gzPath)));
const sha = createHash("sha256").update(json).digest("hex");
await writeFile(`${jsonPath}.sha256`, `${sha}  ${path.basename(jsonPath)}\n`);
console.log(`packed ${jsonPath}`);
console.log(`packed ${gzPath}`);

async function walk(dir) {
  const entries = await readdir(dir);
  const out = [];
  for (const entry of entries) {
    const file = path.join(dir, entry);
    const info = await stat(file);
    if (info.isDirectory()) {
      out.push(...await walk(file));
    } else {
      out.push(file);
    }
  }
  return out.sort();
}
