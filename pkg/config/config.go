package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

func Load[T any](c *T) error {
	k := koanf.New(".")

	err := k.Load(file.Provider(".env"), dotenv.ParserEnv("", "__", func(s string) string { return s }))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load dotenv: %w", err)
	}

	if err := k.Load(env.Provider("", "__", nil), nil); err != nil {
		return fmt.Errorf("load env: %w", err)
	}

	if err := k.Unmarshal("", c); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	return nil
}
