// Package version reports the application version number.
package version

import "fmt"

const (
	// Major is incremented when making incompatible API changes.
	Major = 0
	// Minor is incremented when adding backwards compatible functionality.
	Minor = 7
	// Patch is incremented when making backwards compatible bug fixes.
	Patch = 0
)

// String returns the full version number.
func String() string {
	return fmt.Sprintf("%d.%d.%d", Major, Minor, Patch)
}
