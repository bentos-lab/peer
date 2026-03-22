package cli

// ResolveBool returns the first non-nil bool pointer value or the default value if none are set.
func ResolveBool(primary *bool, fallback *bool, defaultValue bool) bool {
	if primary != nil {
		return *primary
	}
	if fallback != nil {
		return *fallback
	}
	return defaultValue
}
