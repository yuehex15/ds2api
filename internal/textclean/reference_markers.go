package textclean

import "regexp"

var referenceMarkerPattern = regexp.MustCompile(`(?i)\[reference:\s*\d+\]`)

func StripReferenceMarkers(text string) string {
	if text == "" {
		return text
	}
	return referenceMarkerPattern.ReplaceAllString(text, "")
}

// StripReferenceMarkersEnabled returns true while reference-marker
// stripping remains the fixed runtime default.  When the behaviour is
// eventually removed this function can be deleted and callers can drop
// the conditional.
func StripReferenceMarkersEnabled() bool {
	return true
}
