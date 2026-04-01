import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';

export default defineConfig({
  plugins: [preact()],
  build: {
    lib: {
      entry: 'src/index.tsx',
      name: 'TaskUI',
      formats: ['iife'],
      fileName: () => 'taskui.js',
    },
    cssFileName: 'taskui',
    outDir: 'dist',
    minify: true,
    rollupOptions: {
      output: { inlineDynamicImports: true },
    },
  },
});
