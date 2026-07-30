package catalog

import (
	"testing"
)

func TestAllAgents_ReturnsThree(t *testing.T) {
	agents := AllAgents()
	if len(agents) != 3 {
		t.Fatalf("AllAgents() = %d entries, want 3", len(agents))
	}
	for _, a := range agents {
		if a.ID == "" || a.Name == "" || a.Description == "" || a.Tier == "" {
			t.Errorf("AllAgents() entry %+v has empty fields", a)
		}
	}
}

func TestAllAgents_Immutability(t *testing.T) {
	agents := AllAgents()
	if len(agents) == 0 {
		t.Fatal("AllAgents() returned empty")
	}
	agents[0].Name = "Hacked"
	agents2 := AllAgents()
	if agents2[0].Name == "Hacked" {
		t.Error("AllAgents() returned slice shares backing array with original")
	}
}

func TestGetAgent_Found(t *testing.T) {
	a := GetAgent("opencode")
	if a == nil {
		t.Fatal("GetAgent(\"opencode\") returned nil")
	}
	if a.ID != "opencode" || a.Name != "OpenCode" {
		t.Errorf("GetAgent(\"opencode\") = %+v, want ID=opencode Name=OpenCode", a)
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	if got := GetAgent("nonexistent"); got != nil {
		t.Errorf("GetAgent(\"nonexistent\") = %v, want nil", got)
	}
}

func TestIsSupportedAgent(t *testing.T) {
	if !IsSupportedAgent("opencode") {
		t.Error("IsSupportedAgent(\"opencode\") = false, want true")
	}
	if !IsSupportedAgent("claude-code") {
		t.Error("IsSupportedAgent(\"claude-code\") = false, want true")
	}
	if !IsSupportedAgent("qwen-code") {
		t.Error("IsSupportedAgent(\"qwen-code\") = false, want true")
	}
	if IsSupportedAgent("unknown") {
		t.Error("IsSupportedAgent(\"unknown\") = true, want false")
	}
}

func TestAllComponents_ReturnsThree(t *testing.T) {
	components := AllComponents()
	if len(components) != 3 {
		t.Fatalf("AllComponents() = %d entries, want 3", len(components))
	}
}

func TestAllComponents_Immutability(t *testing.T) {
	components := AllComponents()
	if len(components) == 0 {
		t.Fatal("AllComponents() returned empty")
	}
	components[0].Name = "Hacked"
	components2 := AllComponents()
	if components2[0].Name == "Hacked" {
		t.Error("AllComponents() returned slice shares backing array")
	}
}

func TestListComponents_TierFilter(t *testing.T) {
	all := ListComponents("")
	if len(all) != 3 {
		t.Errorf("ListComponents(\"\") = %d, want 3", len(all))
	}
	native := ListComponents("native")
	if len(native) != 3 {
		t.Errorf("ListComponents(\"native\") = %d, want 3", len(native))
	}
	community := ListComponents("community")
	if len(community) != 0 {
		t.Errorf("ListComponents(\"community\") = %d, want 0", len(community))
	}
}

func TestAllSkills_ReturnsAtLeastOnePerTier(t *testing.T) {
	skills := AllSkills()
	if len(skills) == 0 {
		t.Fatal("AllSkills() returned empty")
	}
	tiers := make(map[string]bool)
	for _, s := range skills {
		tiers[s.Tier] = true
		if s.Platforms == nil {
			t.Errorf("Skill %q has nil Platforms", s.ID)
		}
		if s.DependsOn == nil {
			t.Errorf("Skill %q has nil DependsOn", s.ID)
		}
	}
	if !tiers["native"] {
		t.Error("AllSkills() missing native tier entry")
	}
	if !tiers["community"] {
		t.Error("AllSkills() missing community tier entry")
	}
}
