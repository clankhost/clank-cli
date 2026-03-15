// Package skill embeds the Clank Claude Code skill for distribution via the CLI.
package skill

import _ "embed"

//go:embed SKILL.md
var Content string
