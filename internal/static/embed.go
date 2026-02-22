package static

import "embed"

// Web contains static HTML and assets for the web app.
//
//go:embed *.html destinations legal dashboard
var Web embed.FS
