package config

import (
	"encoding/json"
	"fmt"
	"os"

	"nanoctl/internal/scene"
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

func buttonToJSON(b scene.ButtonConfig) ButtonJSON {
	return ButtonJSON{
		Assign:   scene.EnumToString(b.Assign, scene.AssignNames),
		Behavior: scene.EnumToString(b.Behavior, scene.BehaviorNames),
		CC:       b.CC,
		OffVal:   b.OffVal,
		OnVal:    b.OnVal,
	}
}

func buttonFromJSON(j ButtonJSON) (scene.ButtonConfig, error) {
	assign, err := scene.EnumFromString(j.Assign, scene.AssignNames, "assign")
	if err != nil {
		return scene.ButtonConfig{}, err
	}
	behavior, err := scene.EnumFromString(j.Behavior, scene.BehaviorNames, "behavior")
	if err != nil {
		return scene.ButtonConfig{}, err
	}
	return scene.ButtonConfig{
		Assign:   assign,
		Behavior: behavior,
		CC:       j.CC,
		OffVal:   j.OffVal,
		OnVal:    j.OnVal,
	}, nil
}

func groupToJSON(g scene.Group) GroupJSON {
	return GroupJSON{
		MidiCh:        scene.ChToString(g.MidiCh),
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

func groupFromJSON(j GroupJSON) (scene.Group, error) {
	ch, err := scene.ChFromString(j.MidiCh)
	if err != nil {
		return scene.Group{}, fmt.Errorf("midi_ch: %w", err)
	}
	solo, err := buttonFromJSON(j.Solo)
	if err != nil {
		return scene.Group{}, fmt.Errorf("solo: %w", err)
	}
	mute, err := buttonFromJSON(j.Mute)
	if err != nil {
		return scene.Group{}, fmt.Errorf("mute: %w", err)
	}
	rec, err := buttonFromJSON(j.Rec)
	if err != nil {
		return scene.Group{}, fmt.Errorf("rec: %w", err)
	}
	return scene.Group{
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
func sceneToJSON(s *scene.Scene) SceneJSON {
	j := SceneJSON{
		GlobalMidiCh: s.GlobalMidiCh + 1,
		ControlMode:  scene.EnumToString(s.ControlMode, scene.ControlModeNames),
		LEDMode:      scene.EnumToString(s.LEDMode, scene.LEDModeNames),
		TransportCh:  scene.ChToString(s.TransportCh),
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
func sceneFromJSON(j SceneJSON) (*scene.Scene, error) {
	if j.GlobalMidiCh < 1 || j.GlobalMidiCh > 16 {
		return nil, fmt.Errorf("global_midi_ch %d out of range 1-16", j.GlobalMidiCh)
	}
	controlMode, err := scene.EnumFromString(j.ControlMode, scene.ControlModeNames, "control_mode")
	if err != nil {
		return nil, err
	}
	ledMode, err := scene.EnumFromString(j.LEDMode, scene.LEDModeNames, "led_mode")
	if err != nil {
		return nil, err
	}
	transportCh, err := scene.ChFromString(j.TransportCh)
	if err != nil {
		return nil, fmt.Errorf("transport_ch: %w", err)
	}

	s := &scene.Scene{
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

// MarshalSceneJSON encodes a Scene as indented JSON.
func MarshalSceneJSON(s *scene.Scene) ([]byte, error) {
	return json.MarshalIndent(sceneToJSON(s), "", "  ")
}

// LoadSceneFromFile reads a JSON file and returns the parsed Scene.
func LoadSceneFromFile(path string) (*scene.Scene, error) {
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
