package serve

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/eljojo/rememory/internal/core"
)

// Config holds the configuration for the server.
type Config struct {
	Host            string
	Port            string
	DataDir         string
	MaxManifestSize int  // Maximum MANIFEST.age size in bytes
	NoTlock         bool // Omit time-lock support
	Version         string
}

// Server implements http.Handler for the self-hosted ReMemory web app.
type Server struct {
	store           *Store
	maxManifestSize int
	noTlock         bool
	version         string
	githubURL       string
	mux             *http.ServeMux
}

// New creates a new Server from the given config.
func New(cfg Config) (*Server, error) {
	store, err := NewStore(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("initializing store: %w", err)
	}

	var githubURL string
	if strings.HasPrefix(cfg.Version, "v") {
		githubURL = fmt.Sprintf("%s/releases/tag/%s", core.GitHubRepo, cfg.Version)
	} else {
		githubURL = core.GitHubRepo + "/releases/latest"
	}

	s := &Server{
		store:           store,
		maxManifestSize: cfg.MaxManifestSize,
		noTlock:         cfg.NoTlock,
		version:         cfg.Version,
		githubURL:       githubURL,
		mux:             http.NewServeMux(),
	}

	s.routes()
	return s, nil
}

// routes registers all HTTP routes.
func (s *Server) routes() {
	// Pages
	s.mux.HandleFunc("GET /", s.handleRoot)
	s.mux.HandleFunc("GET /create", s.handleCreate)
	s.mux.HandleFunc("GET /recover", s.handleRecover)
	s.mux.HandleFunc("GET /about", s.handleAbout)
	s.mux.HandleFunc("GET /docs", s.handleDocs)

	// API
	s.mux.HandleFunc("GET /api/status", s.handleAPIStatus)
	s.mux.HandleFunc("POST /api/setup", s.handleAPISetup)
	s.mux.HandleFunc("POST /api/bundle", s.handleAPISaveBundle)
	s.mux.HandleFunc("DELETE /api/bundle", s.handleAPIDeleteBundle)
	s.mux.HandleFunc("GET /api/bundle/manifest", s.handleAPIManifest)
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s)
}

// Store returns the server's store (for testing).
func (s *Server) Store() *Store {
	return s.store
}

// generateSetupHTML creates a simple setup page for setting the admin password.
func (s *Server) generateSetupHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ReMemory — Setup</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;
      background: #f5f5f5;
      color: #2E2A26;
      display: flex;
      justify-content: center;
      align-items: center;
      min-height: 100vh;
      padding: 2rem;
    }
    .setup-card {
      background: #fff;
      border: 1px solid #ddd;
      border-radius: 8px;
      padding: 2rem;
      max-width: 400px;
      width: 100%;
    }
    h1 {
      font-size: 1.25rem;
      margin-bottom: 0.5rem;
    }
    p {
      color: #6B6560;
      font-size: 0.875rem;
      margin-bottom: 1.5rem;
      line-height: 1.5;
    }
    label {
      display: block;
      font-size: 0.875rem;
      font-weight: 500;
      margin-bottom: 0.375rem;
    }
    input[type="password"] {
      width: 100%;
      padding: 0.5rem 0.75rem;
      border: 1px solid #ddd;
      border-radius: 4px;
      font-size: 0.875rem;
      margin-bottom: 1rem;
    }
    input[type="password"]:focus {
      outline: none;
      border-color: #55735A;
      box-shadow: 0 0 0 2px rgba(85, 115, 90, 0.2);
    }
    button {
      width: 100%;
      padding: 0.625rem;
      background: #55735A;
      color: #fff;
      border: none;
      border-radius: 4px;
      font-size: 0.875rem;
      font-weight: 500;
      cursor: pointer;
    }
    button:hover { background: #466B4A; }
    button:disabled { opacity: 0.6; cursor: not-allowed; }
    .error { color: #c44; font-size: 0.8125rem; margin-top: 0.5rem; }
    .hint { color: #8A8480; font-size: 0.8125rem; margin-top: 1rem; line-height: 1.4; }
  </style>
</head>
<body>
  <div class="setup-card">
    <h1>Set up ReMemory</h1>
    <p>Choose an admin password. You'll need it to delete bundles from this server.</p>
    <form id="setup-form">
      <label for="password">Admin password</label>
      <input type="password" id="password" name="password" required autocomplete="new-password">
      <label for="confirm">Confirm password</label>
      <input type="password" id="confirm" name="confirm" required autocomplete="new-password">
      <button type="submit">Set password</button>
      <div id="error" class="error"></div>
    </form>
    <p class="hint">This password protects administrative actions only. Your encrypted archives are secured by age encryption and Shamir's Secret Sharing.</p>
  </div>
  <script>
    document.getElementById('setup-form').addEventListener('submit', async function(e) {
      e.preventDefault();
      const pw = document.getElementById('password').value;
      const confirm = document.getElementById('confirm').value;
      const errorEl = document.getElementById('error');
      errorEl.textContent = '';

      if (pw !== confirm) {
        errorEl.textContent = 'Passwords do not match.';
        return;
      }
      if (pw.length < 8) {
        errorEl.textContent = 'Password must be at least 8 characters.';
        return;
      }

      const btn = this.querySelector('button');
      btn.disabled = true;
      btn.textContent = 'Setting up...';

      try {
        const resp = await fetch('/api/setup', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ password: pw }),
        });
        if (!resp.ok) {
          const text = await resp.text();
          throw new Error(text || 'Setup failed.');
        }
        window.location.href = '/';
      } catch (err) {
        errorEl.textContent = err.message;
        btn.disabled = false;
        btn.textContent = 'Set password';
      }
    });
  </script>
</body>
</html>`
}
