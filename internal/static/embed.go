package static

import "embed"

// Web contains static HTML and assets for the web app.
//
//go:embed *.html *.js destinations legal dashboard admin warehouse
var Web embed.FS
