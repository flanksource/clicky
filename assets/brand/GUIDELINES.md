# clicky: logo guidelines

## 1. The logo
- **Idea:** one solid core at the back of the c fans out into lanes towards both ends: one toolkit, every surface (CLI, API, JSON, HTML, MCP).
- **Versions:** horizontal (primary) `source/clicky-horizontal.svg` · stacked `source/clicky-stacked.svg` · symbol `source/clicky-symbol.svg` · wordmark `source/clicky-wordmark.svg`. Each has a `-reversed` version for dark backgrounds.
- **Small sizes:** below 32 px use `source/clicky-symbol-small.svg`, which keeps two 19-unit lanes and one 16-unit gap per arm in place of three 14/6 lanes.
- **Files:** `digital/` holds one-colour black, white and navy versions with PNG exports; `web/` holds the favicon set (`favicon.svg`, `favicon.ico`, 16/32/48 px PNGs), Apple and PWA icons, `site.webmanifest` and `head-snippet.html`.
- **Family:** the lanes use UIR's 14-unit lanes and 6-unit gaps; UIR is many lanes fusing into one, clicky is one core fanning out into many.

## 2. Clear space
Keep a clear zone of **1 × the ring width** (54 units on the 256 grid, 29 % of the symbol's height) on every side. The zone scales with the logo.

## 3. Minimum size
| Version | Screen | Print |
|---|---|---|
| Horizontal | 96 px wide | 25 mm wide |
| Symbol | 16 px (small cut) | 6 mm |

## 4. Colour
| Name | HEX | RGB | CMYK | Pantone | Use |
|---|---|---|---|---|---|
| Dark Navy | #131f3b | 19 31 59 | 81 68 51 60 | 533 C | The core and the wordmark |
| Flanksource Blue | #3578e5 | 53 120 229 | 79 53 0 0 | 2727 C | The lanes |
| Sky Blue | #0ea5e9 | 14 165 233 | 72 12 0 0 | 2985 C | The lanes on dark backgrounds |

**Approved pairs:** full colour on white or light grey · reversed (white core, sky lanes) on navy or black · one-colour navy, black or white when only one ink is available.

## 5. Typography
- Wordmark: Inter Bold, tracked −3 units at 120, outlined. Never retype it.
- Inter is licensed under the SIL Open Font License 1.1, which permits use in logos.

## 6. Don'ts
Don't change the number of lanes or close the gaps · don't swap the core and lane colours · don't stretch, rotate or add effects · don't rearrange the lockup or change the symbol/wordmark ratio · don't use the full-size symbol below 32 px.

## 7. Masters
`source/build_kit.py` holds the geometry and colours and writes every SVG in `source/`. The one-colour and web exports in `digital/` and `web/` come from the logo-design skill's `export_variants.py`. Edit `build_kit.py` and regenerate rather than editing exported files.
