#!/usr/bin/env bash
set -euo pipefail

test -f site/index.html
test -f site/styles.css
test -f site/.nojekyll
test -f site/assets/favicon.svg
test -f site/assets/fleet-dashboard.png
test -f site/assets/fleet-menubar.png
test -f site/assets/social-card.png
test -f site/assets/social-card.svg
test -f site/site.webmanifest
test -f site/robots.txt
test -f site/sitemap.xml

python3 -m json.tool site/site.webmanifest >/dev/null

python3 - <<'PY'
from html.parser import HTMLParser
from pathlib import Path
import struct
import re
import sys

html = Path("site/index.html").read_text()


class Parser(HTMLParser):
    def __init__(self):
        super().__init__()
        self.ids = set()
        self.hrefs = []
        self.srcs = []

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if "id" in attrs:
            self.ids.add(attrs["id"])
        if "href" in attrs:
            self.hrefs.append(attrs["href"])
        if "src" in attrs:
            self.srcs.append(attrs["src"])


parser = Parser()
parser.feed(html)

errors = []
for href in parser.hrefs:
    if href.startswith("#") and href[1:] not in parser.ids:
        errors.append(f"missing anchor target: {href}")
    if href.startswith("assets/") and not Path("site", href).exists():
        errors.append(f"missing linked asset: {href}")
    if href == "site.webmanifest" and not Path("site/site.webmanifest").exists():
        errors.append("missing linked manifest")

for src in parser.srcs:
    if not src.startswith(("http://", "https://", "//", "data:")) and not Path("site", src).exists():
        errors.append(f"missing linked source: {src}")

for match in re.findall(r'content="(assets/[^"]+)"', html):
    if not Path("site", match).exists():
        errors.append(f"missing metadata asset: {match}")

if errors:
    for error in errors:
        print(error, file=sys.stderr)
    raise SystemExit(1)


def png_size(path):
    data = Path(path).read_bytes()
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise SystemExit(f"not a png: {path}")
    width, height = struct.unpack(">II", data[16:24])
    return width, height


expected_sizes = {
    "site/assets/social-card.png": (1200, 630),
    "site/assets/fleet-menubar.png": (330, 455),
}

for path, expected in expected_sizes.items():
    actual = png_size(path)
    if actual != expected:
        raise SystemExit(f"{path} is {actual[0]}x{actual[1]}, expected {expected[0]}x{expected[1]}")

dashboard_width, dashboard_height = png_size("site/assets/fleet-dashboard.png")
if dashboard_width < 900 or dashboard_height < 350:
    raise SystemExit(
        "site/assets/fleet-dashboard.png is too small: "
        f"{dashboard_width}x{dashboard_height}"
    )
PY

echo "site validation ok"
