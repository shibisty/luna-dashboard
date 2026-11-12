# The page editor's bundle

GrapesJS and its plugins as the dashboard serves them at `/assets/editor/<file>`
(embedded into the binary; no Node on the servers, no CDN at runtime). Every
file is listed in `manifest.json` with its package, version and SHA-256; a test
checks the files against it, so a file changes only together with its entry.

| File | From |
|---|---|
| `grapes.min.js`, `grapes.min.css` | npm `grapesjs` (`dist/`, `dist/css/`) |
| `grapick.min.css` | npm `grapick` (`dist/`); the gradient picker of grapesjs-style-bg |
| `grapesjs-preset-webpage.js`, `grapesjs-blocks-basic.js`, `grapesjs-style-bg.js`, `grapesjs-parser-postcss.js` | npm, `dist/index.js` |
| `font-awesome.min.css`, `fontawesome-webfont.woff2`, `fontawesome-webfont.woff` | npm `font-awesome@4.7.0` (`css/`, `fonts/`): GrapesJS' icon font, which it would otherwise load from cdnjs (`cssIcons`). The CSS's `@font-face` is rewritten to these two files in the same folder (`url('fontawesome-webfont.woff2?v=4.7.0')`, then the `.woff`) |
| `grapesjs-plugin-forms.js`, `grapesjs-navbar.js`, `grapesjs-component-countdown.js`, `grapesjs-custom-code.js`, `grapesjs-tooltip.js`, `grapesjs-typed.js`, `grapesjs-style-gradient.js`, `grapesjs-style-filter.js`, `grapesjs-plugin-export.js`, `grapesjs-tui-image-editor.js` | npm, `dist/index.js` |
| `grapesjs-tabs.js` | npm `grapesjs-tabs`, `dist/grapesjs-tabs.min.js` |
| `tui-code-snippet.min.js`, `tui-color-picker.min.js`, `tui-color-picker.min.css`, `tui-image-editor.min.js`, `tui-image-editor.min.css` | npm (`dist/`): what grapesjs-tui-image-editor would load from uicdn.toast.com |
| `grapesjs-cloud-assets.js`, `dropbox-callback.html`, `microsoft-callback.html` | [grapes-cloud](https://github.com/shibisty/grapes-cloud): `npm run build`, and `public/` (the sign-in popups return there) |
| `grapesjs-export-jsx.js` | [grapesjs-react](https://github.com/shibisty/grapesjs-react): `npm run build` |
| `grapesjs-sidebar-tools.js`, `grapesjs-context-menu.js`, `grapesjs-pages.js`, `grapesjs-storage.js` | our plugins: `npm run build`, `dist/<name>.umd.cjs` |

The trailing `//# sourceMappingURL=` line is removed from each file (the maps
are not shipped).

## Updating

1. Get the new file: `npm pack <package>@<version>` and take it from the
   tarball's `package/dist`, or build the plugin (`npm ci && npm run build`).
2. Copy it here under the name above and drop its last line if it is a
   `//# sourceMappingURL=` comment.
3. Put the version and the new hash into `manifest.json`
   (`sha256sum <file>`, or `certutil -hashfile <file> SHA256` on Windows).
4. `go test ./tests/ -run Editor` and open an application's Pages.

Our plugins are tested against the pinned GrapesJS too (their suites pass on
0.21.13 and 0.23.9).
