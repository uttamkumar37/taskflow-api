// Package version exposes the build version, overridden at build time via
// `-ldflags "-X taskflow/internal/version.Version=..."` (see Makefile and
// Dockerfile). Surfacing this in /healthz and startup logs answers the
// "what build is actually running on this instance" question that always
// comes up during an incident.
package version

var Version = "dev"
