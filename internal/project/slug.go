package project

import "strings"

// Slugify converts a string into a filesystem-friendly identifier.
func Slugify(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "migration"
	}
	return value
}
