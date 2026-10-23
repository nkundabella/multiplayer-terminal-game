package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gliderlabs/ssh"
)

const (
	port = "2222"
)

// --- Visual Styles (Lip Gloss) ---
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 2).
			MarginBottom(1)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 3).
			Width(52)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Italic(true)
)

// --- Bubble Tea: The Elm Architecture ---
// 1. Model: The state of the UI for each connected user
type model struct {
	user      string
	connected time.Time
	counter   int
}

// 2. Init: Initial command run when user joins
func (m model) Init() tea.Cmd {
	return nil
}

// 3. Update: Event handler (keyboard presses, window resize, timers)
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k", "space":
			m.counter++
		case "down", "j":
			m.counter--
		}
	}
	return m, nil
}

// 4. View: Renders the Model into terminal text & colors
func (m model) View() string {
	elapsed := time.Since(m.connected).Round(time.Second)

	body := fmt.Sprintf(
		"%s\n\n"+
			"👤 Player: %s\n"+
			"⏱️  Online: %s\n"+
			"🔢 Counter: %s\n\n"+
			"%s",
		titleStyle.Render("🎮 SSH TERMINAL GAME SERVER"),
		infoStyle.Render(m.user),
		elapsed.String(),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87")).Bold(true).Render(fmt.Sprintf("%d", m.counter)),
		hintStyle.Render("Press [Space/Up] to increment • [Down] decrement • [q] to quit"),
	)

	return cardStyle.Render(body) + "\n"
}

// --- SSH Handler: Runs per connected SSH client ---
func sshHandler(s ssh.Session) {
	ptyReq, winCh, isPty := s.Pty()
	if !isPty {
		fmt.Fprintln(s, "Error: Terminal (PTY) required to play.")
		return
	}

	// Initialize state for this specific user
	m := model{
		user:      s.User(),
		connected: time.Now(),
		counter:   0,
	}

	// Start Bubble Tea program piped directly to the SSH session
	p := tea.NewProgram(
		m,
		tea.WithInput(s),
		tea.WithOutput(s),
		tea.WithAltScreen(),       // Uses alternate screen buffer (like vim or htop)
		tea.WithMouseCellMotion(), // Mouse support in terminal!
	)

	// Keep window dimensions synchronized when client resizes terminal
	go func() {
		for win := range winCh {
			p.Send(tea.WindowSizeMsg{
				Width:  win.Width,
				Height: win.Height,
			})
		}
	}()

	_ = ptyReq
	if _, err := p.Run(); err != nil {
		log.Printf("Error running TUI for %s: %v", s.User(), err)
	}
}

func main() {
	// Create the SSH Server
	server := &ssh.Server{
		Addr:    ":" + port,
		Handler: sshHandler,
	}

	// Graceful shutdown on Ctrl+C in server console
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("✨ SSH Server running on port %s...", port)
		log.Printf("👉 Connect using: ssh -p %s <your_name>@localhost", port)
		if err := server.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
			log.Fatalf("SSH server error: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down SSH server...")
}
