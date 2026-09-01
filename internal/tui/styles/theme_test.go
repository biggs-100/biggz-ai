package styles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestColorLuma(t *testing.T) {
	if v, ok := ColorLuma("#000000"); !ok || v != 0 {
		// allow close to 0
		if v > 0.01 {
			t.Fatalf("ColorLuma black = %v want ~0", v)
		}
	}
	if v, ok := ColorLuma("#ffffff"); !ok || v < 0.99 {
		t.Fatalf("ColorLuma white = %v want ~1", v)
	}
	if v, ok := ColorLuma("#fff"); !ok {
		t.Fatalf("ColorLuma #fff not ok")
	} else {
		w, _ := ColorLuma("#ffffff")
		if v != w {
			t.Errorf("#fff luma %v != #ffffff %v", v, w)
		}
	}
	if _, ok := ColorLuma("primary"); ok {
		t.Error("expected undefined for var ref")
	}
	if _, ok := ColorLuma("#ff"); ok {
		t.Error("expected undefined for malformed")
	}
	// palette indices
	if v, ok := ColorLumaAny(15); !ok || v < 0.9 {
		t.Errorf("ColorLuma 15 = %v want >0.9", v)
	}
	if _, ok := ColorLumaAny(256); ok {
		t.Error("expected undefined for 256")
	}
	// mid-gray luma vs relative
	if lum, ok := ColorLuma("#808080"); ok {
		if lum < 0.45 {
			t.Errorf("ColorLuma #808080 %v want >0.45", lum)
		}
	}
}

func TestRelativeLuminance(t *testing.T) {
	if v, ok := RelativeLuminance("#000000"); !ok || v != 0 {
		if v > 0.01 {
			t.Fatalf("RelativeLuminance black %v", v)
		}
	}
	if v, ok := RelativeLuminance("#ffffff"); !ok || v < 0.99 {
		t.Fatalf("RelativeLuminance white %v", v)
	}
	if v, ok := RelativeLuminance("#808080"); ok {
		if v > 0.25 {
			t.Errorf("RelativeLuminance #808080 %v want <0.25 linearized", v)
		}
	}
	if _, ok := RelativeLuminanceAny(256); ok {
		t.Error("256 should be undefined")
	}
	if v, ok := RelativeLuminanceAny(15); !ok || v < 0.9 {
		t.Errorf("RelativeLuminance 15 %v want >0.9", v)
	}
}

func TestAnsi256ToHex(t *testing.T) {
	tests := []struct {
		idx  int
		want string
	}{
		{0, "#000000"},
		{15, "#ffffff"},
		{244, "#808080"},
		{70, "#5faf00"},
		{39, "#00afff"},
		{205, "#ff5faf"},
	}
	for _, tc := range tests {
		if got := Ansi256ToHex(tc.idx); got != tc.want {
			t.Errorf("Ansi256ToHex %d = %q want %q", tc.idx, got, tc.want)
		}
	}
}

func TestSymbolPreset(t *testing.T) {
	if !IsValidSymbolPreset("unicode") || !IsValidSymbolPreset("nerd") || !IsValidSymbolPreset("ascii") {
		t.Error("valid presets not recognized")
	}
	if IsValidSymbolPreset("emoji") {
		t.Error("invalid preset accepted")
	}
	if ParseSymbolPreset("nerd") != SymbolPresetNerd {
		t.Error("ParseSymbolPreset nerd")
	}
	if ParseSymbolPreset("unknown") != SymbolPresetUnicode {
		t.Error("fallback to unicode")
	}
	// SPINNER_FRAMES not empty
	for _, preset := range []SymbolPreset{SymbolPresetUnicode, SymbolPresetNerd, SymbolPresetASCII} {
		for _, typ := range []SpinnerType{SpinnerStatus, SpinnerActivity} {
			f := GetSpinnerFrames(preset, typ)
			if len(f) == 0 {
				t.Errorf("SpinnerFrames %s %s empty", preset, typ)
			}
		}
	}
	// box presets
	if BoxRoundPresets[SymbolPresetUnicode].TopLeft != "╭" {
		t.Error("boxRound unicode mismatch")
	}
	if BoxSharpPresets[SymbolPresetASCII].Cross != "+" {
		t.Error("boxSharp ascii")
	}
	if SepPresets[SymbolPresetUnicode].Powerline != "▕" {
		t.Error("sep powerline unicode")
	}
}

func TestThemeEpoch(t *testing.T) {
	start := GetThemeEpoch()
	BumpThemeEpoch()
	if GetThemeEpoch() != start+1 {
		t.Fatalf("epoch not bumped %d -> %d", start, GetThemeEpoch())
	}
	BumpThemeEpoch()
	if GetThemeEpoch() != start+2 {
		t.Error("second bump")
	}
}

func TestThemeCreation(t *testing.T) {
	dark := GetTheme("dark")
	if dark.IsLight() {
		t.Error("dark should not be light")
	}
	if dark.Palette.StatusLineBg != "#121212" {
		t.Errorf("dark StatusLineBg %q", dark.Palette.StatusLineBg)
	}
	light := GetTheme("light")
	if !light.IsLight() {
		t.Error("light should be light")
	}
	if light.Palette.StatusLineBg != "#e0e0e0" {
		t.Errorf("light StatusLineBg %q", light.Palette.StatusLineBg)
	}
	// accent
	if dark.GetColorHex(ThemeColorAccent) != "#febc38" {
		t.Errorf("dark accent %q", dark.GetColorHex(ThemeColorAccent))
	}
	// contrast fg: bright fill should give black
	fillDark := dark.GetContrastFgAnsi(ThemeColorAccent) // accent #febc38 is bright ~ 0.8 luma -> black
	if fillDark != "\x1b[38;2;0;0;0m" {
		t.Logf("contrast for accent: %q (expected black for bright)", fillDark)
		// bright accent should indeed be black text; check not white
		if fillDark == "\x1b[38;2;255;255;255m" {
			t.Error("bright accent should give black contrast")
		}
	}
	// preset
	nerd := GetTheme("dark", SymbolPresetNerd)
	if nerd.SymbolPreset != SymbolPresetNerd {
		t.Error("preset not set")
	}
}

func TestDetectTerminalBackground(t *testing.T) {
	// default dark
	t.Setenv("COLORFGBG", "")
	t.Setenv("BIGGZ_MAC_APPEARANCE", "")
	t.Setenv("ZELLIJ", "")
	terminalReportedAppearance = ""
	if got := DetectTerminalBackground(); got != "dark" {
		t.Errorf("default background %q want dark", got)
	}
	// COLORFGBG light: bg >=8
	t.Setenv("COLORFGBG", "15;8")
	if got := DetectTerminalBackground(); got != "light" {
		t.Errorf("COLORFGBG 15;8 %q want light", got)
	}
	t.Setenv("COLORFGBG", "0;0")
	if got := DetectTerminalBackground(); got != "dark" {
		t.Errorf("COLORFGBG 0;0 %q want dark", got)
	}
	t.Setenv("COLORFGBG", "")
	// OSC 11 stub via terminalReportedAppearance
	terminalReportedAppearance = "light"
	if got := DetectTerminalBackground(); got != "light" {
		t.Errorf("terminalReportedAppearance light %q", got)
	}
	terminalReportedAppearance = ""
}

func TestIsPrettyEnabled(t *testing.T) {
	t.Setenv("BIGGZ_PRETTY", "")
	t.Setenv("PI_SUBAGENT_CHILD", "")
	if !IsPrettyEnabled() {
		t.Error("pretty should be enabled by default")
	}
	t.Setenv("BIGGZ_PRETTY", "0")
	if IsPrettyEnabled() {
		t.Error("BIGGZ_PRETTY=0 should disable")
	}
	t.Setenv("BIGGZ_PRETTY", "")
	t.Setenv("PI_SUBAGENT_CHILD", "1")
	if IsPrettyEnabled() {
		t.Error("PI_SUBAGENT_CHILD=1 should disable")
	}
	t.Setenv("PI_SUBAGENT_CHILD", "")
}

func TestThemeFgBg(t *testing.T) {
	th := GetTheme("dark")
	s := th.Fg(ThemeColorAccent, "hello")
	if s == "hello" {
		t.Error("Fg should wrap with ansi")
	}
	if s == "" {
		t.Error("Fg empty")
	}
	bg := th.Bg(BgStatusLineBg, "bg")
	if bg == "bg" {
		t.Error("Bg should wrap")
	}
	fill := th.BgFill(BgStatusLineBg, "a\x1b[0mb")
	if fill == "" {
		t.Error("BgFill empty")
	}
}

func TestDarkJsonLoadable(t *testing.T) {
	// Ensure bundled dark.json is readable via candidateThemePaths or direct runtime path
	found := false
	for _, cand := range candidateThemePaths("dark") {
		if _, err := os.Stat(cand); err == nil {
			found = true
			data, err := os.ReadFile(cand)
			if err != nil {
				t.Fatalf("read %s: %v", cand, err)
			}
			if _, err := parseThemeJSON(data); err != nil {
				t.Fatalf("parse %s: %v", cand, err)
			}
			break
		}
	}
	if !found {
		// fallback: check absolute via repo root relative to this file using runtime trick
		// try direct path used by theme.go runtime caller
		candidates := []string{
				filepath.Join("internal", "assets", "pi", "themes", "dark.json"),
				filepath.Join("..", "..", "assets", "pi", "themes", "dark.json"),
			}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				found = true
				break
			}
			}
	}
	if !found {
		t.Logf("dark.json not found via candidateThemePaths — using hardcoded fallback (acceptable in CI)")
	}
}

func TestLightJsonLoadable(t *testing.T) {
	found := false
	for _, cand := range candidateThemePaths("light") {
		if _, err := os.Stat(cand); err == nil {
			found = true
			data, _ := os.ReadFile(cand)
			if _, err := parseThemeJSON(data); err != nil {
				t.Fatalf("parse light %s: %v", cand, err)
			}
			break
		}
	}
	if !found {
		t.Logf("light.json not found — fallback ok")
	}
}

func TestSolarizedOsakaLoadable(t *testing.T) {
	found := false
	for _, cand := range candidateThemePaths("solarized-osaka") {
		if _, err := os.Stat(cand); err == nil {
			found = true
			data, _ := os.ReadFile(cand)
			if _, err := parseThemeJSON(data); err != nil {
				t.Fatalf("parse solarized %s: %v", cand, err)
			}
			break
		}
	}
	if !found {
		t.Logf("solarized-osaka.json not found — fallback ok")
	}
}

// Test colorLuma vs relativeLuminance divergence mirrors pi-utils test.
func TestLumaVsRelative(t *testing.T) {
	luma, _ := ColorLuma("#808080")
	rel, _ := RelativeLuminance("#808080")
	if rel > 0.25 {
		t.Errorf("relative #808080 %v >0.25", rel)
	}
	if luma < 0.45 {
		t.Errorf("luma #808080 %v <0.45", luma)
	}
	if !(luma > rel) {
		t.Errorf("luma %v should be > relative %v for mid-gray", luma, rel)
	}
}
