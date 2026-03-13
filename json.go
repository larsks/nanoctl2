package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// JSON-friendly string enum tables (lowercase, matching CLI flag values).
var (
	jsonControlModeNames = []string{"cc", "cubase", "dp", "live", "protools", "sonar"}
	jsonLEDModeNames     = []string{"internal", "external"}
	jsonAssignNames      = []string{"none", "cc", "note"}
	jsonBehaviorNames    = []string{"momentary", "toggle"}
)

// SceneJSON is the JSON representation of a Scene.
type SceneJSON struct {
	GlobalMidiCh int            `json:"global_midi_ch"` // 1-16
	ControlMode  string         `json:"control_mode"`   // "cc"|"cubase"|"dp"|"live"|"protools"|"sonar"
	LEDMode      string         `json:"led_mode"`       // "internal"|"external"
	Groups       [8]GroupJSON   `json:"groups"`
	TransportCh  string         `json:"transport_ch"` // "1"-"16" or "global"
	Transport    [11]ButtonJSON `json:"transport"`
}

// GroupJSON is the JSON representation of a Group.
type GroupJSON struct {
	MidiCh        string     `json:"midi_ch"` // "1"-"16" or "global"
	SliderEnabled bool       `json:"slider_enabled"`
	SliderCC      int        `json:"slider_cc"`
	SliderMin     int        `json:"slider_min"`
	SliderMax     int        `json:"slider_max"`
	KnobEnabled   bool       `json:"knob_enabled"`
	KnobCC        int        `json:"knob_cc"`
	KnobMin       int        `json:"knob_min"`
	KnobMax       int        `json:"knob_max"`
	Solo          ButtonJSON `json:"solo"`
	Mute          ButtonJSON `json:"mute"`
	Rec           ButtonJSON `json:"rec"`
}

// ButtonJSON is the JSON representation of a ButtonConfig.
type ButtonJSON struct {
	Assign   string `json:"assign"`   // "none"|"cc"|"note"
	Behavior string `json:"behavior"` // "momentary"|"toggle"
	CC       int    `json:"cc"`
	OffVal   int    `json:"off_val"`
	OnVal    int    `json:"on_val"`
}

// chToString converts an internal channel value (0-15 or 16=global) to a JSON string.
func chToString(ch int) string {
	if ch == 16 {
		return "global"
	}
	return fmt.Sprintf("%d", ch+1)
}

// chFromString parses a channel string ("1"-"16" or "global") to an internal value.
func chFromString(s string) (int, error) {
	if strings.ToLower(s) == "global" {
		return 16, nil
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || n < 1 || n > 16 {
		return 0, fmt.Errorf("invalid channel %q: must be 1-16 or \"global\"", s)
	}
	return n - 1, nil
}

// enumToString returns the string name for an int enum index, or an error string.
func enumToString(idx int, names []string) string {
	if idx >= 0 && idx < len(names) {
		return names[idx]
	}
	return fmt.Sprintf("unknown(%d)", idx)
}

// enumFromString returns the index for a string enum name, or an error.
func enumFromString(val string, names []string, field string) (int, error) {
	v := strings.ToLower(val)
	for i, name := range names {
		if v == name {
			return i, nil
		}
	}
	return 0, fmt.Errorf("invalid %s %q: must be one of %v", field, val, names)
}

func buttonToJSON(b ButtonConfig) ButtonJSON {
	return ButtonJSON{
		Assign:   enumToString(b.Assign, jsonAssignNames),
		Behavior: enumToString(b.Behavior, jsonBehaviorNames),
		CC:       b.CC,
		OffVal:   b.OffVal,
		OnVal:    b.OnVal,
	}
}

func buttonFromJSON(j ButtonJSON) (ButtonConfig, error) {
	assign, err := enumFromString(j.Assign, jsonAssignNames, "assign")
	if err != nil {
		return ButtonConfig{}, err
	}
	behavior, err := enumFromString(j.Behavior, jsonBehaviorNames, "behavior")
	if err != nil {
		return ButtonConfig{}, err
	}
	return ButtonConfig{
		Assign:   assign,
		Behavior: behavior,
		CC:       j.CC,
		OffVal:   j.OffVal,
		OnVal:    j.OnVal,
	}, nil
}

func groupToJSON(g Group) GroupJSON {
	return GroupJSON{
		MidiCh:        chToString(g.MidiCh),
		SliderEnabled: g.SliderEnabled,
		SliderCC:      g.SliderCC,
		SliderMin:     g.SliderMin,
		SliderMax:     g.SliderMax,
		KnobEnabled:   g.KnobEnabled,
		KnobCC:        g.KnobCC,
		KnobMin:       g.KnobMin,
		KnobMax:       g.KnobMax,
		Solo:          buttonToJSON(g.Solo),
		Mute:          buttonToJSON(g.Mute),
		Rec:           buttonToJSON(g.Rec),
	}
}

func groupFromJSON(j GroupJSON) (Group, error) {
	ch, err := chFromString(j.MidiCh)
	if err != nil {
		return Group{}, fmt.Errorf("midi_ch: %w", err)
	}
	solo, err := buttonFromJSON(j.Solo)
	if err != nil {
		return Group{}, fmt.Errorf("solo: %w", err)
	}
	mute, err := buttonFromJSON(j.Mute)
	if err != nil {
		return Group{}, fmt.Errorf("mute: %w", err)
	}
	rec, err := buttonFromJSON(j.Rec)
	if err != nil {
		return Group{}, fmt.Errorf("rec: %w", err)
	}
	return Group{
		MidiCh:        ch,
		SliderEnabled: j.SliderEnabled,
		SliderCC:      j.SliderCC,
		SliderMin:     j.SliderMin,
		SliderMax:     j.SliderMax,
		KnobEnabled:   j.KnobEnabled,
		KnobCC:        j.KnobCC,
		KnobMin:       j.KnobMin,
		KnobMax:       j.KnobMax,
		Solo:          solo,
		Mute:          mute,
		Rec:           rec,
	}, nil
}

// sceneToJSON converts a Scene to its JSON representation.
func sceneToJSON(s *Scene) SceneJSON {
	j := SceneJSON{
		GlobalMidiCh: s.GlobalMidiCh + 1,
		ControlMode:  enumToString(s.ControlMode, jsonControlModeNames),
		LEDMode:      enumToString(s.LEDMode, jsonLEDModeNames),
		TransportCh:  chToString(s.TransportCh),
	}
	for i, g := range s.Groups {
		j.Groups[i] = groupToJSON(g)
	}
	for i, b := range s.Transport {
		j.Transport[i] = buttonToJSON(b)
	}
	return j
}

// sceneFromJSON converts a SceneJSON to a Scene, returning an error on invalid values.
func sceneFromJSON(j SceneJSON) (*Scene, error) {
	if j.GlobalMidiCh < 1 || j.GlobalMidiCh > 16 {
		return nil, fmt.Errorf("global_midi_ch %d out of range 1-16", j.GlobalMidiCh)
	}
	controlMode, err := enumFromString(j.ControlMode, jsonControlModeNames, "control_mode")
	if err != nil {
		return nil, err
	}
	ledMode, err := enumFromString(j.LEDMode, jsonLEDModeNames, "led_mode")
	if err != nil {
		return nil, err
	}
	transportCh, err := chFromString(j.TransportCh)
	if err != nil {
		return nil, fmt.Errorf("transport_ch: %w", err)
	}

	s := &Scene{
		GlobalMidiCh: j.GlobalMidiCh - 1,
		ControlMode:  controlMode,
		LEDMode:      ledMode,
		TransportCh:  transportCh,
	}
	for i, gj := range j.Groups {
		g, err := groupFromJSON(gj)
		if err != nil {
			return nil, fmt.Errorf("group %d: %w", i+1, err)
		}
		s.Groups[i] = g
	}
	for i, bj := range j.Transport {
		b, err := buttonFromJSON(bj)
		if err != nil {
			return nil, fmt.Errorf("transport[%d]: %w", i, err)
		}
		s.Transport[i] = b
	}
	return s, nil
}

// marshalSceneJSON encodes a Scene as indented JSON.
func marshalSceneJSON(s *Scene) ([]byte, error) {
	return json.MarshalIndent(sceneToJSON(s), "", "  ")
}

// loadSceneFromFile reads a JSON file and returns the parsed Scene.
func loadSceneFromFile(path string) (*Scene, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	var j SceneJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	return sceneFromJSON(j)
}
