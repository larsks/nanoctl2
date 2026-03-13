package scene

import (
	"testing"
)

func TestControlModeStrings(t *testing.T) {
	cases := []struct {
		idx  int
		want string
	}{
		{0, "cc"},
		{1, "cubase"},
		{2, "dp"},
		{3, "live"},
		{4, "protools"},
		{5, "sonar"},
	}
	for _, c := range cases {
		got := EnumToString(c.idx, ControlModeNames)
		if got != c.want {
			t.Errorf("ControlMode %d: got %q, want %q", c.idx, got, c.want)
		}
		back, err := EnumFromString(c.want, ControlModeNames, "control_mode")
		if err != nil {
			t.Fatalf("EnumFromString error for ControlMode %q: %v", c.want, err)
		}
		if back != c.idx {
			t.Errorf("ControlMode round-trip %q: got %d, want %d", c.want, back, c.idx)
		}
	}
}

func TestLEDModeStrings(t *testing.T) {
	cases := []struct {
		idx  int
		want string
	}{
		{0, "internal"},
		{1, "external"},
	}
	for _, c := range cases {
		got := EnumToString(c.idx, LEDModeNames)
		if got != c.want {
			t.Errorf("LEDMode %d: got %q, want %q", c.idx, got, c.want)
		}
		back, err := EnumFromString(c.want, LEDModeNames, "led_mode")
		if err != nil {
			t.Fatalf("EnumFromString error for LEDMode %q: %v", c.want, err)
		}
		if back != c.idx {
			t.Errorf("LEDMode round-trip %q: got %d, want %d", c.want, back, c.idx)
		}
	}
}

func TestAssignStrings(t *testing.T) {
	cases := []struct {
		idx  int
		want string
	}{
		{0, "none"},
		{1, "cc"},
		{2, "note"},
	}
	for _, c := range cases {
		got := EnumToString(c.idx, AssignNames)
		if got != c.want {
			t.Errorf("Assign %d: got %q, want %q", c.idx, got, c.want)
		}
		back, err := EnumFromString(c.want, AssignNames, "assign")
		if err != nil {
			t.Fatalf("EnumFromString error for Assign %q: %v", c.want, err)
		}
		if back != c.idx {
			t.Errorf("Assign round-trip %q: got %d, want %d", c.want, back, c.idx)
		}
	}
}

func TestBehaviorStrings(t *testing.T) {
	cases := []struct {
		idx  int
		want string
	}{
		{0, "momentary"},
		{1, "toggle"},
	}
	for _, c := range cases {
		got := EnumToString(c.idx, BehaviorNames)
		if got != c.want {
			t.Errorf("Behavior %d: got %q, want %q", c.idx, got, c.want)
		}
		back, err := EnumFromString(c.want, BehaviorNames, "behavior")
		if err != nil {
			t.Fatalf("EnumFromString error for Behavior %q: %v", c.want, err)
		}
		if back != c.idx {
			t.Errorf("Behavior round-trip %q: got %d, want %d", c.want, back, c.idx)
		}
	}
}

func TestChannelFormatting(t *testing.T) {
	cases := []struct {
		internal int
		str      string
	}{
		{0, "1"},
		{14, "15"},
		{15, "16"},
		{16, "global"},
	}
	for _, c := range cases {
		got := ChToString(c.internal)
		if got != c.str {
			t.Errorf("ChToString(%d): got %q, want %q", c.internal, got, c.str)
		}
		back, err := ChFromString(c.str)
		if err != nil {
			t.Fatalf("ChFromString(%q) error: %v", c.str, err)
		}
		if back != c.internal {
			t.Errorf("ChFromString(%q): got %d, want %d", c.str, back, c.internal)
		}
	}
}
