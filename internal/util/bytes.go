package util

import (
	"fmt"
	"math"
)

// FormatBytes formats bytes as IEC units (KiB, MiB, GiB, ...).
func FormatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	f := float64(bytes)
	exp := int(math.Floor(math.Log(f) / math.Log(1024)))
	if exp < 0 {
		exp = 0
	}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	value := f / math.Pow(1024, float64(exp))
	if exp == 0 {
		return fmt.Sprintf("%d %s", bytes, units[exp])
	}
	return fmt.Sprintf("%.1f %s", value, units[exp])
}
