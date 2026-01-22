package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Service implements the Translator interface.
type Service struct {
	translations map[string]string
}

// NewService creates a new translator for the specified language and translations path.
func NewService(config Config) (*Service, error) {
	translations, err := loadTranslations(config.TranslationsPath, config.Language)
	if err != nil {
		return nil, fmt.Errorf("load translations: %w", err)
	}

	return &Service{
		translations: translations,
	}, nil
}

// Translate returns the translated string for the given key.
func (t *Service) Translate(key string) string {
	translated, ok := t.translations[key]
	if !ok {
		return key // fallback only when missing
	}

	return translated
}

// TranslateWithArgs returns the translated string with placeholders replaced by args.
func (t *Service) TranslateWithArgs(key string, args map[string]any) string {
	translated := t.Translate(key)

	// Simple placeholder replacement, assuming placeholders like {{key}}
	for k, v := range args {
		placeholder := "{{" + k + "}}"
		translated = strings.ReplaceAll(translated, placeholder, fmt.Sprintf("%v", v))
	}

	return translated
}

// loadTranslations loads translations from JSON files in the specified path.
func loadTranslations(path, language string) (map[string]string, error) {
	filePath := filepath.Join(path, language+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read translation file %s: %w", filePath, err)
	}

	var translations map[string]string
	if jsonErr := json.Unmarshal(data, &translations); jsonErr != nil {
		return nil, fmt.Errorf("unmarshal translations: %w", jsonErr)
	}

	return translations, nil
}
