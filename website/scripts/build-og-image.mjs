// Rasterize the social preview card at `public/og.png` from the brand wordmark.
//
// Every page declares `twitter:card: summary_large_image`, which reserves a
// 1.91:1 image slot; without `og:image` that slot renders empty. The card is
// generated rather than hand-drawn so it stays in step with the wordmark, and
// it is committed rather than built in CI because it changes only when the
// brand does.
//
// Deterministic by construction: the wordmark is a grid of solid rects with no
// text elements, so nothing here depends on a font being installed. Run
// `node scripts/build-og-image.mjs` after editing the wordmark.

import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import sharp from 'sharp';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const WORDMARK = path.join(HERE, '..', 'src', 'assets', 'slipway-wordmark.svg');
const OUTPUT = path.join(HERE, '..', 'public', 'og.png');

// Open Graph's canonical 1.91:1 slot. Facebook, X, Slack, and Discord all
// downscale from this; going smaller shows visible softening on retina.
const WIDTH = 1200;
const HEIGHT = 630;

// The wordmark's own canvas, read back from its viewBox so a redraw at a
// different size cannot silently shift the composition.
const VIEWBOX_RE = /viewBox="0 0 (\d+(?:\.\d+)?) (\d+(?:\.\d+)?)"/;

// The wordmark is a grid of 12x12 cells. Scaling it by an arbitrary factor puts
// cell edges on fractional pixels, and the renderer antialiases every one of
// them — 1087 rects worth of seams, which reads as a mesh laid over the letters.
// Pick whole pixels per cell instead, and derive the width from that: 7px per
// cell gives a 777px mark on a 1200px card. Offsets are rounded for the same
// reason, so every edge lands on a device pixel.
const WORDMARK_CELL = 12;
const PIXELS_PER_CELL = 7;

function innerMarkup(svg) {
	const opening = svg.indexOf('>', svg.indexOf('<svg'));
	const closing = svg.lastIndexOf('</svg>');
	if (opening === -1 || closing === -1) {
		throw new Error(`${WORDMARK} is not a single well-formed <svg> element`);
	}
	// The wordmark's <title> is its accessible name. Inside the card it would be
	// a second, competing title, and the card carries its alt text in the meta
	// tag instead.
	return svg.slice(opening + 1, closing).replace(/<title>[\s\S]*?<\/title>/, '');
}

const wordmark = await readFile(WORDMARK, 'utf8');
const viewBox = VIEWBOX_RE.exec(wordmark);
if (!viewBox) {
	throw new Error(`${WORDMARK} has no "0 0 w h" viewBox to scale from`);
}

const [, rawWidth, rawHeight] = viewBox;
const scale = PIXELS_PER_CELL / WORDMARK_CELL;
const markWidth = Number(rawWidth) * scale;
const markHeight = Number(rawHeight) * scale;
const offsetX = Math.round((WIDTH - markWidth) / 2);
// Sits slightly above centre: the accent bar below carries visual weight, and
// an optically centred mark reads better than a measured one.
const offsetY = Math.round((HEIGHT - markHeight) / 2 - 28);

const card = `<svg xmlns="http://www.w3.org/2000/svg" width="${WIDTH}" height="${HEIGHT}" viewBox="0 0 ${WIDTH} ${HEIGHT}">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%" stop-color="#0B0A18"/>
      <stop offset="100%" stop-color="#171334"/>
    </linearGradient>
    <radialGradient id="glow" cx="0.5" cy="0.44" r="0.5">
      <stop offset="0%" stop-color="#6366F1" stop-opacity="0.30"/>
      <stop offset="100%" stop-color="#6366F1" stop-opacity="0"/>
    </radialGradient>
    <linearGradient id="accent" x1="0" y1="0" x2="1" y2="0">
      <stop offset="0%" stop-color="#6366F1"/>
      <stop offset="100%" stop-color="#FF7A59"/>
    </linearGradient>
  </defs>
  <rect width="${WIDTH}" height="${HEIGHT}" fill="url(#bg)"/>
  <rect width="${WIDTH}" height="${HEIGHT}" fill="url(#glow)"/>
  <g transform="translate(${offsetX} ${offsetY}) scale(${scale})" shape-rendering="crispEdges">${innerMarkup(wordmark)}</g>
  <rect x="0" y="${HEIGHT - 10}" width="${WIDTH}" height="10" fill="url(#accent)"/>
</svg>`;

// No palette quantisation: the background is a radial glow over a gradient, and
// 256 colours band it into visible rings. Full-colour deflate handles this image
// well because most of it is smooth.
const { size } = await sharp(Buffer.from(card)).png({ compressionLevel: 9 }).toFile(OUTPUT);
console.log(`wrote ${path.relative(process.cwd(), OUTPUT)} (${WIDTH}x${HEIGHT}, ${size} bytes)`);
