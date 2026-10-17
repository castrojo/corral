package web

import (
	"testing"

	"golang.org/x/net/websocket"
)

// ── StrictOrigin tests ─────────────────────────────────────────────
// With StrictOrigin on (the try-tuna-desktop one-liner serves this way),
// a WebSocket upgrade from a foreign Origin must be refused at the
// handshake, while the page the server itself served keeps working.

func TestWebSocket_StrictOrigin_RefusesForeign(t *testing.T) {
	old := StrictOrigin
	StrictOrigin = true
	defer func() { StrictOrigin = old }()

	fx := NewTestFixture()
	defer fx.Close()

	url := wsURL(fx.Server.URL) + "/api/vnc/tailvm/testvm"
	ws, err := websocket.Dial(url, "", "http://evil.example")
	if err == nil {
		ws.Close()
		t.Error("foreign Origin dial succeeded, want handshake refusal")
	}
}

func TestWebSocket_StrictOrigin_AllowsSameOrigin(t *testing.T) {
	old := StrictOrigin
	StrictOrigin = true
	defer func() { StrictOrigin = old }()

	fx := NewTestFixture()
	defer fx.Close()

	// Origin equal to the server host: the handshake must pass. The
	// bridge itself then fails (no virtctl in tests) — Dial still
	// returns a connection, as in TestWebSocket_VNC_Upgrade.
	url := wsURL(fx.Server.URL) + "/api/vnc/tailvm/testvm"
	ws, err := websocket.Dial(url, "", fx.Server.URL)
	if err != nil {
		t.Fatalf("same-Origin dial failed: %v", err)
	}
	defer ws.Close()
}

func TestWebSocket_StrictOrigin_OffByDefault(t *testing.T) {
	if StrictOrigin {
		t.Error("StrictOrigin defaults to true; single-user back-compat requires false")
	}
}
