package logger

import (
	"fmt"
	"os"
	"strconv"

	"go.uber.org/zap"
)

func New() (*zap.Logger, error) {
	isDebug, _ := strconv.ParseBool(os.Getenv("DEBUG"))

	logConfig := zap.NewProductionConfig()
	if isDebug {
		logConfig = zap.NewDevelopmentConfig()
	}

	l, err := logConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return l, nil
}
