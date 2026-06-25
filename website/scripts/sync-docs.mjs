// Sync + transform the repo's `docs/**/*.md` (the source of truth, kept
// GitHub-faithful and contract-anchored) into the Starlight content collection
// at `src/content/docs/`. Generated output is gitignored; never edit it by hand.
//
// Transforms applied per page:
//   - inject Starlight frontmatter (title from the first H1, description from
//     the first paragraph), stripping the now-duplicate H1 from the body;
//   - rewrite relative `*.md` links to base-aware Starlight routes;
//   - rewrite `assets/**` image references to the copied public asset URLs;
//   - drop mkdocs-only `markdown` HTML attributes.
//
// `index.md` is intentionally skipped: the landing page is the hand-authored
// splash at `src/content/docs/index.mdx`.

import { promises as fs } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const DOCS_DIR = path.resolve(SCRIPT_DIR, '../../docs');
const OUT_DIR = path.resolve(SCRIPT_DIR, '../src/content/docs');
const PUBLIC_ASSETS = path.resolve(SCRIPT_DIR, '../public/assets');

// Must match `base` in astro.config.mjs (project Pages path).
const BASE = '/slipway';

const KEEP_IN_OUT = new Set(['index.mdx']);
const IMAGE_EXT = /\.(svg|png|jpe?g|gif|webp|avif)$/i;

async function walk(dir, rootRel = '') {
  const out = [];
  for (const entry of await fs.readdir(dir, { withFileTypes: true })) {
    const abs = path.join(dir, entry.name);
    const rel = path.posix.join(rootRel, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === 'assets') continue; // copied wholesale, not parsed
      out.push(...(await walk(abs, rel)));
    } else if (entry.name.endsWith('.md')) {
      out.push(rel);
    }
  }
  return out;
}

function routeFor(docsRelNoExt) {
  if (docsRelNoExt === 'index') return `${BASE}/`;
  return `${BASE}/${docsRelNoExt}/`;
}

function rewriteTarget(target, currentDir) {
  const trimmed = target.trim();
  if (/^(https?:|mailto:|tel:|\/\/|#)/i.test(trimmed)) return null; // external / anchor-only
  const [pathPart, anchor] = trimmed.split('#');
  const suffix = anchor ? `#${anchor}` : '';

  if (pathPart.endsWith('.md')) {
    const resolved = path.posix.normalize(path.posix.join(currentDir, pathPart));
    return routeFor(resolved.replace(/\.md$/, '')) + suffix;
  }
  if (IMAGE_EXT.test(pathPart) || pathPart.includes('assets/')) {
    const resolved = path.posix.normalize(path.posix.join(currentDir, pathPart));
    // docs/assets/... is copied to public/assets/..., served at `${BASE}/assets/...`
    const idx = resolved.indexOf('assets/');
    if (idx >= 0) return `${BASE}/${resolved.slice(idx)}${suffix}`;
  }
  return null;
}

function rewriteLinks(body, currentDir) {
  return body.replace(/(\]\()([^)]+)(\))/g, (whole, open, target, close) => {
    const next = rewriteTarget(target, currentDir);
    return next === null ? whole : `${open}${next}${close}`;
  });
}

function deriveDescription(body) {
  const lines = body.split('\n');
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line) continue;
    if (/^[#>|`]|^!\[|^<|^-{3,}|^\d+\.\s|^[-*]\s/.test(line)) continue; // skip headings, tables, code, html, lists, images
    const text = line
      .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1') // links -> text
      .replace(/[`*_]/g, '')
      .replace(/\s+/g, ' ')
      .trim();
    if (text.length < 12) continue;
    return text.length > 158 ? `${text.slice(0, 155).replace(/\s+\S*$/, '')}…` : text;
  }
  return undefined;
}

function yamlQuote(value) {
  return `"${value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}

function transform(raw, rel) {
  const currentDir = path.posix.dirname(rel); // '.' for top-level
  let body = raw.replace(/\r\n/g, '\n');

  // Title from the first H1, which is then removed (Starlight renders the
  // frontmatter title as the page H1).
  let title;
  body = body.replace(/^#\s+(.+?)\s*$/m, (_m, heading) => {
    if (title === undefined) {
      title = heading.trim();
      return '';
    }
    return _m;
  });
  if (!title) {
    title = path.posix
      .basename(rel, '.md')
      .replace(/[-_]/g, ' ')
      .replace(/\b\w/g, (c) => c.toUpperCase());
  }

  const description = deriveDescription(body);
  body = rewriteLinks(body, currentDir);
  body = body.replace(/\s+markdown(=("1"|'1'))?(?=[\s>])/g, ''); // drop mkdocs md_in_html attr
  body = body.replace(/^\n+/, ''); // trim leading blank lines left by H1 removal

  const fm = ['---', `title: ${yamlQuote(title)}`];
  if (description) fm.push(`description: ${yamlQuote(description)}`);
  fm.push('---', '');
  return `${fm.join('\n')}\n${body.replace(/\n*$/, '\n')}`;
}

async function cleanGenerated() {
  await fs.mkdir(OUT_DIR, { recursive: true });
  for (const entry of await fs.readdir(OUT_DIR, { withFileTypes: true })) {
    if (KEEP_IN_OUT.has(entry.name)) continue;
    await fs.rm(path.join(OUT_DIR, entry.name), { recursive: true, force: true });
  }
  await fs.rm(PUBLIC_ASSETS, { recursive: true, force: true });
}

async function main() {
  await cleanGenerated();

  const files = (await walk(DOCS_DIR)).filter((rel) => rel !== 'index.md');
  for (const rel of files) {
    const raw = await fs.readFile(path.join(DOCS_DIR, rel), 'utf8');
    const outPath = path.join(OUT_DIR, rel);
    await fs.mkdir(path.dirname(outPath), { recursive: true });
    await fs.writeFile(outPath, transform(raw, rel));
  }

  // Copy docs/assets wholesale -> public/assets (served at `${BASE}/assets/**`).
  const assetsSrc = path.join(DOCS_DIR, 'assets');
  if (await fs.stat(assetsSrc).then(() => true).catch(() => false)) {
    await fs.cp(assetsSrc, PUBLIC_ASSETS, { recursive: true });
  }

  console.log(`sync-docs: ${files.length} pages -> ${path.relative(process.cwd(), OUT_DIR)}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
