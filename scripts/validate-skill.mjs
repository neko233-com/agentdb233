import { readFile } from "node:fs/promises";
import path from "node:path";

const dir = process.argv[2];
if (!dir) {
  throw new Error("usage: node scripts/validate-skill.mjs <skill-dir>");
}

const skillPath = path.join(dir, "SKILL.md");
const body = await readFile(skillPath, "utf8");
if (!body.startsWith("---\n")) {
  throw new Error("SKILL.md missing YAML frontmatter");
}
const end = body.indexOf("\n---", 4);
if (end < 0) {
  throw new Error("SKILL.md frontmatter not closed");
}
const frontmatter = body.slice(4, end).trim();
for (const key of ["name:", "description:"]) {
  if (!frontmatter.includes(key)) {
    throw new Error(`SKILL.md missing ${key}`);
  }
}
const name = frontmatter.match(/^name:\s*([a-z0-9-]+)\s*$/m)?.[1];
if (!name) {
  throw new Error("skill name must be lowercase letters, digits, hyphens");
}
if (name.length > 64) {
  throw new Error("skill name too long");
}
const content = body.slice(end + 4).trim();
if (content.length < 50) {
  throw new Error("SKILL.md body too short");
}
console.log(`valid skill: ${name}`);
