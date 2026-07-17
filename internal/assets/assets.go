package assets

import "embed"

// FS contains versioned prompts and generated configuration templates.
//
//go:embed prompts templates
var FS embed.FS
