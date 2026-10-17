# The social preview, 1280×640: an SVG rendered to site/public/social-preview.png.
import sys

from outline import text_path

fonts_dir, out = sys.argv[1], sys.argv[2]
archivo = f"{fonts_dir}/Archivo.ttf"
mono = f"{fonts_dir}/MartianMono.ttf"

bg, strong, muted, sig = "#14121B", "#F4F1FA", "#9D97B2", "#E8734A"
lanes = ("#E8734A", "#9B8CFF", "#5FD4C0")

wordmark = text_path(archivo, "wtm", 150, 141, 46, 850, 112, -1)
line1 = text_path(archivo, "Every branch,", 90, 318, 112, 300, 70, -2)
line2 = text_path(archivo, "running.", 88, 436, 124, 850, 118, -3)
sub = text_path(archivo, "A git worktree manager for you and your coding agents.", 92, 520, 27, 400, 100)
urls = [text_path(mono, u, 900, y, 15, 400, 100) for u, y in (("web.feat-login.acme.localhost", 186), ("web.agent-checkout.acme.localhost", 306), ("web.agent-search.acme.localhost", 426))]

mark = f'''<g transform="translate(92 105) scale(1.7)"><circle cx="4" cy="12" r="3" fill="{strong}"/><path d="M6 12 C14 12 14 4 22 4 H27" fill="none" stroke="{lanes[0]}" stroke-width="3" stroke-linecap="round"/><path d="M6 12 H27" fill="none" stroke="{lanes[1]}" stroke-width="3" stroke-linecap="round"/><path d="M6 12 C14 12 14 20 22 20 H27" fill="none" stroke="{lanes[2]}" stroke-width="3" stroke-linecap="round"/></g>'''

svg = f'''<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="640" viewBox="0 0 1280 640">
<rect width="1280" height="640" fill="{bg}"/>
{mark}
<path fill="{sig}" d="{wordmark}"/>
<path fill="{strong}" d="{line1}"/>
<path fill="{sig}" d="{line2}"/>
<path fill="{muted}" d="{sub}"/>
<circle cx="760" cy="320" r="10" fill="{strong}"/>
<path d="M768 320 C840 320 850 200 930 200 H1290" fill="none" stroke="{lanes[0]}" stroke-width="10" stroke-linecap="round"/>
<path d="M768 320 H1290" fill="none" stroke="{lanes[1]}" stroke-width="10" stroke-linecap="round"/>
<path d="M768 320 C840 320 850 440 930 440 H1290" fill="none" stroke="{lanes[2]}" stroke-width="10" stroke-linecap="round"/>
<path fill="{muted}" d="{" ".join(urls)}"/>
</svg>
'''
with open(out, "w") as f:
    f.write(svg)
