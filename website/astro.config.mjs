// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
	site: 'https://signalridge.github.io',
	base: '/slipway',
	integrations: [
		starlight({
			disable404Route: true,
			title: 'Slipway',
			tagline: 'User-controlled soft autopilot for AI coding',
			logo: { src: './src/assets/slipway-wordmark.svg', replacesTitle: true },
			favicon: '/favicon.svg',
			customCss: ['./src/styles/custom.css'],
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/signalridge/slipway' },
			],
			editLink: { baseUrl: 'https://github.com/signalridge/slipway/edit/main/docs/' },
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
					label: 'Reference',
					translations: { 'zh-CN': '参考', ja: 'リファレンス' },
					items: [
						{ label: 'Product Authority', translations: { 'zh-CN': '产品权威', ja: '製品 authority' }, slug: 'reference/product-overview' },
						{ label: 'Issue Workflow', translations: { 'zh-CN': 'Issue 工作流', ja: 'Issue workflow' }, slug: 'reference/issue-workflow' },
						{ label: 'Commands', translations: { 'zh-CN': '命令', ja: 'コマンド' }, slug: 'reference/commands' },
						{ label: 'Machine Protocol', translations: { 'zh-CN': '机器协议', ja: 'マシンプロトコル' }, slug: 'reference/machine-protocol' },
						{ label: 'Host Adapters', translations: { 'zh-CN': '宿主适配器', ja: 'ホストアダプター' }, slug: 'reference/adapters' },
						{ label: 'Windows', translations: { 'zh-CN': 'Windows', ja: 'Windows' }, slug: 'reference/windows-rendering-and-durability' },
						{ label: 'Acceptance Evidence', translations: { 'zh-CN': '验收证据', ja: 'Acceptance evidence' }, slug: 'reference/acceptance-evidence' },
					],
				},
				{
					label: 'Explanation',
					translations: { 'zh-CN': '原理', ja: '解説' },
					items: [
						{ label: 'Architecture', translations: { 'zh-CN': '架构', ja: 'アーキテクチャ' }, slug: 'explanation/architecture' },
						{ label: 'Runs and Privacy', translations: { 'zh-CN': '运行与隐私', ja: 'run とプライバシー' }, slug: 'explanation/runs-and-privacy' },
					],
				},
				{ label: 'Contributing', translations: { 'zh-CN': '贡献', ja: 'コントリビュート' }, slug: 'contributing' },
			],
		}),
	],
});
