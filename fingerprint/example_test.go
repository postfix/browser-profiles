package fingerprint_test

import (
	"fmt"

	"github.com/postfix/browser-profiles/fingerprint"
)

// ExampleGenerateFingerprint generates a realistic, internally-consistent
// fingerprint for a given platform. Platform deterministically maps to a
// fixed navigator.platform string ("windows" -> "Win32"); other fields
// (UserAgent, WebGL renderer, HardwareConcurrency, etc.) are randomized on
// every call.
func ExampleGenerateFingerprint() {
	fp := fingerprint.GenerateFingerprint(fingerprint.GenerateFingerprintOptions{Platform: "windows"})
	fmt.Println(fp.Platform)
	// Output:
	// Win32
}
