# Fleet GitHub Pages site

This directory contains the static marketing site published by the `Pages`
workflow.

The site is intentionally dependency-free:

- `index.html` is the page content.
- `styles.css` is the full visual system.
- `assets/` contains the favicon, social cards, screenshots, and demo media.
- `site.webmanifest`, `robots.txt`, and `sitemap.xml` provide basic browser and
  crawler metadata.

GitHub Pages deploys this directory when changes land on `main`. During branch
work, open `site/index.html` directly in a browser or serve the directory with a
local static server.

Run the static checks with:

```sh
scripts/validate-site.sh
```

The validator checks required files, internal anchors, linked static assets,
manifest JSON, and expected screenshot/social-card image dimensions.

## Asset checklist

Before launch, replace or supplement the starter SVG social card with captured
product media:

- `assets/fleet-dashboard.png` - terminal dashboard screenshot. Captured from
  the live CLI on April 30, 2026.
- `assets/fleet-menubar.png` - native menu bar popover screenshot. Captured from
  the live app on April 30, 2026.
- `assets/demo-loop.gif` - short launch/status/tunnel workflow.
- `assets/social-card.png` - 1200x630 Open Graph image for broad social support.
  A starter card is present; replace it if captured product media becomes the
  stronger launch asset.

## Pre-merge checklist

- Open `site/index.html` locally at desktop and mobile widths.
- Confirm the GitHub Pages source is set to GitHub Actions in repository
  settings before merging.
- Confirm release assets exist before public promotion links point at
  `/releases/latest`.
- Replace starter mock visuals with real screenshots when available.
