package i18n_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/capcom6/phone2tg-proxy/internal/i18n"
)

func TestNewService_English(t *testing.T) {
	config := i18n.Config{
		Language:         "en",
		TranslationsPath: filepath.Join("..", "..", "i18n", "locales"),
	}

	service, err := i18n.NewService(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Test translation
	translated := service.Translate("send_contact")
	expected := "Please, send me your contact"
	if translated != expected {
		t.Errorf("Expected '%s', got '%s'", expected, translated)
	}
}

func TestNewService_Russian(t *testing.T) {
	config := i18n.Config{
		Language:         "ru",
		TranslationsPath: filepath.Join("..", "..", "i18n", "locales"),
	}

	service, err := i18n.NewService(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Test translation
	translated := service.Translate("send_contact")
	expected := "Пожалуйста, отправьте мне ваш контакт"
	if translated != expected {
		t.Errorf("Expected '%s', got '%s'", expected, translated)
	}
}

func TestTranslate_ExistingKey(t *testing.T) {
	config := i18n.Config{
		Language:         "en",
		TranslationsPath: filepath.Join("..", "..", "i18n", "locales"),
	}

	service, err := i18n.NewService(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	translated := service.Translate("thanks_contact")
	expected := "Thanks for sharing your contact!"
	if translated != expected {
		t.Errorf("Expected '%s', got '%s'", expected, translated)
	}
}

func TestTranslate_MissingKey(t *testing.T) {
	config := i18n.Config{
		Language:         "en",
		TranslationsPath: filepath.Join("..", "..", "i18n", "locales"),
	}

	service, err := i18n.NewService(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	translated := service.Translate("missing_key")
	expected := "missing_key" // fallback to key
	if translated != expected {
		t.Errorf("Expected '%s', got '%s'", expected, translated)
	}
}

func TestTranslateWithArgs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "en.json"),
		[]byte(`{"greet":"Hello, {{name}}"}`),
		0o600,
	); err != nil {
		t.Fatalf("write temp translation: %v", err)
	}
	config := i18n.Config{
		Language:         "en",
		TranslationsPath: dir,
	}

	service, err := i18n.NewService(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	translated := service.TranslateWithArgs("greet", map[string]any{"name": "John"})
	expected := "Hello, John"
	if translated != expected {
		t.Errorf("Expected '%s', got '%s'", expected, translated)
	}
}

func TestNewService_InvalidLanguage(t *testing.T) {
	config := i18n.Config{
		Language:         "invalid",
		TranslationsPath: filepath.Join("..", "..", "i18n", "locales"),
	}

	_, err := i18n.NewService(config)
	if err == nil {
		t.Fatal("Expected error for invalid language, got nil")
	}
}
