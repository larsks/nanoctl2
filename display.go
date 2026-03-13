package main

import "fmt"

var (
	controlModeNames = []string{"CC", "Cubase", "DP", "Live", "ProTools", "SONAR"}
	ledModeNames     = []string{"Internal", "External"}
	assignNames      = []string{"No Assign", "CC", "Note"}
	behaviorNames    = []string{"Momentary", "Toggle"}
	transportNames   = []string{
		"Prev Track", "Next Track", "Cycle",
		"Marker Set", "Prev Marker", "Next Marker",
		"REW", "FF", "Stop", "Play", "Rec",
	}
)

func fmtCh(ch int) string {
	if ch == 16 {
		return "Global"
	}
	return fmt.Sprintf("%d", ch+1)
}

func fmtMode(idx int, names []string) string {
	if idx >= 0 && idx < len(names) {
		return names[idx]
	}
	return fmt.Sprintf("unknown(%d)", idx)
}

func fmtButton(b ButtonConfig) string {
	if b.Assign == 0 {
		return "No Assign"
	}
	assign := fmtMode(b.Assign, assignNames)
	behavior := fmtMode(b.Behavior, behaviorNames)
	label := "CC"
	if b.Assign == 2 {
		label = "Note"
	}
	return fmt.Sprintf("%s, %s, %s=%d, off=%d, on=%d", assign, behavior, label, b.CC, b.OffVal, b.OnVal)
}

// displayScene prints the full scene configuration to stdout.
func displayScene(s *Scene) {
	fmt.Println("=== nanoKONTROL2 Scene Configuration ===")
	fmt.Printf("Global: MIDI ch=%d, mode=%s, LED=%s\n",
		s.GlobalMidiCh+1,
		fmtMode(s.ControlMode, controlModeNames),
		fmtMode(s.LEDMode, ledModeNames),
	)

	for i, g := range s.Groups {
		fmt.Printf("\nGroup %d (ch=%s):\n", i+1, fmtCh(g.MidiCh))

		if g.SliderEnabled {
			fmt.Printf("  Slider: CC=%d, range=%d-%d\n", g.SliderCC, g.SliderMin, g.SliderMax)
		} else {
			fmt.Println("  Slider: Disabled")
		}

		if g.KnobEnabled {
			fmt.Printf("  Knob:   CC=%d, range=%d-%d\n", g.KnobCC, g.KnobMin, g.KnobMax)
		} else {
			fmt.Println("  Knob:   Disabled")
		}

		fmt.Printf("  Solo:   %s\n", fmtButton(g.Solo))
		fmt.Printf("  Mute:   %s\n", fmtButton(g.Mute))
		fmt.Printf("  Rec:    %s\n", fmtButton(g.Rec))
	}

	fmt.Printf("\nTransport (ch=%s):\n", fmtCh(s.TransportCh))
	for i, name := range transportNames {
		fmt.Printf("  %-14s %s\n", name+":", fmtButton(s.Transport[i]))
	}
}
