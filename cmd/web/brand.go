package main

import (
	"io"
	"net/http"
)

const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
<style>
.bg{fill:#5865f2}.roof{fill:none;stroke:#fff}.shape{fill:#fff}.primary{stroke:#5865f2}.muted{stroke:#5c6470}
@media(prefers-color-scheme:dark){.bg{fill:#7983f5}.primary{stroke:#7983f5}.muted{stroke:#b5bac1}}
</style>
<rect class="bg" width="512" height="512" rx="112"/>
<path class="roof" d="M108 140 260 50 412 140" stroke-width="26" stroke-linecap="round" stroke-linejoin="round"/>
<path class="shape" d="M96 390V206q0-28 28-28h82q14 0 26 12l24 24h136q32 0 32 32v144q0 34-34 34H130q-34 0-34-34Z" stroke="none"/>
<path class="shape" d="M96 392V246q0-28 28-28h268q32 0 32 32v142q0 32-34 32H130q-34 0-34-32Z" stroke="none"/>
<path class="primary" d="M154 296h164" fill="none" stroke-width="20" stroke-linecap="round"/>
<path class="muted" d="M154 340h112" fill="none" stroke-width="20" stroke-linecap="round"/>
</svg>`

func favicon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.WriteString(w, faviconSVG)
}
