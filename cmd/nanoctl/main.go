package main

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/pflag"

	"nanoctl/internal/config"
	"nanoctl/internal/display"
	"nanoctl/internal/midi"
	"nanoctl/internal/scene"
)

// transportButtonNames lists the transport buttons in scene data order.
var transportButtonNames = []string{
	"prev-track", "next-track", "cycle",
	"marker-set", "prev-marker", "next-marker",
	"rew", "ff", "stop", "play", "rec",
}

// transportAliases maps alias button name → canonical button name.
var transportAliases = []struct{ alias, canonical string }{
	{"record", "rec"},
	{"ffwd", "ff"},
	{"fastforward", "ff"},
	{"rewind", "rew"},
}

// groupButtonNames lists the per-strip button types.
var groupButtonNames = []string{"solo", "mute", "rec"}

// ccSender is a function that sends a single MIDI CC message.
type ccSender func(ch, cc, val int) error

func main() {
	fs := pflag.NewFlagSet("nanoctl", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	// --- Global flags ---
	portFlag := fs.String("port", "nanoKONTROL2", "MIDI port name (substring match)")
	listPortsFlag := fs.Bool("list-ports", false, "list available MIDI ports and exit")
	showFlag := fs.Bool("show", false, "display the current configuration")
	jsonFlag := fs.BoolP("json", "j", false, "output scene as JSON (use with --show)")
	outputFlag := fs.StringP("output", "o", "", "write output to a file instead of stdout")
	configFlag := fs.StringP("config-file", "f", "", "read JSON config file and apply to device")
	globalMidiCh := fs.Int("global-midi-ch", 1, "Global MIDI channel (1-16)")
	controlMode := fs.String("control-mode", "cc", "Control mode: cc|cubase|dp|live|protools|sonar")
	ledMode := fs.String("led-mode", "internal", "LED mode: internal|external")

	// --- Per-group flags ---
	// indexed [group 0..7][button 0..2]
	groupMidiCh := make([]string, 8)
	groupSliderEnabled := make([]string, 8)
	groupSliderCC := make([]int, 8)
	groupSliderMin := make([]int, 8)
	groupSliderMax := make([]int, 8)
	groupKnobEnabled := make([]string, 8)
	groupKnobCC := make([]int, 8)
	groupKnobMin := make([]int, 8)
	groupKnobMax := make([]int, 8)

	// button configs: [group][button] where button 0=solo,1=mute,2=rec
	groupBtnAssign := make([][]string, 8)
	groupBtnBehavior := make([][]string, 8)
	groupBtnCC := make([][]int, 8)
	groupBtnOff := make([][]int, 8)
	groupBtnOn := make([][]int, 8)
	groupBtnLED := make([][]string, 8)

	for i := range 8 {
		n := i + 1
		groupBtnAssign[i] = make([]string, 3)
		groupBtnBehavior[i] = make([]string, 3)
		groupBtnCC[i] = make([]int, 3)
		groupBtnOff[i] = make([]int, 3)
		groupBtnOn[i] = make([]int, 3)
		groupBtnLED[i] = make([]string, 3)

		fs.StringVar(&groupMidiCh[i], fmt.Sprintf("group-%d-midi-ch", n), "global", "MIDI channel (1-16 or global)")
		fs.StringVar(&groupSliderEnabled[i], fmt.Sprintf("group-%d-slider-enabled", n), "true", "Slider enabled (true|false)")
		fs.IntVar(&groupSliderCC[i], fmt.Sprintf("group-%d-slider-cc", n), 0, "Slider CC number (0-127)")
		fs.IntVar(&groupSliderMin[i], fmt.Sprintf("group-%d-slider-min", n), 0, "Slider min value (0-127)")
		fs.IntVar(&groupSliderMax[i], fmt.Sprintf("group-%d-slider-max", n), 127, "Slider max value (0-127)")
		fs.StringVar(&groupKnobEnabled[i], fmt.Sprintf("group-%d-knob-enabled", n), "true", "Knob enabled (true|false)")
		fs.IntVar(&groupKnobCC[i], fmt.Sprintf("group-%d-knob-cc", n), 0, "Knob CC number (0-127)")
		fs.IntVar(&groupKnobMin[i], fmt.Sprintf("group-%d-knob-min", n), 0, "Knob min value (0-127)")
		fs.IntVar(&groupKnobMax[i], fmt.Sprintf("group-%d-knob-max", n), 127, "Knob max value (0-127)")

		for j, btn := range groupButtonNames {
			fs.StringVar(&groupBtnAssign[i][j], fmt.Sprintf("group-%d-%s-assign", n, btn), "cc", "assign: none|cc|note")
			fs.StringVar(&groupBtnBehavior[i][j], fmt.Sprintf("group-%d-%s-behavior", n, btn), "momentary", "behavior: momentary|toggle")
			fs.IntVar(&groupBtnCC[i][j], fmt.Sprintf("group-%d-%s-cc", n, btn), 0, "CC number (0-127)")
			fs.IntVar(&groupBtnOff[i][j], fmt.Sprintf("group-%d-%s-off", n, btn), 0, "Off value (0-127)")
			fs.IntVar(&groupBtnOn[i][j], fmt.Sprintf("group-%d-%s-on", n, btn), 127, "On value (0-127)")
			fs.StringVar(&groupBtnLED[i][j], fmt.Sprintf("group-%d-%s-led", n, btn), "off", "LED state: on|off")
		}
	}

	// --- Transport flags ---
	transportMidiCh := fs.String("transport-midi-ch", "global", "Transport MIDI channel (1-16 or global)")
	transportBtnAssign := make([]string, 11)
	transportBtnBehavior := make([]string, 11)
	transportBtnCC := make([]int, 11)
	transportBtnOff := make([]int, 11)
	transportBtnOn := make([]int, 11)
	transportBtnLED := make([]string, 11)

	for i, name := range transportButtonNames {
		fs.StringVar(&transportBtnAssign[i], fmt.Sprintf("transport-%s-assign", name), "cc", "assign: none|cc|note")
		fs.StringVar(&transportBtnBehavior[i], fmt.Sprintf("transport-%s-behavior", name), "momentary", "behavior: momentary|toggle")
		fs.IntVar(&transportBtnCC[i], fmt.Sprintf("transport-%s-cc", name), 0, "CC number (0-127)")
		fs.IntVar(&transportBtnOff[i], fmt.Sprintf("transport-%s-off", name), 0, "Off value (0-127)")
		fs.IntVar(&transportBtnOn[i], fmt.Sprintf("transport-%s-on", name), 127, "On value (0-127)")
		fs.StringVar(&transportBtnLED[i], fmt.Sprintf("transport-%s-led", name), "off", "LED state: on|off")
	}

	for _, a := range transportAliases {
		canonIdx := slices.Index(transportButtonNames, a.canonical)
		for _, suffix := range []string{"assign", "behavior", "cc", "off", "on", "led"} {
			aliasName := fmt.Sprintf("transport-%s-%s", a.alias, suffix)
			canonName := fmt.Sprintf("transport-%s-%s", a.canonical, suffix)
			switch suffix {
			case "assign":
				fs.StringVar(&transportBtnAssign[canonIdx], aliasName, fs.Lookup(canonName).DefValue, "")
			case "behavior":
				fs.StringVar(&transportBtnBehavior[canonIdx], aliasName, fs.Lookup(canonName).DefValue, "")
			case "cc":
				fs.IntVar(&transportBtnCC[canonIdx], aliasName, 0, "")
			case "off":
				fs.IntVar(&transportBtnOff[canonIdx], aliasName, 0, "")
			case "on":
				fs.IntVar(&transportBtnOn[canonIdx], aliasName, 127, "")
			case "led":
				fs.StringVar(&transportBtnLED[canonIdx], aliasName, fs.Lookup(canonName).DefValue, "")
			}
			fs.MarkHidden(aliasName)
		}
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == pflag.ErrHelp {
			fmt.Printf("Usage of nanoctl:\n%s", fs.FlagUsages())
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *listPortsFlag {
		midi.ListPorts(*portFlag)
		os.Exit(0)
	}

	// Determine if any scene-param or LED flags were changed.
	sceneFlags := collectSceneFlags(fs)
	ledFlags := collectLEDFlags(fs)

	if len(sceneFlags) == 0 && len(ledFlags) == 0 && !*showFlag && !fs.Changed("config-file") {
		fmt.Fprintln(os.Stderr, "error: nothing to do — specify at least one flag or --show")
		os.Exit(2)
	}

	// Open MIDI port.
	conn, cleanup, err := midi.OpenPorts(*portFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening MIDI port: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	var rawScene []byte
	var s *scene.Scene

	// Apply config file if specified.
	if fs.Changed("config-file") {
		if len(sceneFlags) > 0 {
			fmt.Fprintln(os.Stderr, "error: --config-file cannot be combined with individual scene flags")
			os.Exit(1)
		}
		newScene, err := config.LoadSceneFromFile(*configFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if rawScene == nil {
			rawScene, _, err = midi.QueryScene(conn)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error querying scene: %v\n", err)
				os.Exit(1)
			}
		}
		rawScene = scene.ApplySceneToBytes(rawScene, newScene)
		if err := midi.WriteScene(conn, rawScene); err != nil {
			fmt.Fprintf(os.Stderr, "error writing scene: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Scene written successfully.")
	}

	// Apply scene parameter changes if any were set.
	if len(sceneFlags) > 0 {
		rawScene, s, err = midi.QueryScene(conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error querying scene: %v\n", err)
			os.Exit(1)
		}

		applySceneFlags(s, sceneFlags, globalMidiCh, controlMode, ledMode,
			groupMidiCh, groupSliderEnabled, groupSliderCC, groupSliderMin, groupSliderMax,
			groupKnobEnabled, groupKnobCC, groupKnobMin, groupKnobMax,
			groupBtnAssign, groupBtnBehavior, groupBtnCC, groupBtnOff, groupBtnOn,
			transportMidiCh, transportBtnAssign, transportBtnBehavior, transportBtnCC, transportBtnOff, transportBtnOn)

		rawScene = scene.ApplySceneToBytes(rawScene, s)

		if err := midi.WriteScene(conn, rawScene); err != nil {
			fmt.Fprintf(os.Stderr, "error writing scene: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Scene written successfully.")
	}

	// Send LED CC messages if any LED flags were set.
	if len(ledFlags) > 0 {
		if s == nil {
			rawScene, s, err = midi.QueryScene(conn)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error querying scene: %v\n", err)
				os.Exit(1)
			}
			_ = rawScene
		}

		if s.LEDMode != 1 {
			fmt.Fprintln(os.Stderr, "warning: LED mode is Internal — LEDs are controlled by the device, not the host")
		}

		if err := sendLEDs(func(ch, cc, val int) error { return midi.SendCC(conn, ch, cc, val) },
			s, ledFlags, groupBtnLED, transportBtnLED); err != nil {
			fmt.Fprintf(os.Stderr, "error sending LED messages: %v\n", err)
			os.Exit(1)
		}
	}

	// If --show was requested, query the (possibly updated) scene and display it.
	if *showFlag {
		_, s, err = midi.QueryScene(conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error querying scene: %v\n", err)
			os.Exit(1)
		}

		out, closeOut, err := openOutput(*outputFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		defer closeOut()

		if *jsonFlag {
			data, err := config.MarshalSceneJSON(s)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error marshalling JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintln(out, string(data))
		} else {
			display.DisplayScene(out, s)
		}
	}
}

// openOutput returns an io.Writer for path (or stdout if path is empty),
// along with a close function to call when done.
func openOutput(path string) (io.Writer, func(), error) {
	if path == "" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("opening output file: %w", err)
	}
	return f, func() { f.Close() }, nil
}

// collectSceneFlags returns the set of flag names that were changed and affect scene data.
func collectSceneFlags(fs *pflag.FlagSet) map[string]bool {
	sceneParamPrefixes := []string{
		"global-midi-ch", "control-mode", "led-mode",
	}
	sceneParamContains := []string{
		"-midi-ch", "-slider-", "-knob-",
		"-solo-assign", "-solo-behavior", "-solo-cc", "-solo-off", "-solo-on",
		"-mute-assign", "-mute-behavior", "-mute-cc", "-mute-off", "-mute-on",
		"-rec-assign", "-rec-behavior", "-rec-cc", "-rec-off", "-rec-on",
		"transport-midi-ch",
		"-assign", "-behavior", "-cc", "-off", "-on",
	}

	changed := map[string]bool{}
	fs.VisitAll(func(f *pflag.Flag) {
		if !f.Changed {
			return
		}
		name := normalizeTransportFlagName(f.Name)
		if strings.HasSuffix(name, "-led") {
			return // LED flags handled separately
		}
		if slices.Contains(sceneParamPrefixes, name) {
			changed[name] = true
			return
		}
		for _, sub := range sceneParamContains {
			if strings.Contains(name, sub) {
				changed[name] = true
				return
			}
		}
	})
	return changed
}

// collectLEDFlags returns flag names ending in "-led" that were changed.
func collectLEDFlags(fs *pflag.FlagSet) map[string]bool {
	changed := map[string]bool{}
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Changed && strings.HasSuffix(f.Name, "-led") {
			changed[normalizeTransportFlagName(f.Name)] = true
		}
	})
	return changed
}

// normalizeTransportFlagName maps alias transport flag names to their canonical equivalents.
func normalizeTransportFlagName(name string) string {
	for _, a := range transportAliases {
		old := fmt.Sprintf("transport-%s-", a.alias)
		new := fmt.Sprintf("transport-%s-", a.canonical)
		if strings.HasPrefix(name, old) {
			return strings.Replace(name, old, new, 1)
		}
	}
	return name
}

func parseBoolFlag(val string) bool {
	return strings.ToLower(val) == "true" || val == "1" || strings.ToLower(val) == "yes"
}

// applySceneFlags applies all changed scene-param flags to the scene struct.
func applySceneFlags(
	s *scene.Scene, changed map[string]bool,
	globalMidiCh *int, controlMode, ledMode *string,
	groupMidiCh, groupSliderEnabled []string,
	groupSliderCC, groupSliderMin, groupSliderMax []int,
	groupKnobEnabled []string,
	groupKnobCC, groupKnobMin, groupKnobMax []int,
	groupBtnAssign, groupBtnBehavior [][]string,
	groupBtnCC, groupBtnOff, groupBtnOn [][]int,
	transportMidiCh *string,
	transportBtnAssign, transportBtnBehavior []string,
	transportBtnCC, transportBtnOff, transportBtnOn []int,
) {
	if changed["global-midi-ch"] {
		s.GlobalMidiCh = *globalMidiCh - 1
	}
	if changed["control-mode"] {
		if idx, err := scene.EnumFromString(*controlMode, scene.ControlModeNames, "control_mode"); err == nil {
			s.ControlMode = idx
		}
	}
	if changed["led-mode"] {
		if idx, err := scene.EnumFromString(*ledMode, scene.LEDModeNames, "led_mode"); err == nil {
			s.LEDMode = idx
		}
	}

	for i := range 8 {
		n := i + 1
		g := &s.Groups[i]

		if changed[fmt.Sprintf("group-%d-midi-ch", n)] {
			if ch, err := scene.ChFromString(groupMidiCh[i]); err == nil {
				g.MidiCh = ch
			}
		}
		if changed[fmt.Sprintf("group-%d-slider-enabled", n)] {
			g.SliderEnabled = parseBoolFlag(groupSliderEnabled[i])
		}
		if changed[fmt.Sprintf("group-%d-slider-cc", n)] {
			g.SliderCC = groupSliderCC[i]
		}
		if changed[fmt.Sprintf("group-%d-slider-min", n)] {
			g.SliderMin = groupSliderMin[i]
		}
		if changed[fmt.Sprintf("group-%d-slider-max", n)] {
			g.SliderMax = groupSliderMax[i]
		}
		if changed[fmt.Sprintf("group-%d-knob-enabled", n)] {
			g.KnobEnabled = parseBoolFlag(groupKnobEnabled[i])
		}
		if changed[fmt.Sprintf("group-%d-knob-cc", n)] {
			g.KnobCC = groupKnobCC[i]
		}
		if changed[fmt.Sprintf("group-%d-knob-min", n)] {
			g.KnobMin = groupKnobMin[i]
		}
		if changed[fmt.Sprintf("group-%d-knob-max", n)] {
			g.KnobMax = groupKnobMax[i]
		}

		btns := []*scene.ButtonConfig{&g.Solo, &g.Mute, &g.Rec}
		for j, btn := range groupButtonNames {
			b := btns[j]
			if changed[fmt.Sprintf("group-%d-%s-assign", n, btn)] {
				if idx, err := scene.EnumFromString(groupBtnAssign[i][j], scene.AssignNames, "assign"); err == nil {
					b.Assign = idx
				}
			}
			if changed[fmt.Sprintf("group-%d-%s-behavior", n, btn)] {
				if idx, err := scene.EnumFromString(groupBtnBehavior[i][j], scene.BehaviorNames, "behavior"); err == nil {
					b.Behavior = idx
				}
			}
			if changed[fmt.Sprintf("group-%d-%s-cc", n, btn)] {
				b.CC = groupBtnCC[i][j]
			}
			if changed[fmt.Sprintf("group-%d-%s-off", n, btn)] {
				b.OffVal = groupBtnOff[i][j]
			}
			if changed[fmt.Sprintf("group-%d-%s-on", n, btn)] {
				b.OnVal = groupBtnOn[i][j]
			}
		}
	}

	if changed["transport-midi-ch"] {
		if ch, err := scene.ChFromString(*transportMidiCh); err == nil {
			s.TransportCh = ch
		}
	}
	for i, name := range transportButtonNames {
		b := &s.Transport[i]
		if changed[fmt.Sprintf("transport-%s-assign", name)] {
			if idx, err := scene.EnumFromString(transportBtnAssign[i], scene.AssignNames, "assign"); err == nil {
				b.Assign = idx
			}
		}
		if changed[fmt.Sprintf("transport-%s-behavior", name)] {
			if idx, err := scene.EnumFromString(transportBtnBehavior[i], scene.BehaviorNames, "behavior"); err == nil {
				b.Behavior = idx
			}
		}
		if changed[fmt.Sprintf("transport-%s-cc", name)] {
			b.CC = transportBtnCC[i]
		}
		if changed[fmt.Sprintf("transport-%s-off", name)] {
			b.OffVal = transportBtnOff[i]
		}
		if changed[fmt.Sprintf("transport-%s-on", name)] {
			b.OnVal = transportBtnOn[i]
		}
	}
}

// sendLEDs sends CC messages for all changed LED flags.
func sendLEDs(send ccSender, s *scene.Scene, ledFlags map[string]bool,
	groupBtnLED [][]string, transportBtnLED []string,
) error {
	for i := range 8 {
		n := i + 1
		g := s.Groups[i]
		btns := []scene.ButtonConfig{g.Solo, g.Mute, g.Rec}
		for j, btn := range groupButtonNames {
			flagName := fmt.Sprintf("group-%d-%s-led", n, btn)
			if !ledFlags[flagName] {
				continue
			}
			ch := scene.EffectiveMidiCh(g.MidiCh, s.GlobalMidiCh)
			cc := btns[j].CC
			val := 0
			if strings.ToLower(groupBtnLED[i][j]) == "on" {
				val = 127
			}
			if err := send(ch, cc, val); err != nil {
				return fmt.Errorf("send LED CC for %s: %w", flagName, err)
			}
		}
	}

	for i, name := range transportButtonNames {
		flagName := fmt.Sprintf("transport-%s-led", name)
		if !ledFlags[flagName] {
			continue
		}
		ch := scene.EffectiveMidiCh(s.TransportCh, s.GlobalMidiCh)
		cc := s.Transport[i].CC
		val := 0
		if strings.ToLower(transportBtnLED[i]) == "on" {
			val = 127
		}
		if err := send(ch, cc, val); err != nil {
			return fmt.Errorf("send LED CC for %s: %w", flagName, err)
		}
	}
	return nil
}
