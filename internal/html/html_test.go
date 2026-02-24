package html

import (
	"strings"
	"testing"
)

// TestStaticHTMLHasNoServerCode verifies that static HTML builds (non-selfhosted)
// contain zero traces of server-interaction code. This ensures offline bundles
// never contain code that could phone home.
func TestStaticHTMLHasNoServerCode(t *testing.T) {
	// These patterns must NOT appear in static output.
	// Note: SELFHOSTED_CONFIG = null is allowed (it's inert).
	// We check for the server-interaction code that should be eliminated by esbuild.
	forbidden := []string{
		"/api/bundle",
		"/api/setup",
		"rememoryOnBundlesCreated",
		"rememoryLoadManifest",
	}

	// Test maker.html (static mode)
	// Need WASM bytes to generate — use a minimal stub
	wasmStub := []byte{0x00, 0x61, 0x73, 0x6d} // WASM magic bytes
	maker := GenerateMakerHTML(wasmStub, "test", "https://github.com/eljojo/rememory", MakerHTMLOptions{})

	for _, pattern := range forbidden {
		if strings.Contains(maker, pattern) {
			t.Errorf("static maker.html contains server code: found %q", pattern)
		}
	}

	// Test recover.html (static mode, no personalization)
	recover := GenerateRecoverHTML("test", "https://github.com/eljojo/rememory", nil)
	for _, pattern := range forbidden {
		if strings.Contains(recover, pattern) {
			t.Errorf("static recover.html contains server code: found %q", pattern)
		}
	}

	// Test index.html
	index := GenerateIndexHTML("test", "https://github.com/eljojo/rememory")
	for _, pattern := range forbidden {
		if strings.Contains(index, pattern) {
			t.Errorf("static index.html contains server code: found %q", pattern)
		}
	}
}
