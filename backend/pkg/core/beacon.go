package core

import (
	"encoding/json"
	"net/http"

	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
)

// The opt-in behaviour beacon. A customer adds
//
//	<script src="/__hakaishield/b.js" async></script>
//
// to their pages. The script notes whether any pointer, scroll, key or
// touch input happened and whether navigator.webdriver is set, and sends
// those five flags once when the page is left. Nothing else: no
// coordinates, no keys, no timings, no identifiers.
const (
	beaconScriptPath = "/__hakaishield/b.js"
	beaconPath       = "/__hakaishield/beacon"
	// maxBeaconBytes bounds the body; a real beacon is about 40 bytes.
	maxBeaconBytes = 256
)

// beaconScript is served as-is. It sends at most one beacon per page,
// on pagehide or when the tab is hidden, whichever comes first.
const beaconScript = `(function(){var s={pm:0,sc:0,kd:0,tc:0},sent=0;` +
	`function on(t,e,k){t.addEventListener(e,function(){s[k]=1},{passive:true,capture:true})}` +
	`on(window,"pointermove","pm");on(window,"mousemove","pm");on(window,"scroll","sc");on(window,"wheel","sc");` +
	`on(window,"keydown","kd");on(window,"touchstart","tc");` +
	`function send(){if(sent)return;sent=1;var b=JSON.stringify({wd:navigator.webdriver?1:0,pm:s.pm,sc:s.sc,kd:s.kd,tc:s.tc});` +
	`if(navigator.sendBeacon){navigator.sendBeacon("` + beaconPath + `",b)}else{fetch("` + beaconPath + `",{method:"POST",body:b,keepalive:true})}}` +
	`window.addEventListener("pagehide",send);document.addEventListener("visibilitychange",function(){if(document.visibilityState==="hidden")send()})})();`

// serveBeaconScript serves the page script. It is cacheable: it has no
// per-visitor content.
func serveBeaconScript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodGet {
		_, _ = w.Write([]byte(beaconScript))
	}
}

// receiveBeacon records one beacon against the sender's own session. The
// tenant and client IP come from the guard, never from the body. A forged
// beacon can only make the sender's own session look more human or more
// automated, so it is accepted without further proof; the signals it
// feeds are evidence only.
func receiveBeacon(w http.ResponseWriter, r *http.Request, tenantID, ip string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		WD int `json:"wd"`
		PM int `json:"pm"`
		SC int `json:"sc"`
		KD int `json:"kd"`
		TC int `json:"tc"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBeaconBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		observability.Inc("beacon_rejected_total")
		http.Error(w, "bad beacon", http.StatusBadRequest)
		return
	}
	signals.RecordBeacon(tenantID, ip, r.UserAgent(), signals.Beacon{
		Webdriver: body.WD > 0,
		Pointer:   body.PM > 0,
		Scroll:    body.SC > 0,
		Key:       body.KD > 0,
		Touch:     body.TC > 0,
	})
	observability.Inc("beacon_received_total")
	w.WriteHeader(http.StatusNoContent)
}
