package static

import "embed"

// Web contains static HTML and assets for the web app.
//go:embed *.html
var Web embed.FS
