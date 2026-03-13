package display

import (
	"fmt"
	"io"

	"nanoctl/internal/scene"
)

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

func fmtButton(b scene.ButtonConfig) string {
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

// DisplayScene prints the full scene configuration to w.
func DisplayScene(w io.Writer, s *scene.Scene) {
	fmt.Fprintln(w, "=== nanoKONTROL2 Scene Configuration ===")
	fmt.Fprintf(w, "Global: MIDI ch=%d, mode=%s, LED=%s\n",
		s.GlobalMidiCh+1,
		fmtMode(s.ControlMode, controlModeNames),
		fmtMode(s.LEDMode, ledModeNames),
	)

	for i, g := range s.Groups {
		fmt.Fprintf(w, "\nGroup %d (ch=%s):\n", i+1, fmtCh(g.MidiCh))

		if g.SliderEnabled {
			fmt.Fprintf(w, "  Slider: CC=%d, range=%d-%d\n", g.SliderCC, g.SliderMin, g.SliderMax)
		} else {
			fmt.Fprintln(w, "  Slider: Disabled")
		}

		if g.KnobEnabled {
			fmt.Fprintf(w, "  Knob:   CC=%d, range=%d-%d\n", g.KnobCC, g.KnobMin, g.KnobMax)
		} else {
			fmt.Fprintln(w, "  Knob:   Disabled")
		}

		fmt.Fprintf(w, "  Solo:   %s\n", fmtButton(g.Solo))
		fmt.Fprintf(w, "  Mute:   %s\n", fmtButton(g.Mute))
		fmt.Fprintf(w, "  Rec:    %s\n", fmtButton(g.Rec))
	}

	fmt.Fprintf(w, "\nTransport (ch=%s):\n", fmtCh(s.TransportCh))
	for i, name := range transportNames {
		fmt.Fprintf(w, "  %-14s %s\n", name+":", fmtButton(s.Transport[i]))
	}
}
