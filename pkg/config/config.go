package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Load reads configuration from various sources and unmarshals it into a given struct.
//
// It looks for configuration in the following order (later overrides earlier):
// 1. Local file, if `WithLocalYAML` is provided.
// 2. `.env` file in the current working directory.
// 3. Environment variables.
//
// If any of the above sources result in an error (other than `os.ErrNotExist`), it will be returned.
//
// If a source results in `os.ErrNotExist`, it will be skipped.
//
// The final configuration will be unmarshaled into the given struct. If unmarshaling fails, an error will be returned.
func Load[T any](c *T, opts ...Option) error {
	options := new(options)
	options.apply(opts...)

	k := koanf.New(".")

	if err := loadFromYAML(options.withYaml, k); err != nil {
		return err
	}

	if err := loadDotenv(k); err != nil {
		return err
	}

	if err := loadEnv(k); err != nil {
		return err
	}

	if err := k.Unmarshal("", c); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	return nil
}

func loadFromYAML(path string, k *koanf.Koanf) error {
	if path == "" {
		return nil
	}

	err := k.Load(file.Provider(path), yaml.Parser())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load yaml: %w", err)
	}

	return nil
}

func loadDotenv(k *koanf.Koanf) error {
	err := k.Load(file.Provider(".env"), dotenv.ParserEnvWithValue("", "__", envTransform))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load dotenv: %w", err)
	}

	return nil
}

func loadEnv(k *koanf.Koanf) error {
	if err := k.Load(env.Provider("__", env.Opt{
		Prefix:        "",
		TransformFunc: envTransform,
		EnvironFunc:   nil,
	}), nil); err != nil {
		return fmt.Errorf("load env: %w", err)
	}

	return nil
}

func envTransform(k, v string) (string, any) {
	k = strings.ToLower(k)
	// JSON object -> map
	if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") {
		var m map[string]any
		if err := json.Unmarshal([]byte(v), &m); err == nil {
			return k, m
		}
	}
	// JSON array -> []any
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		var a []any
		if err := json.Unmarshal([]byte(v), &a); err == nil {
			return k, a
		}
	}
	return k, v
}
