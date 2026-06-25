// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// Content is synced from the repo's `docs/**` (source of truth) into
// `src/content/docs/` by `scripts/sync-docs.mjs` before every dev/build.
// https://astro.build/config
export default defineConfig({
	site: 'https://signalridge.github.io',
	base: '/slipway',
	integrations: [
		starlight({
			title: 'Slipway',
			tagline: 'Governance CLI for AI-assisted software delivery',
			logo: { src: './src/assets/slipway-wordmark.svg', replacesTitle: true },
			favicon: '/favicon.svg',
			customCss: ['./src/styles/custom.css'],
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/signalridge/slipway' },
			],
			editLink: { baseUrl: 'https://github.com/signalridge/slipway/edit/main/docs/' },
			lastUpdated: true,
			defaultLocale: 'root',
			locales: {
				root: { label: 'English', lang: 'en' },
				zh: { label: '简体中文', lang: 'zh-CN' },
				ja: { label: '日本語', lang: 'ja' },
			},
			sidebar: [
				{
					label: 'Start Here',
					translations: { 'zh-CN': '从这里开始', ja: 'はじめに' },
					slug: 'start-here',
				},
				{
					label: 'Real-World Scenarios',
					translations: { 'zh-CN': '实战场景', ja: '実践シナリオ' },
					slug: 'real-world-scenarios',
				},
				{
					label: 'Tutorials',
					translations: { 'zh-CN': '教程', ja: 'チュートリアル' },
					items: [
						{
							label: 'First Governed Change',
							translations: { 'zh-CN': '第一个受管变更', ja: '初めてのガバナンス変更' },
							slug: 'tutorials/first-governed-change',
						},
						{
							label: 'Onboarding an Existing Codebase',
							translations: { 'zh-CN': '接入既有代码库', ja: '既存コードベースの導入' },
							slug: 'tutorials/onboarding-existing-codebase',
						},
					],
				},
				{
					label: 'How-To',
					translations: { 'zh-CN': '操作指南', ja: 'ハウツー' },
					items: [
						{
							label: 'Install & Refresh Adapters',
							translations: { 'zh-CN': '安装与刷新适配器', ja: 'アダプターの導入と更新' },
							slug: 'how-to/install-and-refresh-adapters',
						},
						{
							label: 'Recover & Troubleshoot',
							translations: { 'zh-CN': '恢复与排错', ja: '復旧とトラブルシューティング' },
							slug: 'how-to/recover-and-troubleshoot',
						},
					],
				},
				{
					label: 'Reference',
					translations: { 'zh-CN': '参考', ja: 'リファレンス' },
					items: [
						{
							label: 'Commands',
							translations: { 'zh-CN': '命令', ja: 'コマンド' },
							slug: 'reference/commands',
						},
						{
							label: 'AI Tool Adapters',
							translations: { 'zh-CN': 'AI 工具适配器', ja: 'AI ツールアダプター' },
							slug: 'reference/ai-tools',
						},
						{
							label: 'Contributing',
							translations: { 'zh-CN': '贡献指南', ja: 'コントリビュート' },
							slug: 'contributing',
						},
					],
				},
				{
					label: 'Explanation',
					translations: { 'zh-CN': '原理', ja: '解説' },
					items: [
						{
							label: 'Design',
							translations: { 'zh-CN': '设计', ja: '設計' },
							slug: 'explanation/design',
						},
						{
							label: 'Design Philosophy (deep dive)',
							translations: { 'zh-CN': '设计哲学（深入）', ja: '設計思想（詳説）' },
							slug: 'design',
						},
						{
							label: 'Workflow',
							translations: { 'zh-CN': '工作流', ja: 'ワークフロー' },
							slug: 'explanation/workflow',
						},
						{
							label: 'Governed Workflow (deep dive)',
							translations: { 'zh-CN': '受管工作流（深入）', ja: 'ガバナンスワークフロー（詳説）' },
							slug: 'workflow',
						},
					],
				},
			],
		}),
	],
});
