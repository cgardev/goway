# goway documentation site

This directory contains the documentation website for goway, built with
[Astro](https://astro.build/) and [Starlight](https://starlight.astro.build/)
using the Ion theme with a monochrome palette.

## Local development

```sh
pnpm install
pnpm dev      # serves the site at http://localhost:4321/goway
```

## Building

```sh
pnpm build    # outputs the static site to ./dist
pnpm preview  # serves the built site locally
```

## Layout

- `astro.config.mjs` — site configuration, theme, and the navigation sidebar.
- `src/content/docs/` — the documentation pages, grouped into Start Here,
  Guides, Reference, and Cookbook.
- `src/styles/theme.css` — the monochrome palette overrides for the Ion theme.
- `design.md` — the internal design notes for the library, kept alongside the
  site sources.
