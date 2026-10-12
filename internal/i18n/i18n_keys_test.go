package i18n_test

// i18n_keys_test.go pins the exact 8-key translation set the bot depends on.
// A dropped key is a SILENT runtime failure: i18n.Service.Translate returns
// the key itself on miss, so no error ever surfaces. This oracle keeps both
// locales at exactly the pinned key set - no more, no fewer.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// i18nExpectedKeys is the single source of truth for the translation keys.
var i18nExpectedKeys = []string{
	"welcome_message",
	"send_contact",
	"share_contact",
	"invalid_phone",
	"thanks_contact",
	"delete_success",
	"delete_failed",
	"share_contact_button",
}

// localesDir points at the repo-root i18n/locales directory.
const localesDir = "../../i18n/locales"

// TestI18n_AllKeysPresent asserts that en.json and ru.json each contain
// exactly the 8 expected keys.
func TestI18n_AllKeysPresent(t *testing.T) {
	locales := []string{"en", "ru"}

	for _, lang := range locales {
		t.Run(lang, func(t *testing.T) {
			path := filepath.Join(localesDir, lang+".json")

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			var translations map[string]string
			if unmarshalErr := json.Unmarshal(data, &translations); unmarshalErr != nil {
				t.Fatalf("unmarshal %s: %v", path, unmarshalErr)
			}

			// exact key set: no missing, no extra
			got := make(map[string]struct{}, len(translations))
			for key := range translations {
				got[key] = struct{}{}
			}

			for _, key := range i18nExpectedKeys {
				if _, ok := got[key]; !ok {
					t.Errorf("%s.json: missing key %q", lang, key)
				}
			}
			if len(translations) != len(i18nExpectedKeys) {
				t.Errorf("%s.json: expected exactly %d keys, got %d", lang, len(i18nExpectedKeys), len(translations))
			}
		})
	}
}
