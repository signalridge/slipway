// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import sitemap from '@astrojs/sitemap';

const BASE = '/slipway';
const SITE = 'https://signalridge.github.io';

// Absolute, because every consumer of these is off-site: crawlers and social
// unfurlers do not resolve relative URLs.
const OG_IMAGE = `${SITE}${BASE}/og.png`;

const head = [
	// Starlight already emits `twitter:card: summary_large_image`, which reserves
	// a 1.91:1 image slot on every unfurl. Nothing filled it. `public/og.png` is
	// generated from the wordmark by `scripts/build-og-image.mjs`.
	{ tag: 'meta', attrs: { property: 'og:image', content: OG_IMAGE } },
	{ tag: 'meta', attrs: { property: 'og:image:width', content: '1200' } },
	{ tag: 'meta', attrs: { property: 'og:image:height', content: '630' } },
	{ tag: 'meta', attrs: { property: 'og:image:alt', content: 'Slipway' } },
	{ tag: 'meta', attrs: { name: 'twitter:image', content: OG_IMAGE } },
	// `crossorigin` is not optional even though this is same-origin: fonts fetch
	// in CORS mode, and a preload without it is a wasted second request rather
	// than a warmed cache. Only the latin subset is preloaded — latin-ext is
	// needed rarely enough that the browser should decide.
	{
		tag: 'link',
		attrs: {
			rel: 'preload',
			href: `${BASE}/fonts/space-grotesk-600-latin.woff2`,
			as: 'font',
			type: 'font/woff2',
			crossorigin: 'anonymous',
		},
	},
	// Declared here rather than in `custom.css` so Vite never tries to rewrite
	// these `public/` URLs (it warns on each one it cannot resolve at build
	// time), and so the face is known while the HTML is still parsing instead of
	// after the stylesheet arrives.
	//
	// Only weight 600 is vendored: the headings, the hero, and the sidebar group
	// labels all resolve to it, and nothing on the site uses another weight. The
	// subsets and their unicode-ranges are Google's own, kept so latin-ext is
	// fetched only by pages that need it. SIL OFL 1.1, see public/fonts/OFL.txt.
	{
		tag: 'style',
		content: [
			'@font-face{font-family:"Space Grotesk";font-style:normal;font-weight:600;',
			'font-display:swap;',
			`src:url("${BASE}/fonts/space-grotesk-600-latin.woff2") format("woff2");`,
			'unicode-range:U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,',
			'U+02DC,U+0304,U+0308,U+0329,U+2000-206F,U+20AC,U+2122,U+2191,U+2193,',
			'U+2212,U+2215,U+FEFF,U+FFFD}',
			'@font-face{font-family:"Space Grotesk";font-style:normal;font-weight:600;',
			'font-display:swap;',
			`src:url("${BASE}/fonts/space-grotesk-600-latin-ext.woff2") format("woff2");`,
			'unicode-range:U+0100-02BA,U+02BD-02C5,U+02C7-02CC,U+02CE-02D7,U+02DD-02FF,',
			'U+0304,U+0308,U+0329,U+1D00-1DBF,U+1E00-1E9F,U+1EF2-1EFF,U+2020,',
			'U+20A0-20AB,U+20AD-20C0,U+2113,U+2C60-2C7F,U+A720-A7FF}',
		].join(''),
	},
];
const legacyEnglishRedirects = {
	'/': `${BASE}/en/`,
	'/start-here': `${BASE}/en/start-here/`,
	'/installation': `${BASE}/en/installation/`,
	'/guides/idea-to-run-workflow': `${BASE}/en/guides/idea-to-run-workflow/`,
	'/guides/github-issues': `${BASE}/en/guides/github-issues/`,
	'/guides/runs-and-recovery': `${BASE}/en/guides/runs-and-recovery/`,
	'/guides/machine-protocol-v2': `${BASE}/en/guides/machine-protocol-v2/`,
	'/reference/commands': `${BASE}/en/reference/commands/`,
	'/reference/machine-protocol': `${BASE}/en/reference/machine-protocol/`,
	'/reference/adapters': `${BASE}/en/reference/adapters/`,
	'/explanation/concepts': `${BASE}/en/explanation/concepts/`,
	'/explanation/architecture': `${BASE}/en/explanation/architecture/`,
	'/contributing': `${BASE}/en/contributing/`,
};

export default defineConfig({
	site: SITE,
	base: BASE,
	redirects: legacyEnglishRedirects,
	integrations: [
		sitemap({
			filter: (page) => {
				const { pathname } = new URL(page);
				return pathname !== BASE && pathname !== `${BASE}/`;
			},
			i18n: {
				defaultLocale: 'en',
				locales: { en: 'en', zh: 'zh-CN', ja: 'ja' },
			},
		}),
		starlight({
			disable404Route: true,
			title: 'Slipway',
			tagline: 'User-controlled soft autopilot for AI coding',
			logo: { src: './src/assets/slipway-wordmark.svg', replacesTitle: true },
			favicon: '/favicon.svg',
			customCss: ['./src/styles/custom.css'],
			head,
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/signalridge/slipway' },
			],
			lastUpdated: true,
			defaultLocale: 'en',
			locales: {
				en: { label: 'English', lang: 'en' },
				zh: { label: '简体中文', lang: 'zh-CN' },
				ja: { label: '日本語', lang: 'ja' },
			},
			sidebar: [
				{ label: 'Start Here', translations: { 'zh-CN': '从这里开始', ja: 'はじめに' }, slug: 'start-here' },
				{ label: 'Installation', translations: { 'zh-CN': '安装', ja: 'インストール' }, slug: 'installation' },
				{
					label: 'Guides',
					translations: { 'zh-CN': '指南', ja: 'ガイド' },
					items: [
						{ label: 'Idea-to-Run Workflow', translations: { 'zh-CN': '从想法到 Run', ja: 'アイデアから Run まで' }, slug: 'guides/idea-to-run-workflow' },
						{ label: 'GitHub Issues', translations: { 'zh-CN': 'GitHub Issues', ja: 'GitHub Issues' }, slug: 'guides/github-issues' },
						{ label: 'Runs, Recovery, Privacy', translations: { 'zh-CN': 'Run、恢复与隐私', ja: 'Run、復旧、プライバシー' }, slug: 'guides/runs-and-recovery' },
						{ label: 'Protocol v2 Tutorial', translations: { 'zh-CN': '协议 v2 教程', ja: 'プロトコル v2 チュートリアル' }, slug: 'guides/machine-protocol-v2' },
					],
				},
				{
					label: 'Reference',
					translations: { 'zh-CN': '参考', ja: 'リファレンス' },
					items: [
						{ label: 'Commands', translations: { 'zh-CN': '命令', ja: 'コマンド' }, slug: 'reference/commands' },
						{ label: 'Machine Protocol', translations: { 'zh-CN': '机器协议', ja: 'マシンプロトコル' }, slug: 'reference/machine-protocol' },
						{ label: 'Host Adapters', translations: { 'zh-CN': '宿主适配器', ja: 'ホストアダプター' }, slug: 'reference/adapters' },
					],
				},
				{
					label: 'Explanation',
					translations: { 'zh-CN': '原理', ja: '解説' },
					items: [
						{ label: 'Core Concepts', translations: { 'zh-CN': '核心概念', ja: 'コア概念' }, slug: 'explanation/concepts' },
						{ label: 'Architecture', translations: { 'zh-CN': '架构', ja: 'アーキテクチャ' }, slug: 'explanation/architecture' },
					],
				},
				{ label: 'Contributing', translations: { 'zh-CN': '贡献', ja: 'コントリビュート' }, slug: 'contributing' },
			],
		}),
	],
});
