// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { ion } from 'starlight-ion-theme';

// https://astro.build/config
export default defineConfig({
	// The site is published as a GitHub Pages project site at
	// https://cgardev.github.io/goway/, so the origin and the base path
	// are configured separately.
	site: 'https://cgardev.github.io',
	base: '/goway',
	integrations: [
		starlight({
			title: 'goway',
			description:
				'A version-based database schema migration library for Go, modeled on Flyway, for PostgreSQL and SQLite.',
			// Apply the Ion theme, recolored with the monochrome
			// palette defined in ./src/styles/theme.css. useCustomECTheme:false lets
			// code blocks follow the same palette instead of Ion's built-in theme.
			plugins: [ion({ useCustomECTheme: false })],
			customCss: ['./src/styles/theme.css'],
			// The header logo and the browser tab icon both use the SQL file
			// artwork. The logo is resolved relative to the project root, while the
			// favicon is served from the public directory.
			logo: {
				src: './src/assets/sql-file.png',
				alt: 'goway',
				replacesTitle: false,
			},
			favicon: '/sql-file.png',
			social: [
				{
					icon: 'github',
					label: 'GitHub',
					href: 'https://github.com/cgardev/goway',
				},
			],
			// Surface an "Edit page" link pointing at the documentation sources
			// within the repository.
			editLink: {
				baseUrl: 'https://github.com/cgardev/goway/edit/main/docs/',
			},
			sidebar: [
				{
					label: 'Start Here',
					items: [
						{ label: 'Introduction', link: '/' },
						{ label: 'Getting Started', slug: 'getting-started' },
					],
				},
				{
					label: 'Guides',
					items: [
						{ label: 'Writing Migrations', slug: 'guides/writing-migrations' },
						{ label: 'Running Migrations', slug: 'guides/running-migrations' },
						{ label: 'Configuration', slug: 'guides/configuration' },
						{ label: 'Callbacks', slug: 'guides/callbacks' },
						{ label: 'Dialects', slug: 'guides/dialects' },
						{ label: 'Command Line', slug: 'guides/command-line' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ label: 'Overview', slug: 'reference' },
						{ label: 'Configuration', slug: 'reference/configuration' },
						{ label: 'Commands', slug: 'reference/commands' },
						{ label: 'Migration Naming', slug: 'reference/migration-naming' },
						{ label: 'Schema History', slug: 'reference/schema-history' },
						{ label: 'Placeholders', slug: 'reference/placeholders' },
						{ label: 'Callbacks', slug: 'reference/callbacks' },
						{ label: 'Dialects', slug: 'reference/dialects' },
						{ label: 'Command Line', slug: 'reference/cli' },
					],
				},
				{
					label: 'Cookbook',
					items: [
						{ label: 'Overview', slug: 'cookbook' },
						{
							label: 'Embedding Migrations',
							slug: 'cookbook/embedding-migrations',
						},
						{
							label: 'Programmatic Setup',
							slug: 'cookbook/programmatic-setup',
						},
						{
							label: 'Non-Transactional Migrations',
							slug: 'cookbook/non-transactional-migrations',
						},
						{
							label: 'Baselining an Existing Database',
							slug: 'cookbook/baseline-existing-database',
						},
					],
				},
			],
		}),
	],
});
