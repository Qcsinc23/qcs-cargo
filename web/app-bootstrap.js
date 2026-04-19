// Auto-extracted from web/index.html
// Phase 2.4 / SEC-001a: inline <script> moved to external file so the
// CSP can drop 'unsafe-inline' (Phase 3.1).

if (!WebAssembly) {
  document.getElementById('app').innerHTML = '<p class="text-red-600">WebAssembly is not supported.</p>';
} else {
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch('/app.wasm'), go.importObject).then(r => {
    go.run(r.instance);
  }).catch(e => {
    document.getElementById('app').innerHTML = '<p class="text-slate-600">App not built yet. Run: GOOS=js GOARCH=wasm go build -o web/app.wasm ./frontend</p>';
  });
}
