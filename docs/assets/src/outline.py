# Turns text set in a variable font into SVG path data, so the banner and the
# social preview render the same without the font installed (GitHub, social cards).
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont

_cache = {}


def instance(path, wght, wdth):
    key = (path, wght, wdth)
    if key not in _cache:
        _cache[key] = instantiateVariableFont(TTFont(path), {"wght": wght, "wdth": wdth})
    return _cache[key]


def text_path(path, text, x, y, size, wght, wdth, tracking=0.0):
    font = instance(path, wght, wdth)
    glyphs = font.getGlyphSet()
    cmap = font.getBestCmap()
    scale = size / font["head"].unitsPerEm
    pen = SVGPathPen(glyphs)
    cursor = x
    for ch in text:
        name = cmap[ord(ch)]
        glyphs[name].draw(TransformPen(pen, (scale, 0, 0, -scale, cursor, y)))
        cursor += glyphs[name].width * scale + tracking
    return pen.getCommands()
