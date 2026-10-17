#!/bin/sh
# Screenshot a page of the site the way a reviewer sees it.
# Usage: shoot.sh <url> <out.png> [width=1440] [height=900] [css-selector | full]
#   a selector captures that element (e.g. 'section.section:nth-of-type(5)'), "full" the whole page.
# Prints the page's scrollWidth and fails when the page scrolls sideways.
# Motion is reduced, so replayed terminals and the hero show their final frame.
set -e
TOOLS="${TMPDIR:-/tmp}/wtm-site-tools"
if [ ! -d "$TOOLS/node_modules/puppeteer-core" ]; then
  mkdir -p "$TOOLS" && (cd "$TOOLS" && npm init -y >/dev/null && npm install puppeteer-core@23 >/dev/null 2>&1)
fi
NODE_PATH="$TOOLS/node_modules" node "$(dirname "$0")/shoot.cjs" "$@"
