import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import remarkBaseLinks from './src/plugins/remark-base-links.mjs';

// Published as a GitHub Pages project site: https://flanksource.github.io/clicky/
// The Pages workflow sets both from the repository's Pages settings; a custom
// domain reports an empty base path, which means the site root.
const site = process.env.DOCS_SITE || 'https://flanksource.github.io';
const base = process.env.DOCS_BASE === undefined ? '/clicky' : process.env.DOCS_BASE || '/';
// Starlight prefixes `favicon` with the base but leaves `head` hrefs as written.
const publicUrl = (file) => `${base.replace(/\/$/, '')}/${file}`;

export default defineConfig({
  site,
  base,
  markdown: { remarkPlugins: [[remarkBaseLinks, { base }]] },
  integrations: [
    starlight({
      title: 'clicky',
      description: 'Turn Go structs into a CLI, a REST/OpenAPI API, a web UI and MCP tools from one registration.',
      logo: {
        light: './src/assets/clicky-horizontal.svg',
        dark: './src/assets/clicky-horizontal-reversed.svg',
        replacesTitle: true,
      },
      favicon: '/favicon.svg',
      head: [
        { tag: 'link', attrs: { rel: 'icon', href: publicUrl('favicon.ico'), sizes: '48x48' } },
        { tag: 'link', attrs: { rel: 'apple-touch-icon', href: publicUrl('apple-touch-icon.png') } },
        { tag: 'link', attrs: { rel: 'manifest', href: publicUrl('site.webmanifest') } },
        { tag: 'meta', attrs: { name: 'theme-color', content: '#131f3b' } },
      ],
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/flanksource/clicky' }],
      editLink: { baseUrl: 'https://github.com/flanksource/clicky/edit/main/docs/site/' },
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        {
          label: 'Start here',
          items: ['index', 'getting-started', 'concepts'],
        },
        {
          label: 'Pretty printing',
          items: [
            'pretty/overview',
            'pretty/text',
            'pretty/structs',
            'pretty/tables',
            'pretty/llm',
            'pretty/trees',
            'pretty/components',
            'pretty/guidelines',
          ],
        },
        {
          label: 'Entities',
          items: [
            'entities/overview',
            'entities/list-options',
            'entities/crud',
            'entities/sorting-and-paging',
            'entities/long-results',
            'entities/actions',
            'entities/bulk-actions',
            'entities/hierarchy',
            'entities/commands',
          ],
        },
        {
          label: 'Filters & lookups',
          items: [
            'filters/overview',
            'filters/control-types',
            'filters/lookups',
            'filters/searchable',
            'filters/named-filters',
          ],
        },
        {
          label: 'Runtime',
          items: ['runtime/context', 'runtime/errors', 'runtime/operation-listeners', 'runtime/ai-tools'],
        },
        {
          label: 'Dynamic entities',
          items: ['dynamic/schema-entities', 'dynamic/families'],
        },
        {
          label: 'Reference',
          items: ['reference/http-routes', 'reference/x-clicky','reference/struct-tags', 'reference/api-index'],
        },
      ],
    }),
  ],
});
