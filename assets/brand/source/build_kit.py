"""Write the clicky masters (symbols, small cuts, lockups, wordmark) next to this script.

Needs fontTools and Inter Bold (SIL OFL 1.1, https://github.com/rsms/inter/releases):
    python3 build_kit.py path/to/Inter-Bold.ttf
"""
import sys
from pathlib import Path

from outline_text import outline

HERE = Path(__file__).parent
FONT = Path(sys.argv[1]) if len(sys.argv) > 1 else None

NAVY, BLUE, SKY, WHITE = "#131f3b", "#3578e5", "#0ea5e9", "#ffffff"
LIGHT = {"body": NAVY, "accent": BLUE, "word": NAVY}
DARK = {"body": WHITE, "accent": SKY, "word": WHITE}

# Each piece is (role, path data) on the 256 grid; role picks the colour.
BRANDS = {
    "clicky": {
        "title": "clicky",
        # The core spans exactly 120°-240°; each lane runs 1° into it (to 121° / 239°) and is drawn on top, so one-colour
        # exports have no anti-aliasing seam and the colour boundary steps by under 2 units at the gap ends.
        "symbol": [
            ("body", "M88 207.67A92 92 0 0 1 88 48.33L115 95.09A38 38 0 0 0 115 160.91Z"),
            ("accent", "M199.05 193.05A92 92 0 0 1 86.62 206.86L93.83 194.86A78 78 0 0 0 189.15 183.15Z"),
            ("accent", "M184.91 178.91A72 72 0 0 1 96.92 189.72L104.13 177.72A58 58 0 0 0 175.01 169.01Z"),
            ("accent", "M170.77 164.77A52 52 0 0 1 107.22 172.57L114.43 160.57A38 38 0 0 0 160.87 154.87Z"),
            ("accent", "M86.62 49.14A92 92 0 0 1 199.05 62.95L189.15 72.85A78 78 0 0 0 93.83 61.14Z"),
            ("accent", "M96.92 66.28A72 72 0 0 1 184.91 77.09L175.01 86.99A58 58 0 0 0 104.13 78.28Z"),
            ("accent", "M107.22 83.43A52 52 0 0 1 170.77 91.23L160.87 101.13A38 38 0 0 0 114.43 95.43Z"),
        ],
        # Two 19-unit lanes and one 16-unit gap per arm instead of three 14/6 lanes.
        "small": [
            ("body", "M88 207.67A92 92 0 0 1 88 48.33L115 95.09A38 38 0 0 0 115 160.91Z"),
            ("accent", "M199.05 193.05A92 92 0 0 1 86.62 206.86L96.4 190.58A73 73 0 0 0 185.62 179.62Z"),
            ("accent", "M174.3 168.3A57 57 0 0 1 104.64 176.86L114.43 160.57A38 38 0 0 0 160.87 154.87Z"),
            ("accent", "M86.62 49.14A92 92 0 0 1 199.05 62.95L185.62 76.38A73 73 0 0 0 96.4 65.42Z"),
            ("accent", "M104.64 79.14A57 57 0 0 1 174.3 87.7L160.87 101.13A38 38 0 0 0 114.43 95.43Z"),
        ],
        "symbol_right": 200,
    },
}

WORD_SIZE, WORD_BASELINE, WORD_TRACKING, WORD_GAP = 120, 172, -3, 40
STACK_SIZE, STACK_BASELINE = 96, 336


def paths(pieces, palette, dx=0.0, dy=0.0):
    group = "".join(f'<path fill="{palette[role]}" d="{d}"/>' for role, d in pieces)
    return f'<g transform="translate({dx:g} {dy:g})">{group}</g>' if dx or dy else group


def svg(view_box, title, body, background=None):
    bg = f'<rect width="100%" height="100%" fill="{background}"/>' if background else ""
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{view_box}">'
            f"<title>{title} logo</title>{bg}{body}</svg>\n")


def write(brand, name, text):
    (HERE / name).write_text(text)


def build(brand, spec):
    title = spec["title"]
    for variant, palette, suffix in (("symbol", LIGHT, ""), ("symbol", DARK, "-reversed"),
                                     ("small", LIGHT, ""), ("small", DARK, "-reversed")):
        name = f"{brand}-{'symbol' if variant == 'symbol' else 'symbol-small'}{suffix}.svg"
        write(brand, name, svg("0 0 256 256", title, paths(spec[variant], palette)))

    word_x = spec["symbol_right"] + WORD_GAP
    word, right = outline(str(FONT), title, WORD_SIZE, word_x, WORD_BASELINE, WORD_TRACKING)
    width = round(right + 36)
    for palette, suffix in ((LIGHT, ""), (DARK, "-reversed")):
        body = paths(spec["symbol"], palette) + f'<path fill="{palette["word"]}" d="{word}"/>'
        write(brand, f"{brand}-horizontal{suffix}.svg", svg(f"0 0 {width} 256", title, body))

    probe, probe_right = outline(str(FONT), title, STACK_SIZE, 0, STACK_BASELINE, WORD_TRACKING)
    stack_width = max(256, round(probe_right + 64))
    stacked_word, _ = outline(str(FONT), title, STACK_SIZE, (stack_width - probe_right) / 2, STACK_BASELINE, WORD_TRACKING)
    for palette, suffix in ((LIGHT, ""), (DARK, "-reversed")):
        body = paths(spec["symbol"], palette, dx=(stack_width - 256) / 2) + f'<path fill="{palette["word"]}" d="{stacked_word}"/>'
        write(brand, f"{brand}-stacked{suffix}.svg", svg(f"0 0 {stack_width} 372", title, body))

    wordmark, word_right = outline(str(FONT), title, WORD_SIZE, 0, WORD_BASELINE, WORD_TRACKING)
    write(brand, f"{brand}-wordmark.svg", svg(f"0 60 {round(word_right)} 140", title, f'<path fill="{NAVY}" d="{wordmark}"/>'))

    for name, background in ((f"{brand}-horizontal-on-dark.svg", NAVY),):
        body = paths(spec["symbol"], DARK) + f'<path fill="{WHITE}" d="{word}"/>'
        write(brand, name, svg(f"0 0 {width} 256", title, body, background))
    write(brand, f"{brand}-symbol-on-tile.svg",
          svg("0 0 256 256", title, f'<rect width="256" height="256" rx="56" fill="{NAVY}"/>'
              + paths(spec["symbol"], DARK, dx=0, dy=0).replace("<path", "<path transform=\"translate(25.6 25.6) scale(0.8)\"")))


if __name__ == "__main__":
    if FONT is None or not FONT.is_file():
        raise SystemExit("usage: build_kit.py path/to/Inter-Bold.ttf")
    for brand, spec in BRANDS.items():
        build(brand, spec)
        print(f"wrote {brand} masters to {HERE}")
