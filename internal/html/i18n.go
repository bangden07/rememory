package html

import (
	"strings"

	"github.com/eljojo/rememory/internal/translations"
)

// I18nScriptOptions controls the page-specific parts of the shared i18n script block.
type I18nScriptOptions struct {
	Component           string // translation component name: "maker", "recover", "index"
	UseNonce            bool   // whether to add nonce="{{CSP_NONCE}}" to the script tag
	ExtraDeclarations   string // JS inserted after `const translations = ...;` (e.g. index's docsLangs)
	SetLanguageExtra    string // JS appended inside setLanguage() after the core DOM updates
	DOMContentLoadedPre string // JS at start of DOMContentLoaded handler, before setLanguage()
	OnLangChange        string // JS in the lang-select change handler after setLanguage()
}

// i18nScript generates the complete i18n <script> block shared by maker, recover, and index pages.
func i18nScript(opts I18nScriptOptions) string {
	var s strings.Builder

	nonceAttr := ""
	if opts.UseNonce {
		nonceAttr = ` nonce="{{CSP_NONCE}}"`
	}

	s.WriteString(`
  <!-- Translations -->
  <script` + nonceAttr + `>
    const translations = ` + translations.GetTranslationsJS(opts.Component) + `;`)

	if opts.ExtraDeclarations != "" {
		s.WriteString("\n    " + opts.ExtraDeclarations)
	}

	s.WriteString(`

    let currentLang = 'en';

    function t(key, ...args) {
      let text = translations[currentLang][key] || translations['en'][key] || key;
      args.forEach((arg, i) => {
        text = text.replace(` + "`{${i}}`" + `, arg);
      });
      return text;
    }

    function setLanguage(lang) {
      currentLang = lang;
      localStorage.setItem('rememory-lang', lang);

      // Update select
      const sel = document.getElementById('lang-select');
      if (sel) sel.value = lang;

      // Update all translatable elements
      document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.dataset.i18n;
        el.textContent = t(key);
      });

      // Update placeholder attributes
      document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        const key = el.dataset.i18nPlaceholder;
        el.placeholder = t(key);
      });

      // Update page title
      document.title = t('title');`)

	if opts.SetLanguageExtra != "" {
		s.WriteString("\n\n      " + opts.SetLanguageExtra)
	}

	s.WriteString(`
    }

    // Set initial language immediately
    (function() {
      const saved = localStorage.getItem('rememory-lang');
      const langs = ` + translations.LangDetectJS() + `;
      const detected = navigator.languages.find((l) => langs.includes(l))
        || navigator.languages.map((l) => l.split('-')[0]).find((l) => langs.includes(l));
      currentLang = saved || detected || 'en';
    })();

    // Initialize language select after DOM is ready
    document.addEventListener('DOMContentLoaded', () => {`)

	if opts.DOMContentLoadedPre != "" {
		s.WriteString("\n      " + opts.DOMContentLoadedPre + "\n")
	}

	s.WriteString(`
      setLanguage(currentLang);

      document.getElementById('lang-select')?.addEventListener('change', (e) => {
        setLanguage(e.target.value);`)

	if opts.OnLangChange != "" {
		s.WriteString("\n        " + opts.OnLangChange)
	}

	s.WriteString(`
      });
    });
  </script>`)

	return s.String()
}
