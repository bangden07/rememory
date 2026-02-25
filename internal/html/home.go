package html

import (
	"encoding/json"
	"strings"
)

// GenerateHomeHTML creates the selfhosted home page with bundle data.
func GenerateHomeHTML(bundlesJSON string) string {
	content := homeHTMLTemplate
	content = strings.Replace(content, "{{BUNDLES_JSON}}", bundlesJSON, 1)

	script := `<script>` + strings.Replace(homeJS, "{{BUNDLES_JSON}}", bundlesJSON, 1) + `</script>`

	result := applyLayout(LayoutOptions{
		Title:         "ReMemory",
		Selfhosted:    true,
		PageStyles:    homeCSS,
		Content:       content,
		FooterContent: `<p>ReMemory</p><p class="version">{{VERSION}}</p>`,
		Scripts:       script,
	})

	return result
}

// HomeBundlesJSON serializes bundle metadata to JSON for the home page.
func HomeBundlesJSON(bundles any) string {
	data, _ := json.Marshal(bundles)
	return string(data)
}
