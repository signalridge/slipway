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
			sidebar: [
				{ label: 'Start Here', slug: 'start-here' },
				{ label: 'Real-World Scenarios', slug: 'real-world-scenarios' },
				{
					label: 'Tutorials',
					items: [
						{ label: 'First Governed Change', slug: 'tutorials/first-governed-change' },
						{ label: 'Onboarding an Existing Codebase', slug: 'tutorials/onboarding-existing-codebase' },
					],
				},
				{
					label: 'How-To',
					items: [
						{ label: 'Install & Refresh Adapters', slug: 'how-to/install-and-refresh-adapters' },
						{ label: 'Recover & Troubleshoot', slug: 'how-to/recover-and-troubleshoot' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ label: 'Commands', slug: 'reference/commands' },
						{ label: 'AI Tool Adapters', slug: 'reference/ai-tools' },
						{ label: 'Contributing', slug: 'contributing' },
					],
				},
				{
					label: 'Explanation',
					items: [
						{ label: 'Design', slug: 'explanation/design' },
						{ label: 'Workflow', slug: 'explanation/workflow' },
					],
				},
			],
		}),
	],
});
