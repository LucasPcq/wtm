# Brand image sources

The README banner and the social preview are drawn by these scripts from the tokens of [identity.md](../../dev/identity.md), with every piece of text outlined into SVG paths so they render the same where Archivo and Martian Mono are not installed (GitHub, social cards).

They need Python 3, `fonttools`, and the two variable fonts from Google Fonts (OFL): `Archivo[wdth,wght].ttf` saved as `Archivo.ttf`, and `MartianMono[wdth,wght].ttf` saved as `MartianMono.ttf`, in one directory.

```bash
python3 -m venv /tmp/wtm-brand && /tmp/wtm-brand/bin/pip install fonttools
fonts=/tmp/wtm-fonts && mkdir -p $fonts
curl -fsSL -o $fonts/Archivo.ttf "https://github.com/google/fonts/raw/main/ofl/archivo/Archivo%5Bwdth,wght%5D.ttf"
curl -fsSL -o $fonts/MartianMono.ttf "https://github.com/google/fonts/raw/main/ofl/martianmono/MartianMono%5Bwdth,wght%5D.ttf"

cd docs/assets/src
/tmp/wtm-brand/bin/python banner.py $fonts ..                                  # banner-dark.svg, banner-light.svg
/tmp/wtm-brand/bin/python social.py $fonts /tmp/social-preview.svg
rsvg-convert -w 1280 -h 640 /tmp/social-preview.svg -o ../../../site/public/social-preview.png
```

The social preview is also uploaded by hand in the repository settings (Settings → General → Social preview): GitHub has no API for it.
