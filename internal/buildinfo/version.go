// Package buildinfo owns the version stamped into a release binary.
package buildinfo

// Version is overridden by release builds with -ldflags -X. Unstamped source
// builds stay visibly development builds; Runtime and --version share this value.
var Version = "0.2.0-dev"
