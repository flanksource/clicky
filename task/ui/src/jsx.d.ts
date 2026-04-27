import type { JSX } from 'preact';

declare module 'preact' {
  namespace JSX {
    interface IntrinsicElements {
      'iconify-icon': JSX.HTMLAttributes<HTMLElement> & {
        icon?: string;
        width?: string | number;
        height?: string | number;
        flip?: string;
        rotate?: string | number;
        inline?: boolean;
        mode?: 'svg' | 'style' | 'bg' | 'mask';
      };
    }
  }
}
