// Package web embeds the rider and driver browser apps so they ship inside the
// web service binary — no static files to mount in the container image.
package web

import "embed"

//go:embed index.html rider driver shared
var Assets embed.FS
