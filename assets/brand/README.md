# clicky logo kit

One solid core fans out into lanes: one toolkit, every surface. Start with [the usage guide](GUIDELINES.md) and [the presentation](presentation/presentation.html).

## Files

| Folder | Contents |
| --- | --- |
| `source/` | Colour masters: horizontal, stacked, symbol, small symbol and wordmark, each with a `-reversed` version; the on-dark lockup and the on-tile symbol; `build_kit.py` and `outline_text.py`, which generate them |
| `digital/` | One-colour black, white, navy and blue SVGs with transparent PNG exports, plus the square, app-icon and favicon artwork |
| `web/` | Favicon SVG and ICO (16/32/48), matching PNGs, Apple touch icon, PWA icons, `site.webmanifest`, `head-snippet.html` |
| `presentation/` | Self-contained presentation HTML, its spec and the slide PNGs |

Use `source/clicky-horizontal.svg` on light backgrounds and `source/clicky-horizontal-reversed.svg` on dark ones. Use `source/clicky-symbol-small.svg` below 32 px; the favicons are built from it.

## Web handover

Copy `web/` to the host's public root and add the markup from `web/head-snippet.html`. The snippet assumes a public-root install; for a base path, prefix its URLs. The manifest's icon paths are relative to the manifest. The docs site in `docs/site/public/` already uses this set.

## Regenerating

```bash
pip install fonttools
python3 assets/brand/source/build_kit.py path/to/Inter-Bold.ttf
```

Inter is available from <https://github.com/rsms/inter/releases> under the SIL Open Font License 1.1. Regenerate the one-colour and web exports with `export_variants.py` from the logo-design skill.

## Status

The SVG audit (`svg_audit.py` from the logo-design skill) passes; the near-miss-angle warnings come from the lanes running 1° into the core (so one-colour exports have no seam) and from Inter's glyph angles. No print proof or trademark clearance has been done.
