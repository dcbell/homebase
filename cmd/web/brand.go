package main

import (
	"io"
	"net/http"
)

const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
<style>
.bg{fill:#5865f2}.shape{fill:#fff}
@media(prefers-color-scheme:dark){.bg{fill:#7983f5}}
</style>
<rect class="bg" width="512" height="512" rx="112"/>
<path class="shape" d="M132 112h248q18 0 18 18v144q0 12-9 20L265 407q-9 8-18 0L123 294q-9-8-9-20V130q0-18 18-18Z"/>
</svg>`

func favicon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.WriteString(w, faviconSVG)
}
