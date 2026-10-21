package config

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

const (
	DefaultAddress      = ":8080"
	DefaultMaxBodyBytes = 32 << 20
	DefaultMaxActions   = 10_000
	DefaultMaxXLSXCells = 200_000
)

type Config struct {
	Address               string
	MaxBodyBytes          int64
	MaxActions            int
	MaxXLSXCells          int
	MaxConcurrentRequests int
	PprofAddr             string
}

func Load(getenv func(string) string) (*Config, error) {
	cfg := &Config{
		Address:   valueOr(getenv("SERVER_ADDRESS"), DefaultAddress),
		PprofAddr: getenv("PPROF_ADDR"),
	}

	var errs []error
	parse := func(key string, fallback int) int {
		value, err := parseInt(getenv, key, fallback)
		if err != nil {
			errs = append(errs, err)
		}
		return value
	}
	cfg.MaxBodyBytes = int64(parse("MAX_BODY_BYTES", DefaultMaxBodyBytes))
	cfg.MaxActions = parse("MAX_ACTIONS", DefaultMaxActions)
	cfg.MaxXLSXCells = parse("MAX_XLSX_CELLS", DefaultMaxXLSXCells)
	cfg.MaxConcurrentRequests = parse("MAX_CONCURRENT_REQUESTS", 2*runtime.GOMAXPROCS(0))

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return cfg, nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func parseInt(getenv func(string) string, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}
	if value < 1 {
		return 0, fmt.Errorf("%s must be >= 1, got %d", key, value)
	}
	return value, nil
}
