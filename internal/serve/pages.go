package serve

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed assets/setup.html
var setupHTMLTemplate string

//go:embed assets/home.html
var homeHTMLTemplate string

// generateSetupHTML returns the admin password setup page.
func (s *Server) generateSetupHTML() string {
	return setupHTMLTemplate
}

// generateHomeHTML returns the home page with bundle data embedded as JSON.
func (s *Server) generateHomeHTML() string {
	bundles, _ := s.store.List()
	if bundles == nil {
		bundles = []BundleMeta{}
	}
	bundlesJSON, _ := json.Marshal(bundles)

	html := homeHTMLTemplate
	html = strings.Replace(html, "{{VERSION}}", s.version, 1)
	html = strings.Replace(html, "{{BUNDLES_JSON}}", string(bundlesJSON), 1)
	return html
}
