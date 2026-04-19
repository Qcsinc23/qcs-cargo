package static

import "embed"

// Web contains static HTML and assets for the web app.
//
//go:embed *.html *.css *.js css js scripts destinations legal dashboard admin warehouse
var Web embed.FS
