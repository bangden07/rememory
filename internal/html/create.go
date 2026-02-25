package html

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/eljojo/rememory/internal/core"
	"github.com/eljojo/rememory/internal/translations"
)

// tlockTabsHTML is the Simple/Advanced tab switcher injected into maker.html step 3.
const tlockTabsHTML = `<span id="advanced-options" class="mode-tabs hidden">
          <button type="button" class="mode-tab active" data-mode="simple" data-i18n="mode_simple">Simple</button>
          <button type="button" class="mode-tab" data-mode="advanced" data-i18n="mode_advanced">Advanced</button>
        </span>`

// tlockPanelHTML is the time-lock options panel injected into maker.html step 3.
const tlockPanelHTML = `<!-- Advanced: time lock (shown when Advanced tab is active) -->
      <div id="timelock-panel" class="hidden" style="margin-bottom: 1rem;">
        <div style="display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap;">
          <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer; margin: 0;">
            <input type="checkbox" id="timelock-checkbox">
            <span data-i18n="timelock_label">Add a time lock</span>
            <span style="font-size: 0.75rem; color: #8A8480; background: #f0f0ee; padding: 0.125rem 0.5rem; border-radius: 3px;" data-i18n="timelock_experimental">experimental</span>
          </label>
          <div id="timelock-options" class="hidden" style="display: flex; align-items: center; gap: 0.5rem;">
            <input type="number" id="timelock-value" min="1" value="30" style="width: 5rem; padding: 0.375rem; border: 1px solid #ddd; border-radius: 4px;">
            <select id="timelock-unit" style="padding: 0.375rem; border: 1px solid #ddd; border-radius: 4px;">
              <option value="min" data-i18n="timelock_minutes">minutes</option>
              <option value="h" data-i18n="timelock_hours">hours</option>
              <option value="d" selected data-i18n="timelock_days">days</option>
              <option value="w" data-i18n="timelock_weeks">weeks</option>
              <option value="m" data-i18n="timelock_months">months</option>
              <option value="y" data-i18n="timelock_years">years</option>
            </select>
          </div>
        </div>
        <div id="timelock-details" class="hidden" style="margin-top: 0.5rem;">
          <p id="timelock-date-preview" style="margin: 0; font-size: 0.875rem; color: #6B6560;"></p>
          <p style="margin: 0.25rem 0 0; font-size: 0.8125rem; color: #8A8480;"><span data-i18n="timelock_hint">Even with enough pieces, the files stay locked until this date.</span> <a href="{{GITHUB_PAGES}}/docs#timelock" target="_blank" style="color: #7A8FA6;" data-i18n="timelock_learn_more">How does this work?</a></p>
          <p style="margin: 0.25rem 0 0; font-size: 0.8125rem; color: #8A8480;" data-i18n="timelock_network_hint">Recovery will need a brief internet connection to verify the time lock.</p>
        </div>
      </div>`

// MakerHTMLOptions holds optional parameters for GenerateMakerHTML.
type MakerHTMLOptions struct {
	Selfhosted       bool              // Use selfhosted JS variant with server integration
	SelfhostedConfig *SelfhostedConfig // Config injected into the HTML for selfhosted mode
}

// SelfhostedConfig holds configuration passed to the selfhosted frontend.
type SelfhostedConfig struct {
	MaxManifestSize int    `json:"maxManifestSize"`       // Maximum MANIFEST.age size the server accepts
	HasManifest     bool   `json:"hasManifest"`           // Whether a manifest currently exists on the server
	ManifestURL     string `json:"manifestURL,omitempty"` // URL to fetch manifest from (set by server or static pages)
}

// GenerateMakerHTML creates the complete maker.html with all assets embedded.
// createWASMBytes is the create.wasm binary (runs in browser for bundle creation).
// Note: recover.html uses native JavaScript crypto, not WASM.
func GenerateMakerHTML(createWASMBytes []byte, opts MakerHTMLOptions) string {
	// Process content template
	content := makerHTMLTemplate
	content = strings.Replace(content, "{{TLOCK_TABS_HTML}}", tlockTabsHTML, 1)
	content = strings.Replace(content, "{{TLOCK_PANEL_HTML}}", tlockPanelHTML, 1)

	// Build CSP connect-src
	cspConnectSrc := "blob:"
	if opts.Selfhosted {
		cspConnectSrc += " 'self'"
	}

	// CSP meta tag
	headMeta := `<meta name="generator" content="ReMemory {{VERSION}}">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'nonce-{{CSP_NONCE}}' 'wasm-unsafe-eval'; style-src 'unsafe-inline'; img-src blob: data:; connect-src ` + cspConnectSrc + `; form-action 'none';">`

	// Language selector for nav
	navExtras := `<select class="lang-select" id="lang-select">
        ` + translations.LangSelectOptions() + `
      </select>`

	// Selfhosted config
	var selfhostedConfigJSON string
	if opts.Selfhosted && opts.SelfhostedConfig != nil {
		configData, _ := json.Marshal(opts.SelfhostedConfig)
		selfhostedConfigJSON = string(configData)
	} else {
		selfhostedConfigJSON = "null"
	}

	// Max total file size
	var maxTotalFileSize string
	if opts.Selfhosted && opts.SelfhostedConfig != nil {
		maxTotalFileSize = strconv.Itoa(opts.SelfhostedConfig.MaxManifestSize)
	} else {
		maxTotalFileSize = strconv.Itoa(core.MaxTotalSize)
	}

	// Select app JS variant
	appScript := createAppJS
	if opts.Selfhosted {
		appScript = createAppSelfhostedJS
	}

	// Embed create.wasm as gzip-compressed base64
	createWASMB64 := compressAndEncode(createWASMBytes)

	// Build all scripts
	var scripts strings.Builder

	// Translations
	scripts.WriteString(i18nScript(I18nScriptOptions{
		Component: "maker",
		UseNonce:  true,
		SetLanguageExtra: `// Re-render dynamic content
      if (typeof window.rememoryUpdateUI === 'function') {
        window.rememoryUpdateUI();
      }`,
	}))

	// WASM runtime
	scripts.WriteString("\n\n  <!-- Go WASM runtime -->\n  <script nonce=\"{{CSP_NONCE}}\">" + wasmExecJS + "</script>")

	// WASM binary and config
	scripts.WriteString(`

  <!-- Embedded WASM binary (base64) -->
  <script nonce="{{CSP_NONCE}}">
    window.WASM_BINARY = "` + createWASMB64 + `";
    window.VERSION = "{{VERSION}}";
    window.SELFHOSTED_CONFIG = ` + selfhostedConfigJSON + `;
    window.MAX_TOTAL_FILE_SIZE = ` + maxTotalFileSize + `;
  </script>`)

	// Tlock encryption config
	scripts.WriteString("\n\n  <!-- Time-lock encryption (conditionally included) -->\n  " + drandConfigScript())

	// Application logic
	scripts.WriteString("\n\n  <!-- Application logic -->\n  <script nonce=\"{{CSP_NONCE}}\">" + sharedJS + "\n" + appScript + "</script>")

	// WASM loader
	scripts.WriteString(`

  <!-- Load WASM from embedded gzip-compressed binary -->
  <script nonce="{{CSP_NONCE}}">
    (async function() {
      const go = new Go();
      try {
        // Decode base64 to get gzip-compressed data
        const compressed = Uint8Array.from(atob(window.WASM_BINARY), c => c.charCodeAt(0));

        // Decompress using DecompressionStream (modern browsers)
        let bytes;
        if (typeof DecompressionStream !== 'undefined') {
          const ds = new DecompressionStream('gzip');
          const writer = ds.writable.getWriter();
          writer.write(compressed);
          writer.close();
          const reader = ds.readable.getReader();
          const chunks = [];
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            chunks.push(value);
          }
          const totalLength = chunks.reduce((acc, chunk) => acc + chunk.length, 0);
          bytes = new Uint8Array(totalLength);
          let offset = 0;
          for (const chunk of chunks) {
            bytes.set(chunk, offset);
            offset += chunk.length;
          }
        } else {
          // Fallback: use pako if available, or show error
          if (typeof pako !== 'undefined') {
            bytes = pako.inflate(compressed);
          } else {
            throw new Error('Browser does not support DecompressionStream');
          }
        }

        const result = await WebAssembly.instantiate(bytes.buffer, go.importObject);
        go.run(result.instance);
      } catch (err) {
        // Show user-friendly error with guidance
        const indicator = document.getElementById('wasm-loading-indicator');
        if (indicator) indicator.classList.add('hidden');
        const errorContainer = document.getElementById('wasm-error-container');
        if (errorContainer) {
          errorContainer.classList.remove('hidden');
          errorContainer.innerHTML = ` + "`" + `
            <div class="wasm-fallback">
              <p><strong>Could not load the bundle creator</strong></p>
              <p>This can happen with older browsers or certain privacy settings.</p>
              <div class="loading-error-actions">
                <button class="btn btn-primary" id="reload-page-btn">Reload page</button>
                <a href="{{GITHUB_REPO}}" class="btn btn-secondary" target="_blank">Use CLI tool instead</a>
              </div>
            </div>
          ` + "`" + `;
          document.getElementById('reload-page-btn')?.addEventListener('click', () => window.location.reload());
        }
      }
    })();
  </script>`)

	// Nav-hiding script: remove the Create link from nav (current page)
	navHideScript := `
  <script nonce="{{CSP_NONCE}}">document.querySelector('#nav-links-main a[href="maker.html"]')?.remove();</script>`

	// Assemble page using layout
	result := applyLayout(LayoutOptions{
		Title:      "\xF0\x9F\xA7\xA0 ReMemory - Create Recovery Bundles",
		HeadMeta:   headMeta,
		PageStyles: makerCSS,
		Selfhosted: opts.Selfhosted,
		NavExtras:  navExtras,
		BeforeContainer: `<!-- Toast notifications container -->
  <div id="toast-container" class="toast-container" role="alert" aria-live="polite"></div>`,
		Content:       content,
		FooterContent: `<p><span data-i18n="works_offline">Works completely offline</span></p><p class="version">{{VERSION}}</p>`,
		Scripts:       navHideScript + scripts.String(),
	})

	// Apply CSP nonce to all script tags
	result = applyCSPNonce(result)

	return result
}
