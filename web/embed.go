// Package web embeds the built frontend (web/dist, produced by `pnpm build`).
package web

import "embed"

// Dist holds the built single-page app under the "dist/" prefix. When the frontend has not
// been built it only contains dist/.gitkeep and the server shows a "frontend not built" page.
//
//go:embed all:dist
var Dist embed.FS
