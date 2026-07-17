package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Size is an int64 byte count that parses values like "50M", "500K", "1G".
type Size int64

func (s *Size) Set(value string) error {
	v := strings.ToUpper(strings.TrimSpace(value))
	v = strings.TrimSuffix(v, "B")
	if v == "" {
		return fmt.Errorf("invalid size %q", value)
	}
	multiplier := int64(1)
	switch v[len(v)-1] {
	case 'K':
		multiplier, v = 1<<10, v[:len(v)-1]
	case 'M':
		multiplier, v = 1<<20, v[:len(v)-1]
	case 'G':
		multiplier, v = 1<<30, v[:len(v)-1]
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid size %q", value)
	}
	*s = Size(n * float64(multiplier))
	return nil
}

func (s *Size) String() string {
	return formatSize(int64(*s))
}

func formatSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
