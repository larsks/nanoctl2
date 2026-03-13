package scene

import (
	"fmt"
	"strings"
)

// ButtonConfig holds the configuration for a single button (Solo, Mute, Rec, or Transport).
type ButtonConfig struct {
	Assign   int // 0=NoAssign, 1=CC, 2=Note
	Behavior int // 0=Momentary, 1=Toggle
	CC       int
	OffVal   int
	OnVal    int
}

// Group holds the configuration for one of the 8 nanoKONTROL2 channel strips.
type Group struct {
	MidiCh        int // 0-15=ch1-16, 16=global
	SliderEnabled bool
	SliderCC      int
	SliderMin     int
	SliderMax     int
	KnobEnabled   bool
	KnobCC        int
	KnobMin       int
	KnobMax       int
	Solo          ButtonConfig
	Mute          ButtonConfig
	Rec           ButtonConfig
}

// Scene holds the full nanoKONTROL2 scene configuration.
type Scene struct {
	GlobalMidiCh int
	ControlMode  int // 0=CC,1=Cubase,2=DP,3=Live,4=ProTools,5=SONAR
	LEDMode      int // 0=Internal, 1=External
	Groups       [8]Group
	TransportCh  int
	// Transport button order: PrevTrack, NextTrack, Cycle, MarkerSet,
	// PrevMarker, NextMarker, REW, FF, Stop, Play, Rec
	Transport [11]ButtonConfig
}

// Enum name slices for JSON/CLI use (lowercase, matching CLI flag values).
var (
	ControlModeNames = []string{"cc", "cubase", "dp", "live", "protools", "sonar"}
	LEDModeNames     = []string{"internal", "external"}
	AssignNames      = []string{"none", "cc", "note"}
	BehaviorNames    = []string{"momentary", "toggle"}
)

// sceneDataSize is the number of decoded bytes in a scene dump (388 encoded → 339 decoded).
const sceneDataSize = 339

// Decode7bit decodes KORG 7-bit MIDI encoding: groups of 8 MIDI bytes → 7 data bytes.
// The first byte of each group holds the MSBs of the following 7 bytes.
// Bit j of the MSB byte is the MSB of byte j+1 in the group.
func Decode7bit(data []byte) []byte {
	var result []byte
	for i := 0; i < len(data); {
		chunk := min(8, len(data)-i)
		msb := data[i]
		for j := 0; j < chunk-1; j++ {
			result = append(result, data[i+1+j]|((msb>>j)&1)<<7)
		}
		i += chunk
	}
	return result
}

// Encode7bit encodes data bytes into KORG 7-bit MIDI encoding.
// Groups of 7 data bytes → 8 MIDI bytes (each < 0x80).
func Encode7bit(data []byte) []byte {
	var result []byte
	for i := 0; i < len(data); {
		chunk := min(7, len(data)-i)
		group := data[i : i+chunk]
		msb := byte(0)
		for j, b := range group {
			msb |= (b >> 7) << j
		}
		result = append(result, msb)
		for _, b := range group {
			result = append(result, b&0x7F)
		}
		i += chunk
	}
	return result
}

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func readButton(data []byte, offset int) ButtonConfig {
	return ButtonConfig{
		Assign:   int(data[offset]),
		Behavior: int(data[offset+1]),
		CC:       int(data[offset+2]),
		OffVal:   int(data[offset+3]),
		OnVal:    int(data[offset+4]),
	}
}

func writeButton(data []byte, offset int, b ButtonConfig) {
	data[offset] = byte(b.Assign)
	data[offset+1] = byte(b.Behavior)
	data[offset+2] = byte(b.CC)
	data[offset+3] = byte(b.OffVal)
	data[offset+4] = byte(b.OnVal)
}

// Group layout within the 339-byte decoded scene (31 bytes per group, starting at offset 3):
//
//	[0]    MIDI channel (0-15 or 16=global)
//	[1]    slider enabled
//	[2]    slider assign type (preserved, not exposed)
//	[3]    slider CC
//	[4]    slider min
//	[5]    slider max
//	[6]    (reserved, preserved)
//	[7]    knob enabled
//	[8]    knob assign type (preserved)
//	[9]    knob CC
//	[10]   knob min
//	[11]   knob max
//	[12]   (reserved, preserved)
//	[13..18] Solo: assign, behavior, CC, off, on, (reserved)
//	[19..24] Mute: assign, behavior, CC, off, on, (reserved)
//	[25..30] Rec:  assign, behavior, CC, off, on, (reserved)
func readGroup(data []byte, base int) Group {
	g := data[base:]
	return Group{
		MidiCh:        int(g[0]),
		SliderEnabled: g[1] != 0,
		SliderCC:      int(g[3]),
		SliderMin:     int(g[4]),
		SliderMax:     int(g[5]),
		KnobEnabled:   g[7] != 0,
		KnobCC:        int(g[9]),
		KnobMin:       int(g[10]),
		KnobMax:       int(g[11]),
		Solo:          readButton(data, base+13),
		Mute:          readButton(data, base+19),
		Rec:           readButton(data, base+25),
	}
}

func writeGroup(data []byte, base int, grp Group) {
	data[base+0] = byte(grp.MidiCh)
	data[base+1] = boolByte(grp.SliderEnabled)
	data[base+3] = byte(grp.SliderCC)
	data[base+4] = byte(grp.SliderMin)
	data[base+5] = byte(grp.SliderMax)
	data[base+7] = boolByte(grp.KnobEnabled)
	data[base+9] = byte(grp.KnobCC)
	data[base+10] = byte(grp.KnobMin)
	data[base+11] = byte(grp.KnobMax)
	writeButton(data, base+13, grp.Solo)
	writeButton(data, base+19, grp.Mute)
	writeButton(data, base+25, grp.Rec)
}

// DecodeScene parses a 339-byte decoded scene data buffer into a Scene struct.
// Returns nil if buf is too short.
func DecodeScene(buf []byte) *Scene {
	if len(buf) < sceneDataSize {
		return nil
	}
	s := &Scene{
		GlobalMidiCh: int(buf[0]),
		ControlMode:  int(buf[1]),
		LEDMode:      int(buf[2]),
		TransportCh:  int(buf[251]),
	}
	for i := range 8 {
		s.Groups[i] = readGroup(buf, 3+i*31)
	}
	for i := range 11 {
		s.Transport[i] = readButton(buf, 252+i*6)
	}
	return s
}

// ApplySceneToBytes writes a Scene struct's known fields back into a copy of the raw
// 339-byte decoded buffer, preserving any unknown/reserved bytes from the original.
func ApplySceneToBytes(original []byte, s *Scene) []byte {
	data := make([]byte, len(original))
	copy(data, original)
	data[0] = byte(s.GlobalMidiCh)
	data[1] = byte(s.ControlMode)
	data[2] = byte(s.LEDMode)
	for i := range 8 {
		writeGroup(data, 3+i*31, s.Groups[i])
	}
	data[251] = byte(s.TransportCh)
	for i := range 11 {
		writeButton(data, 252+i*6, s.Transport[i])
	}
	return data
}

// EffectiveMidiCh returns the effective MIDI channel (0-15) for a button,
// resolving "global" (16) using the scene's global channel.
func EffectiveMidiCh(ch, globalCh int) int {
	if ch == 16 {
		return globalCh
	}
	return ch
}

// ChToString converts an internal channel value (0-15 or 16=global) to a string.
func ChToString(ch int) string {
	if ch == 16 {
		return "global"
	}
	return fmt.Sprintf("%d", ch+1)
}

// ChFromString parses a channel string ("1"-"16" or "global") to an internal value.
func ChFromString(s string) (int, error) {
	if strings.ToLower(s) == "global" {
		return 16, nil
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || n < 1 || n > 16 {
		return 0, fmt.Errorf("invalid channel %q: must be 1-16 or \"global\"", s)
	}
	return n - 1, nil
}

// EnumToString returns the string name for an int enum index, or an error string.
func EnumToString(idx int, names []string) string {
	if idx >= 0 && idx < len(names) {
		return names[idx]
	}
	return fmt.Sprintf("unknown(%d)", idx)
}

// EnumFromString returns the index for a string enum name, or an error.
func EnumFromString(val string, names []string, field string) (int, error) {
	v := strings.ToLower(val)
	for i, name := range names {
		if v == name {
			return i, nil
		}
	}
	return 0, fmt.Errorf("invalid %s %q: must be one of %v", field, val, names)
}
