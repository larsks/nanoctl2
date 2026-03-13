package main

import (
	"encoding/json"
	"os"
	"testing"
)

// makeTestScene returns a Scene with non-default values for thorough round-trip testing.
func makeTestScene() *Scene {
	s := &Scene{
		GlobalMidiCh: 3,  // 0-based, so ch 4
		ControlMode:  2,  // "dp"
		LEDMode:      1,  // "external"
		TransportCh:  16, // "global"
	}
	for i := range s.Groups {
		s.Groups[i] = Group{
			MidiCh:        i % 17, // mix of channels and global
			SliderEnabled: i%2 == 0,
			SliderCC:      i * 3,
			SliderMin:     i,
			SliderMax:     127 - i,
			KnobEnabled:   i%2 != 0,
			KnobCC:        i*3 + 1,
			KnobMin:       i + 1,
			KnobMax:       126 - i,
			Solo:          ButtonConfig{Assign: 1, Behavior: 0, CC: i * 4, OffVal: 0, OnVal: 127},
			Mute:          ButtonConfig{Assign: 2, Behavior: 1, CC: i*4 + 1, OffVal: 10, OnVal: 100},
			Rec:           ButtonConfig{Assign: 0, Behavior: 0, CC: 0, OffVal: 0, OnVal: 0},
		}
	}
	for i := range s.Transport {
		s.Transport[i] = ButtonConfig{
			Assign:   i % 3,
			Behavior: i % 2,
			CC:       i * 5,
			OffVal:   i,
			OnVal:    127 - i,
		}
	}
	return s
}

func TestSceneRoundTrip(t *testing.T) {
	original := makeTestScene()
	j := sceneToJSON(original)
	got, err := sceneFromJSON(j)
	if err != nil {
		t.Fatalf("sceneFromJSON error: %v", err)
	}

	if got.GlobalMidiCh != original.GlobalMidiCh {
		t.Errorf("GlobalMidiCh: got %d, want %d", got.GlobalMidiCh, original.GlobalMidiCh)
	}
	if got.ControlMode != original.ControlMode {
		t.Errorf("ControlMode: got %d, want %d", got.ControlMode, original.ControlMode)
	}
	if got.LEDMode != original.LEDMode {
		t.Errorf("LEDMode: got %d, want %d", got.LEDMode, original.LEDMode)
	}
	if got.TransportCh != original.TransportCh {
		t.Errorf("TransportCh: got %d, want %d", got.TransportCh, original.TransportCh)
	}
	for i := range original.Groups {
		og := original.Groups[i]
		gg := got.Groups[i]
		if og != gg {
			t.Errorf("Group[%d]: got %+v, want %+v", i, gg, og)
		}
	}
	for i := range original.Transport {
		if original.Transport[i] != got.Transport[i] {
			t.Errorf("Transport[%d]: got %+v, want %+v", i, got.Transport[i], original.Transport[i])
		}
	}
}

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
		s := &Scene{ControlMode: c.idx}
		j := sceneToJSON(s)
		if j.ControlMode != c.want {
			t.Errorf("ControlMode %d: got %q, want %q", c.idx, j.ControlMode, c.want)
		}
		// round-trip
		got, err := sceneFromJSON(j)
		if err != nil {
			t.Fatalf("sceneFromJSON error for ControlMode %q: %v", c.want, err)
		}
		if got.ControlMode != c.idx {
			t.Errorf("ControlMode round-trip %q: got %d, want %d", c.want, got.ControlMode, c.idx)
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
		s := &Scene{LEDMode: c.idx}
		j := sceneToJSON(s)
		if j.LEDMode != c.want {
			t.Errorf("LEDMode %d: got %q, want %q", c.idx, j.LEDMode, c.want)
		}
		got, err := sceneFromJSON(j)
		if err != nil {
			t.Fatalf("sceneFromJSON error for LEDMode %q: %v", c.want, err)
		}
		if got.LEDMode != c.idx {
			t.Errorf("LEDMode round-trip %q: got %d, want %d", c.want, got.LEDMode, c.idx)
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
		b := ButtonConfig{Assign: c.idx}
		bj := buttonToJSON(b)
		if bj.Assign != c.want {
			t.Errorf("Assign %d: got %q, want %q", c.idx, bj.Assign, c.want)
		}
		got, err := buttonFromJSON(bj)
		if err != nil {
			t.Fatalf("buttonFromJSON error for Assign %q: %v", c.want, err)
		}
		if got.Assign != c.idx {
			t.Errorf("Assign round-trip %q: got %d, want %d", c.want, got.Assign, c.idx)
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
		b := ButtonConfig{Behavior: c.idx}
		bj := buttonToJSON(b)
		if bj.Behavior != c.want {
			t.Errorf("Behavior %d: got %q, want %q", c.idx, bj.Behavior, c.want)
		}
		got, err := buttonFromJSON(bj)
		if err != nil {
			t.Fatalf("buttonFromJSON error for Behavior %q: %v", c.want, err)
		}
		if got.Behavior != c.idx {
			t.Errorf("Behavior round-trip %q: got %d, want %d", c.want, got.Behavior, c.idx)
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
		got := chToString(c.internal)
		if got != c.str {
			t.Errorf("chToString(%d): got %q, want %q", c.internal, got, c.str)
		}
		back, err := chFromString(c.str)
		if err != nil {
			t.Fatalf("chFromString(%q) error: %v", c.str, err)
		}
		if back != c.internal {
			t.Errorf("chFromString(%q): got %d, want %d", c.str, back, c.internal)
		}
	}
}

func TestSceneFromJSONErrors(t *testing.T) {
	validBase := func() SceneJSON {
		s := makeTestScene()
		return sceneToJSON(s)
	}

	t.Run("invalid control_mode", func(t *testing.T) {
		j := validBase()
		j.ControlMode = "bogus"
		if _, err := sceneFromJSON(j); err == nil {
			t.Error("expected error for invalid control_mode")
		}
	})

	t.Run("invalid led_mode", func(t *testing.T) {
		j := validBase()
		j.LEDMode = "neon"
		if _, err := sceneFromJSON(j); err == nil {
			t.Error("expected error for invalid led_mode")
		}
	})

	t.Run("invalid global_midi_ch low", func(t *testing.T) {
		j := validBase()
		j.GlobalMidiCh = 0
		if _, err := sceneFromJSON(j); err == nil {
			t.Error("expected error for global_midi_ch=0")
		}
	})

	t.Run("invalid global_midi_ch high", func(t *testing.T) {
		j := validBase()
		j.GlobalMidiCh = 17
		if _, err := sceneFromJSON(j); err == nil {
			t.Error("expected error for global_midi_ch=17")
		}
	})

	t.Run("invalid transport_ch", func(t *testing.T) {
		j := validBase()
		j.TransportCh = "99"
		if _, err := sceneFromJSON(j); err == nil {
			t.Error("expected error for invalid transport_ch")
		}
	})

	t.Run("invalid group midi_ch", func(t *testing.T) {
		j := validBase()
		j.Groups[0].MidiCh = "bad"
		if _, err := sceneFromJSON(j); err == nil {
			t.Error("expected error for invalid group midi_ch")
		}
	})

	t.Run("invalid button assign", func(t *testing.T) {
		j := validBase()
		j.Groups[0].Solo.Assign = "piano"
		if _, err := sceneFromJSON(j); err == nil {
			t.Error("expected error for invalid assign")
		}
	})

	t.Run("invalid transport button behavior", func(t *testing.T) {
		j := validBase()
		j.Transport[0].Behavior = "latch"
		if _, err := sceneFromJSON(j); err == nil {
			t.Error("expected error for invalid behavior")
		}
	})
}

func TestLoadSceneFromFile(t *testing.T) {
	original := makeTestScene()
	data, err := marshalSceneJSON(original)
	if err != nil {
		t.Fatalf("marshalSceneJSON error: %v", err)
	}

	f, err := os.CreateTemp("", "scene-*.json")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	f.Close()

	got, err := loadSceneFromFile(f.Name())
	if err != nil {
		t.Fatalf("loadSceneFromFile error: %v", err)
	}
	if got.GlobalMidiCh != original.GlobalMidiCh {
		t.Errorf("GlobalMidiCh: got %d, want %d", got.GlobalMidiCh, original.GlobalMidiCh)
	}
	if got.ControlMode != original.ControlMode {
		t.Errorf("ControlMode: got %d, want %d", got.ControlMode, original.ControlMode)
	}
	for i := range original.Groups {
		if original.Groups[i] != got.Groups[i] {
			t.Errorf("Group[%d] mismatch", i)
		}
	}
}

func TestLoadSceneFromFileErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		if _, err := loadSceneFromFile("/nonexistent/path.json"); err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		f, err := os.CreateTemp("", "bad-*.json")
		if err != nil {
			t.Fatalf("creating temp file: %v", err)
		}
		defer os.Remove(f.Name())
		f.WriteString("{not valid json}")
		f.Close()
		if _, err := loadSceneFromFile(f.Name()); err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("valid json invalid scene", func(t *testing.T) {
		f, err := os.CreateTemp("", "bad-scene-*.json")
		if err != nil {
			t.Fatalf("creating temp file: %v", err)
		}
		defer os.Remove(f.Name())
		bad := SceneJSON{
			GlobalMidiCh: 5,
			ControlMode:  "cc",
			LEDMode:      "badmode",
			TransportCh:  "global",
		}
		// fill groups with valid data
		for i := range bad.Groups {
			bad.Groups[i] = GroupJSON{
				MidiCh: "1",
				Solo:   ButtonJSON{Assign: "cc", Behavior: "momentary"},
				Mute:   ButtonJSON{Assign: "cc", Behavior: "momentary"},
				Rec:    ButtonJSON{Assign: "cc", Behavior: "momentary"},
			}
		}
		for i := range bad.Transport {
			bad.Transport[i] = ButtonJSON{Assign: "cc", Behavior: "momentary"}
		}
		data, _ := json.Marshal(bad)
		f.Write(data)
		f.Close()
		if _, err := loadSceneFromFile(f.Name()); err == nil {
			t.Error("expected error for invalid led_mode in file")
		}
	})
}
