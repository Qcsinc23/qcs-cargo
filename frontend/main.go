//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"
)

var copy = map[string]map[string]string{
	"en": {
		"title":         "QCS Cargo",
		"subtitle":      "Parcel Forwarding · Air Freight · Warehouse Operations",
		"dashboard":     "Dashboard",
		"tracking":      "Tracking",
		"notifications": "Notifications",
		"summary":       "A routed WASM shell is active. The production dashboard pages keep the primary workflow, while this shell provides a lightweight app surface for PWA navigation and stateful controls.",
		"cta":           "Open Dashboard",
		"empty":         "Nothing to show yet.",
	},
	"es": {
		"title":         "QCS Cargo",
		"subtitle":      "Casillero · Carga Aerea · Operaciones de Almacen",
		"dashboard":     "Panel",
		"tracking":      "Rastreo",
		"notifications": "Notificaciones",
		"summary":       "La interfaz WASM con rutas ya esta activa. Las paginas del tablero siguen siendo el flujo principal, mientras este shell ofrece una superficie ligera para PWA y controles con estado.",
		"cta":           "Abrir Panel",
		"empty":         "Todavia no hay datos.",
	},
}

func main() {
	doc := js.Global().Get("document")
	root := doc.Call("getElementById", "app")
	if root.IsNull() || root.IsUndefined() {
		select {}
	}

	locale := readStorage("qcs_locale", "en")
	if _, ok := copy[locale]; !ok {
		locale = "en"
	}
	theme := readStorage("qcs_theme", "light")
	path := js.Global().Get("location").Get("pathname").String()

	doc.Get("documentElement").Call("setAttribute", "data-theme", theme)
	doc.Get("documentElement").Call("setAttribute", "lang", locale)

	root.Set("innerHTML", shellHTML(locale, theme, path))
	bindInteractions(root, locale, theme)
	select {}
}

func shellHTML(locale, theme, path string) string {
	section := routeSection(path)
	c := copy[locale]
	sectionLabel := map[string]string{
		"dashboard":     c["dashboard"],
		"tracking":      c["tracking"],
		"notifications": c["notifications"],
	}[section]
	return `
<style>
  :root { color-scheme: light; }
  html[data-theme="dark"] { color-scheme: dark; }
  body { margin: 0; font-family: "Segoe UI", sans-serif; background: linear-gradient(180deg, #f8fafc, #e2e8f0); color: #0f172a; }
  html[data-theme="dark"] body { background: linear-gradient(180deg, #020617, #0f172a); color: #e2e8f0; }
  .shell { max-width: 1080px; margin: 0 auto; padding: 24px; }
  .hero { display: grid; gap: 20px; grid-template-columns: 1.3fr 1fr; background: rgba(255,255,255,0.9); border: 1px solid rgba(148,163,184,0.25); border-radius: 24px; padding: 24px; box-shadow: 0 18px 40px rgba(15,23,42,0.12); }
  html[data-theme="dark"] .hero { background: rgba(15,23,42,0.88); border-color: rgba(148,163,184,0.2); }
  .eyebrow { display: inline-block; font-size: 12px; text-transform: uppercase; letter-spacing: 0.14em; color: #475569; }
  html[data-theme="dark"] .eyebrow { color: #94a3b8; }
  h1 { margin: 8px 0 10px; font-size: 44px; line-height: 1.05; }
  p { line-height: 1.6; }
  .nav, .toolbar { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
  .nav a, .toolbar button, .toolbar select { border-radius: 999px; border: 1px solid #cbd5e1; background: #fff; color: inherit; padding: 10px 14px; text-decoration: none; cursor: pointer; }
  html[data-theme="dark"] .nav a, html[data-theme="dark"] .toolbar button, html[data-theme="dark"] .toolbar select { background: #0f172a; border-color: #334155; color: #e2e8f0; }
  .nav a.active, .toolbar button.primary { background: #2563eb; color: #fff; border-color: #2563eb; }
  .cards { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); margin-top: 16px; }
  .card { background: rgba(255,255,255,0.92); border: 1px solid rgba(148,163,184,0.2); border-radius: 18px; padding: 18px; }
  html[data-theme="dark"] .card { background: rgba(15,23,42,0.92); border-color: rgba(148,163,184,0.2); }
  .muted { color: #64748b; }
  html[data-theme="dark"] .muted { color: #94a3b8; }
  .empty { margin-top: 18px; border: 1px dashed #cbd5e1; border-radius: 16px; padding: 18px; }
  html[data-theme="dark"] .empty { border-color: #334155; }
  @media (max-width: 820px) { .hero { grid-template-columns: 1fr; } h1 { font-size: 34px; } }
</style>
<div class="shell">
  <div class="toolbar" style="justify-content:flex-end; margin-bottom:12px;">
    <button id="theme-toggle">Theme: ` + escape(theme) + `</button>
    <select id="locale-select">
      <option value="en"` + selected(locale, "en") + `>EN</option>
      <option value="es"` + selected(locale, "es") + `>ES</option>
    </select>
    <a class="nav-link" href="/dashboard">` + escape(c["cta"]) + `</a>
  </div>
  <section class="hero">
    <div>
      <span class="eyebrow">WASM PWA Shell</span>
      <h1>` + escape(c["title"]) + `</h1>
      <p class="muted">` + escape(c["subtitle"]) + `</p>
      <p style="margin-top:14px;">` + escape(c["summary"]) + `</p>
      <nav class="nav" style="margin-top:18px;">
        <a href="/" class="` + active(section == "dashboard") + `">` + escape(c["dashboard"]) + `</a>
        <a href="/track" class="` + active(section == "tracking") + `">` + escape(c["tracking"]) + `</a>
        <a href="/dashboard/settings/notifications" class="` + active(section == "notifications") + `">` + escape(c["notifications"]) + `</a>
      </nav>
    </div>
    <div class="cards">
      <article class="card"><strong>` + escape(c["dashboard"]) + `</strong><p class="muted">Warehouse queue snapshots, shipments, recipients, and parcel-plus actions remain available from the dashboard surface.</p></article>
      <article class="card"><strong>` + escape(c["tracking"]) + `</strong><p class="muted">Public shipment state and confirmation tracking remain available without authentication.</p></article>
      <article class="card"><strong>` + escape(c["notifications"]) + `</strong><p class="muted">Realtime SSE, in-app notification list, and push subscription endpoints are now part of the platform baseline.</p></article>
    </div>
  </section>
  <section class="empty">
    <strong>` + escape(sectionLabel) + `</strong>
    <p class="muted" style="margin-top:8px;">` + escape(c["empty"]) + `</p>
  </section>
</div>`
}

func bindInteractions(root js.Value, locale, theme string) {
	doc := js.Global().Get("document")
	themeBtn := doc.Call("getElementById", "theme-toggle")
	localeSel := doc.Call("getElementById", "locale-select")

	themeHandler := js.FuncOf(func(this js.Value, args []js.Value) any {
		next := "dark"
		if readStorage("qcs_theme", theme) == "dark" {
			next = "light"
		}
		writeStorage("qcs_theme", next)
		js.Global().Get("location").Call("reload")
		return nil
	})
	themeBtn.Call("addEventListener", "click", themeHandler)

	localeHandler := js.FuncOf(func(this js.Value, args []js.Value) any {
		next := localeSel.Get("value").String()
		writeStorage("qcs_locale", next)
		js.Global().Get("location").Call("reload")
		return nil
	})
	localeSel.Call("addEventListener", "change", localeHandler)

	_ = root
}

func routeSection(pathname string) string {
	switch {
	case strings.HasPrefix(pathname, "/track"):
		return "tracking"
	case strings.Contains(pathname, "/notifications"):
		return "notifications"
	default:
		return "dashboard"
	}
}

func readStorage(key, fallback string) string {
	store := js.Global().Get("localStorage")
	if store.IsUndefined() || store.IsNull() {
		return fallback
	}
	value := store.Call("getItem", key)
	if value.IsNull() || value.IsUndefined() {
		return fallback
	}
	s := value.String()
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func writeStorage(key, value string) {
	store := js.Global().Get("localStorage")
	if store.IsUndefined() || store.IsNull() {
		return
	}
	store.Call("setItem", key, value)
}

func escape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

func selected(a, b string) string {
	if a == b {
		return " selected"
	}
	return ""
}

func active(on bool) string {
	if on {
		return "active"
	}
	return ""
}
