# Embedded material UI

These resources are a Go-owned snapshot of the existing Flask material UI, taken
for migration tasks 5.6–5.7. The original `app/` files are not modified.

- `static/styles.css`, `logo.png`, `api.js`, `login.js`: byte-for-byte copies of
  `app/static/`. `TestMaterialPagesAndStaticBoundary` checks that relationship.
- `templates/login.html` and `materials.html`: flatten the existing Jinja
  `base.html`/child blocks and replace `url_for('static', ...)` with identical
  `/static/...` paths. No user data is rendered into these public page shells.
- The Go materials template omits the `/knowledge` link; `materials.js` omits
  only retrieval `index_status` messages. Knowledge/RAG is outside this change.
- Go uses `html/template` and `go:embed` with an explicit asset allowlist. No
  filesystem directory server is mounted and no Python runtime is needed.

Details are still read into the existing `<pre id="text">` on `/materials` using
`textContent`; downloads still use authenticated fetch, Blob URLs and revocation.
Future changes to the Flask UI need an explicit review of this snapshot; do not
blindly copy unrelated retrieval features into the material-only Go UI.
