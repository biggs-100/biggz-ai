package doctor

import "testing"

func TestSkillRegistryCheck(t *testing.T) {
	cases := []struct {
		poll, watch bool
		want        Status
	}{
		{true, false, StatusWarn},
		{false, true, StatusPass},
		{false, false, StatusPass},
		{true, true, StatusWarn},
	}
	for _, c := range cases {
		r := NewSkillRegistryCheckWithCustom(func() bool { return c.poll }, func() bool { return c.watch }).Run(t.Context())
		if r.Status != c.want {
			t.Errorf("poll %v watch %v want %v got %v", c.poll, c.watch, c.want, r.Status)
		}
	}
}
