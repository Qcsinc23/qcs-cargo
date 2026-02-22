// Package frontend is the WASM entrypoint. Build with:
//   GOOS=js GOARCH=wasm go build -o web/app.wasm .
package main

import (
	"syscall/js"
)

func main() {
	doc := js.Global().Get("document")
	app := doc.Call("getElementById", "app")
	if !app.IsNull() {
		app.Set("innerHTML", `
			<div class="max-w-4xl mx-auto px-4 py-16 text-center">
				<p class="text-slate-500 mb-4">QCS Cargo</p>
				<h1 class="text-4xl font-bold text-slate-900 mb-4">Your Personal US Shipping Address</h1>
				<p class="text-lg text-slate-600 mb-8">Parcel Forwarding · Air Freight · Warehouse Operations</p>
				<a href="/how-it-works" class="text-blue-600 hover:underline">How It Works</a>
			</div>
		`)
	}
	select {}
}
