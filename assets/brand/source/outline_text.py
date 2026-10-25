"""Outline a word into one SVG path: python outline_text.py FONT TEXT SIZE X BASELINE TRACKING.

SIZE is the font size in SVG units, TRACKING is extra space between glyphs in SVG units (negative tightens).
Prints the path data and the word's right edge on two lines.
"""
import sys

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont


def outline(font_path, text, size, x, baseline, tracking):
    font = TTFont(font_path)
    glyph_set = font.getGlyphSet()
    cmap = font.getBestCmap()
    scale = size / font["head"].unitsPerEm
    pen = SVGPathPen(glyph_set, ntos=lambda value: f"{value:.2f}".rstrip("0").rstrip("."))
    cursor = x
    for index, char in enumerate(text):
        if ord(char) not in cmap:
            raise SystemExit(f"{font_path} has no glyph for {char!r}")
        name = cmap[ord(char)]
        glyph_set[name].draw(TransformPen(pen, (scale, 0, 0, -scale, cursor, baseline)))
        cursor += glyph_set[name].width * scale
        if index < len(text) - 1:
            cursor += tracking
    return pen.getCommands(), cursor


if __name__ == "__main__":
    font_path, text, size, x, baseline, tracking = sys.argv[1:7]
    path, right = outline(font_path, text, float(size), float(x), float(baseline), float(tracking))
    print(path)
    print(f"{right:.2f}")
