package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	qrcode "github.com/skip2/go-qrcode"
)

type tickMsg time.Time
type model struct {
	setup         []byte
	frames        [][]byte
	file          string
	index, loops  int
	fps           float64
	streaming     bool
	width, height int
	err           error
}

func tick(fps float64) tea.Cmd {
	return tea.Tick(time.Duration(float64(time.Second)/fps), func(t time.Time) tea.Msg { return tickMsg(t) })
}
func (m model) Init() tea.Cmd { return tick(m.fps) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
	case tickMsg:
		if m.streaming {
			m.index++
			if m.index >= len(m.frames) {
				m.index = 0
				m.loops++
			}
		}
		return m, tick(m.fps)
	case tea.KeyMsg:
		switch v.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if !m.streaming {
				m.streaming = true
				m.index = 0
			}
		case "r":
			m.streaming = false
			m.index = 0
			m.loops = 0
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return "Error: " + m.err.Error() + "\n\nPress q to quit.\n"
	}
	payload := m.setup
	if m.streaming {
		payload = m.frames[m.index]
	}
	qr, err := qrcode.New(string(payload), qrcode.Low)
	if err != nil {
		return "QR error: " + err.Error()
	}
	bitmap := qr.Bitmap()
	var b strings.Builder
	// Each half-block stores two square QR modules: foreground above,
	// background below. This keeps the QR's geometry square and compact.
	pad := 2
	size := len(bitmap) + pad*2
	module := func(x, y int) bool {
		if x < pad || y < pad || x >= size-pad || y >= size-pad {
			return false
		}
		return bitmap[y-pad][x-pad]
	}
	for y := 0; y < size; y += 2 {
		for x := 0; x < size; x++ {
			top, bottom := module(x, y), module(x, y+1)
			fg, bg := 97, 107
			if top {
				fg = 30
			}
			if bottom {
				bg = 40
			}
			b.WriteString(fmt.Sprintf("\x1b[%d;%dm▀", fg, bg))
		}
		b.WriteString("\x1b[0m\n")
	}
	if !m.streaming {
		b.WriteString(fmt.Sprintf("\n%s  •  SETUP QR\n", m.file))
		b.WriteString("Scan this code with the iPhone. When it says ready, press ENTER.\n")
		b.WriteString("r reset  q quit\n")
	} else {
		b.WriteString(fmt.Sprintf("\n%s  •  frame %d/%d  •  loop %d  •  %.1f fps\n", m.file, m.index+1, len(m.frames), m.loops+1, m.fps))
		b.WriteString("r show setup QR  q quit\n")
	}
	return b.String()
}

func main() {
	chunkSize := flag.Int("chunk-size", 80, "raw transfer bytes per QR data frame")
	fps := flag.Float64("fps", 3.0, "data QR frames per second")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: qrairdrop [--chunk-size bytes] [--fps n] <file>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *chunkSize < 1 {
		fmt.Fprintln(os.Stderr, "--chunk-size must be positive")
		os.Exit(2)
	}
	if *fps <= 0 {
		fmt.Fprintln(os.Stderr, "--fps must be positive")
		os.Exit(2)
	}

	path := flag.Arg(0)
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	setup, frames, err := makePackets(path, data, *chunkSize, int(1000/(*fps)))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	m := model{setup: setup, frames: frames, file: path, fps: *fps}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
