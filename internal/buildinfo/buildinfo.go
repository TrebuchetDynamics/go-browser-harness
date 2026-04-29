// Package buildinfo exposes binary version metadata.
package buildinfo

// Version is overridden by release builds with -ldflags "-X ...".
var Version = "dev"
