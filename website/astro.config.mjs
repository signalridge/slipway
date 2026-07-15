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
						{ label: 'GitHub Issues', translations: { 'zh-CN': 'GitHub Issues', ja: 'GitHub Issues' }, slug: 'guides/github-issues' },
						{ label: 'Runs, Recovery, Privacy', translations: { 'zh-CN': 'Run、恢复与隐私', ja: 'Run、復旧、プライバシー' }, slug: 'guides/runs-and-recovery' },
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
