package templates

import "embed"

// Handoff holds the built-in handoff prompt templates (handoff/<name>.md.tmpl),
// which 'metis handoff' renders for a slice's next session. A project
// overrides one by writing .metis/templates/handoff/<name>.md.tmpl.
//
//go:embed handoff/*.md.tmpl
var Handoff embed.FS
