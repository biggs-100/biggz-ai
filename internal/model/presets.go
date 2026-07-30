// Package model provides core types and preset definitions.
package model

// Preset defines a named installation preset with its component list.
type Preset struct {
	Name        string
	Description string
	Components  []string // component IDs
}

// Presets returns the available installation presets.
func Presets() []Preset {
	return []Preset{
		{
			Name:        "full",
			Description: "Complete installation — all features",
			Components: []string{
				"sdd", "bigmem", "rdd", "skills", "commands",
				"mcp", "persona", "tdd", "profiles", "context7",
			},
		},
		{
			Name:        "ecosystem",
			Description: "Core SDD + BigMem + skills",
			Components: []string{
				"sdd", "bigmem", "rdd", "skills", "commands",
				"mcp", "persona",
			},
		},
		{
			Name:        "minimal",
			Description: "Minimal — SDD only",
			Components: []string{
				"sdd", "skills",
			},
		},
		{
			Name:        "custom",
			Description: "Select components manually",
			Components: []string{},
		},
	}
}

// ComponentsForPreset returns the component list for a preset name.
func ComponentsForPreset(name string) []string {
	for _, p := range Presets() {
		if p.Name == name {
			return p.Components
		}
	}
	return nil
}
