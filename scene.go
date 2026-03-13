package main

import "fmt"

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

// sceneDataSize is the number of decoded bytes in a scene dump (388 encoded → 339 decoded).
const sceneDataSize = 339

// decode7bit decodes KORG 7-bit MIDI encoding: groups of 8 MIDI bytes → 7 data bytes.
// The first byte of each group holds the MSBs of the following 7 bytes.
// Bit j of the MSB byte is the MSB of byte j+1 in the group.
func decode7bit(data []byte) []byte {
	var result []byte
	for i := 0; i < len(data); {
		chunk := 8
		if len(data)-i < 8 {
			chunk = len(data) - i
		}
		msb := data[i]
		for j := 0; j < chunk-1; j++ {
			result = append(result, data[i+1+j]|((msb>>j)&1)<<7)
		}
		i += chunk
	}
	return result
}

// encode7bit encodes data bytes into KORG 7-bit MIDI encoding.
// Groups of 7 data bytes → 8 MIDI bytes (each < 0x80).
func encode7bit(data []byte) []byte {
	var result []byte
	for i := 0; i < len(data); {
		chunk := 7
		if len(data)-i < 7 {
			chunk = len(data) - i
		}
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

// decodeScene parses a 339-byte decoded scene data buffer into a Scene struct.
func decodeScene(data []byte) (*Scene, error) {
	if len(data) < sceneDataSize {
		return nil, fmt.Errorf("expected %d bytes of scene data, got %d", sceneDataSize, len(data))
	}
	s := &Scene{
		GlobalMidiCh: int(data[0]),
		ControlMode:  int(data[1]),
		LEDMode:      int(data[2]),
		TransportCh:  int(data[251]),
	}
	for i := 0; i < 8; i++ {
		s.Groups[i] = readGroup(data, 3+i*31)
	}
	for i := 0; i < 11; i++ {
		s.Transport[i] = readButton(data, 252+i*6)
	}
	return s, nil
}

// applySceneToBytes writes a Scene struct's known fields back into a copy of the raw
// 339-byte decoded buffer, preserving any unknown/reserved bytes from the original.
func applySceneToBytes(original []byte, s *Scene) []byte {
	data := make([]byte, len(original))
	copy(data, original)
	data[0] = byte(s.GlobalMidiCh)
	data[1] = byte(s.ControlMode)
	data[2] = byte(s.LEDMode)
	for i := 0; i < 8; i++ {
		writeGroup(data, 3+i*31, s.Groups[i])
	}
	data[251] = byte(s.TransportCh)
	for i := 0; i < 11; i++ {
		writeButton(data, 252+i*6, s.Transport[i])
	}
	return data
}

// effectiveMidiCh returns the effective MIDI channel (0-15) for a button,
// resolving "global" (16) using the scene's global channel.
func effectiveMidiCh(ch, globalCh int) int {
	if ch == 16 {
		return globalCh
	}
	return ch
}
