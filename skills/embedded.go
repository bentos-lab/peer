package skills

import (
	"embed"
	"io/fs"
)

//go:embed peer/**
var embeddedSkills embed.FS

// PeerSkillFS exposes the embedded peer skill filesystem and its root path.
func PeerSkillFS() (fs.FS, string) {
	return embeddedSkills, "peer"
}
