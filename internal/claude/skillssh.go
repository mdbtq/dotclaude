package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// OriginKind says how a skill outside a plugin got into its skills directory.
type OriginKind string

const (
	// OriginSkillsSh is a skill skills.sh installed and lists in its lock file.
	OriginSkillsSh OriginKind = "skills.sh"
	// OriginUntracked is a skill in skills.sh's store, or a copy of one, that
	// its lock file does not list, so skills.sh neither updates nor removes it.
	OriginUntracked OriginKind = "untracked"
	// OriginLink is a symlink to a directory outside skills.sh's store, e.g.
	// a skill being developed in a repository.
	OriginLink OriginKind = "link"
	// OriginLocal is a directory that lives in the skills directory itself.
	OriginLocal OriginKind = "local"
)

// Origin is where a user or project skill comes from.
type Origin struct {
	Kind OriginKind `json:"kind"`
	// Source is the repository skills.sh installed the skill from, e.g.
	// owner/repo.
	Source string `json:"source,omitempty"`
	// Target is the directory a symlinked skill resolves to, or for an
	// untracked copy, the skills.sh store directory of the same name.
	Target string `json:"target,omitempty"`
}

// skillsStore is the place skills.sh (https://skills.sh) installs into: a
// canonical directory per skill, symlinked or copied into each agent's skills
// directory, and a lock file recording where each came from. Globally that is
// ~/.agents/skills with ~/.agents/.skill-lock.json (or
// $XDG_STATE_HOME/skills/.skill-lock.json); in a project, .agents/skills with
// skills-lock.json next to it.
type skillsStore struct {
	dir string
	// sources maps a skill name to the source it was installed from.
	sources map[string]string
}

func readSkillsStore(dir, lockPath string) skillsStore {
	s := skillsStore{dir: dir, sources: map[string]string{}}
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return s
	}
	var lock struct {
		Skills map[string]struct {
			Source string `json:"source"`
		} `json:"skills"`
	}
	if json.Unmarshal(data, &lock) != nil {
		return s
	}
	for name, entry := range lock.Skills {
		s.sources[name] = entry.Source
	}
	return s
}

// projectSkillsStore is the skills.sh store that belongs to a project .claude
// directory: skills.sh installs into .agents/skills of the directory it runs
// in and links from .claude/skills of that same directory.
func projectSkillsStore(claudeDir string) skillsStore {
	base := filepath.Dir(claudeDir)
	return readSkillsStore(filepath.Join(base, ".agents", "skills"), filepath.Join(base, "skills-lock.json"))
}

// setOrigins records where each skill in comps comes from. A skill counts as
// installed by skills.sh when the lock file lists it and it is either in the
// store or a directory in place (skills.sh's copy mode).
func setOrigins(comps []Component, store skillsStore) {
	storeDir, err := filepath.EvalSymlinks(store.dir)
	if err != nil {
		storeDir = ""
	}
	for i := range comps {
		c := &comps[i]
		if c.Kind != KindSkill || c.Error != "" {
			continue
		}
		dir := filepath.Dir(c.Path)
		entry := filepath.Base(dir)
		info, err := os.Lstat(dir)
		if err != nil {
			continue
		}
		linked := info.Mode()&os.ModeSymlink != 0
		target, _ := filepath.EvalSymlinks(dir)
		inStore := storeDir != "" && linked && filepath.Dir(target) == storeDir
		copied := storeDir != "" && !linked && isDir(filepath.Join(storeDir, entry))
		source, locked := store.sources[entry]

		switch {
		case locked && !(linked && !inStore):
			c.Origin = &Origin{Kind: OriginSkillsSh, Source: source}
		case inStore:
			c.Origin = &Origin{Kind: OriginUntracked, Target: target}
		case copied:
			c.Origin = &Origin{Kind: OriginUntracked, Target: filepath.Join(store.dir, entry)}
		case linked:
			c.Origin = &Origin{Kind: OriginLink, Target: target}
		default:
			c.Origin = &Origin{Kind: OriginLocal}
		}
	}
}
