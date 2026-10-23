package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gliderlabs/ssh"
)

const port = "2222"

// ---------------------------------------------------------
// 1. GAME DATA & QUESTIONS
// ---------------------------------------------------------

type Question struct {
	Category string
	Prompt   string
	Options  [4]string
	Answer   int // 0-indexed (0=A, 1=B, 2=C, 3=D)
	Points   int
}

var questionPool = []Question{
	{
		Category: "⚡ CYBER SEC",
		Prompt:   "What does SSH stand for?",
		Options:  [4]string{"Secure Shell", "System Super Host", "Simple Server Hub", "Safe Socket Handler"},
		Answer:   0,
		Points:   100,
	},
	{
		Category: "💻 SYSTEM KERNEL",
		Prompt:   "Which command kills a process by PID with SIGKILL in Linux/Unix?",
		Options:  [4]string{"kill -1", "kill -9", "kill -stop", "terminate /now"},
		Answer:   1,
		Points:   100,
	},
	{
		Category: "🎮 GAMING / TECH",
		Prompt:   "In The Matrix, what color pill lets Neo see the truth?",
		Options:  [4]string{"Blue Pill", "Green Pill", "Red Pill", "Gold Pill"},
		Answer:   2,
		Points:   150,
	},
	{
		Category: "👾 GOLANG LORE",
		Prompt:   "What is the name of Go's official blue mascot?",
		Options:  [4]string{"Ferris", "Gopher", "Tux", "Duke"},
		Answer:   1,
		Points:   100,
	},
	{
		Category: "🌐 NETWORKING",
		Prompt:   "What default port is standard for SSH connections?",
		Options:  [4]string{"80", "443", "8080", "22"},
		Answer:   3,
		Points:   100,
	},
	{
		Category: "🚀 SPACE / SCI-FI",
		Prompt:   "In 2001: A Space Odyssey, what is the rogue AI computer named?",
		Options:  [4]string{"GLADOS", "HAL 9000", "SKYNET", "JARVIS"},
		Answer:   1,
		Points:   150,
	},
	{
		Category: "⚡ HARDWARE",
		Prompt:   "What does GPU stand for?",
		Options:  [4]string{"General Processing Utility", "Graphics Processing Unit", "Gaming Power Upgrade", "Gigabit Port Unit"},
		Answer:   1,
		Points:   100,
	},
	{
		Category: "🔥 TERMINAL LORE",
		Prompt:   "Which editor famously traps beginners who can't figure out :wq?",
		Options:  [4]string{"Nano", "Emacs", "Vim", "VSCode"},
		Answer:   2,
		Points:   100,
	},
}

// ---------------------------------------------------------
// 2. RETRO ARCADE BANNER & STYLES
// ---------------------------------------------------------

const arcadeBanner = `
 █▀▀ █░█ █▄▄ █▀▀ █▀█   █▀▀ █░█ █▀█ █▀▀ █░█
 █▄▄ ▀▄▀ █▄█ ██▄ █▀▄   █▄▄ █▄█ █▀▄ ▄▄█ █▀█
`

var (
	bannerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF007F")).
			Bold(true)

	subBannerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00F0FF")).
			Bold(true)

	cardBox = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#BD00FF")).
		Padding(1, 2).
		Width(56)

	sideBox = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#00F0FF")).
		Padding(1, 2).
		Width(30)

	timerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFE600"))

	goldBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD700"))

	cyanBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00F0FF"))

	feedBox = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#FF007F")).
		PaddingLeft(1).
		MarginTop(1)
)

// ---------------------------------------------------------
// 3. MULTIPLAYER BATTLE ROYALE HUB
// ---------------------------------------------------------

type PlayerInfo struct {
	ID        int
	Name      string
	Score     int
	Streak    int
	HasAnswer bool
	Taunt     string
}

type LobbyState struct {
	RoundIndex int
	Question   Question
	TimeLeft   int
	RoundActive bool
	Winner     string
}

type Lobby struct {
	mu           sync.Mutex
	players      map[int]*PlayerInfo
	subscribers  map[int]chan GameUpdateMsg
	nextPlayerID int
	currentQ     Question
	roundNum     int
	timeLeft     int
	feed         []string
	qIndex       int
}

type GameUpdateMsg struct {
	Players     []PlayerInfo
	Question    Question
	RoundNum    int
	TimeLeft    int
	Feed        []string
	AnsweredBy  string
	Correct     bool
}

type TickMsg time.Time

var globalLobby = &Lobby{
	players:      make(map[int]*PlayerInfo),
	subscribers:  make(map[int]chan GameUpdateMsg),
	nextPlayerID: 1,
	timeLeft:     15,
	roundNum:     1,
	feed:         []string{"⚡ BATTLE ROYALE INITIATED! First to buzz the right answer wins!"},
}

func init() {
	globalLobby.currentQ = questionPool[0]
	// Game Loop Timer (Ticks every 1 second)
	go func() {
		for {
			time.Sleep(1 * time.Second)
			globalLobby.tick()
		}
	}()
}

func (l *Lobby) tick() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.players) == 0 {
		return
	}

	l.timeLeft--
	if l.timeLeft <= 0 {
		// Time expired, move to next question!
		l.qIndex = (l.qIndex + 1) % len(questionPool)
		l.currentQ = questionPool[l.qIndex]
		l.roundNum++
		l.timeLeft = 15

		// Reset answer status for all players
		for _, p := range l.players {
			p.HasAnswer = false
		}
		l.feed = append(l.feed, fmt.Sprintf("⏰ TIME EXPIRED! Next Round: %s", l.currentQ.Category))
	}

	l.broadcastUnlocked("", false)
}

func (l *Lobby) Join(name string) (int, chan GameUpdateMsg) {
	l.mu.Lock()
	defer l.mu.Unlock()

	id := l.nextPlayerID
	l.nextPlayerID++

	player := &PlayerInfo{
		ID:     id,
		Name:   name,
		Score:  0,
		Streak: 0,
	}
	l.players[id] = player

	ch := make(chan GameUpdateMsg, 16)
	l.subscribers[id] = ch

	l.feed = append(l.feed, fmt.Sprintf("👾 %s has entered the arena!", name))
	l.broadcastUnlocked("", false)
	return id, ch
}

func (l *Lobby) Leave(id int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if player, exists := l.players[id]; exists {
		name := player.Name
		delete(l.players, id)
		delete(l.subscribers, id)
		l.feed = append(l.feed, fmt.Sprintf("💀 %s disconnected.", name))
		l.broadcastUnlocked("", false)
	}
}

func (l *Lobby) SubmitAnswer(playerID int, choice int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	player, exists := l.players[playerID]
	if !exists || player.HasAnswer {
		return
	}

	player.HasAnswer = true

	if choice == l.currentQ.Answer {
		// Speed bonus based on timeLeft!
		bonus := l.timeLeft * 10
		totalPts := l.currentQ.Points + bonus
		player.Score += totalPts
		player.Streak++

		var streakBonus string
		if player.Streak > 1 {
			streakBonus = fmt.Sprintf(" (🔥 STREAK x%d!)", player.Streak)
		}

		event := fmt.Sprintf("💥 %s BUZZED FIRST! +%d pts%s", player.Name, totalPts, streakBonus)
		l.feed = append(l.feed, event)

		// Next round!
		l.qIndex = (l.qIndex + 1) % len(questionPool)
		l.currentQ = questionPool[l.qIndex]
		l.roundNum++
		l.timeLeft = 15

		for _, p := range l.players {
			p.HasAnswer = false
		}
		l.broadcastUnlocked(player.Name, true)
	} else {
		player.Streak = 0
		if player.Score >= 50 {
			player.Score -= 50
		}
		event := fmt.Sprintf("❌ %s guessed wrong! (-50 pts)", player.Name)
		l.feed = append(l.feed, event)
		l.broadcastUnlocked(player.Name, false)
	}
}

func (l *Lobby) SendTaunt(playerID int, taunt string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	player, exists := l.players[playerID]
	if !exists {
		return
	}

	player.Taunt = taunt
	event := fmt.Sprintf("💬 %s: %s", player.Name, taunt)
	l.feed = append(l.feed, event)
	l.broadcastUnlocked("", false)
}

func (l *Lobby) broadcastUnlocked(answeredBy string, correct bool) {
	if len(l.feed) > 5 {
		l.feed = l.feed[len(l.feed)-5:]
	}

	playerList := make([]PlayerInfo, 0, len(l.players))
	for _, p := range l.players {
		playerList = append(playerList, *p)
	}

	msg := GameUpdateMsg{
		Players:    playerList,
		Question:   l.currentQ,
		RoundNum:   l.roundNum,
		TimeLeft:   l.timeLeft,
		Feed:       append([]string{}, l.feed...),
		AnsweredBy: answeredBy,
		Correct:    correct,
	}

	for _, ch := range l.subscribers {
		select {
		case ch <- msg:
		default:
		}
	}
}

// ---------------------------------------------------------
// 4. BUBBLE TEA CLIENT UI
// ---------------------------------------------------------

type model struct {
	playerID   int
	playerName string
	lobbyCh    chan GameUpdateMsg
	players    []PlayerInfo
	question   Question
	roundNum   int
	timeLeft   int
	feed       []string
	cursor     int
	myPlayer   PlayerInfo
	feedback   string
}

func waitForGameUpdate(ch chan GameUpdateMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m model) Init() tea.Cmd {
	return waitForGameUpdate(m.lobbyCh)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case GameUpdateMsg:
		m.players = msg.Players
		m.question = msg.Question
		m.roundNum = msg.RoundNum
		m.timeLeft = msg.TimeLeft
		m.feed = msg.Feed

		// Find own updated stats
		for _, p := range m.players {
			if p.ID == m.playerID {
				m.myPlayer = p
				break
			}
		}

		if msg.AnsweredBy != "" {
			if msg.Correct {
				m.feedback = fmt.Sprintf("🎯 %s got it right!", msg.AnsweredBy)
			} else {
				m.feedback = fmt.Sprintf("❌ %s was wrong!", msg.AnsweredBy)
			}
		}
		return m, waitForGameUpdate(m.lobbyCh)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit

		// Option shortcuts (1,2,3,4 or A,B,C,D)
		case "1", "a", "A":
			globalLobby.SubmitAnswer(m.playerID, 0)
		case "2", "b", "B":
			globalLobby.SubmitAnswer(m.playerID, 1)
		case "3", "c", "C":
			globalLobby.SubmitAnswer(m.playerID, 2)
		case "4", "d", "D":
			globalLobby.SubmitAnswer(m.playerID, 3)

		// Arrow navigation
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < 3 {
				m.cursor++
			}
		case "enter", "space":
			globalLobby.SubmitAnswer(m.playerID, m.cursor)

		// Taunt / Emote shortcuts!
		case "t":
			taunts := []string{"Too slow! 😎", "GG! 🚀", "Catch me if you can! ⚡", "Matrix level! 🕶️"}
			globalLobby.SendTaunt(m.playerID, taunts[rand.Intn(len(taunts))])
		}
	}
	return m, nil
}

// ---------------------------------------------------------
// 5. VIEW (RETRO GAMING TUI)
// ---------------------------------------------------------

func (m model) View() string {
	// Top Banner
	banner := bannerStyle.Render(arcadeBanner)
	sub := subBannerStyle.Render("       ═══ MULTIPLAYER CYBER TRIVIA RUSH ═══")

	// Timer Bar (Dynamic Visual Meter)
	var timerMeter strings.Builder
	timerMeter.WriteString("⏳ TIME: [")
	for i := 0; i < 15; i++ {
		if i < m.timeLeft {
			timerMeter.WriteString("█")
		} else {
			timerMeter.WriteString("░")
		}
	}
	timerMeter.WriteString(fmt.Sprintf("] %02ds", m.timeLeft))

	timerRendered := timerStyle.Render(timerMeter.String())
	roundBadge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF007F")).Render(fmt.Sprintf("ROUND #%d", m.roundNum))
	statusBar := fmt.Sprintf("%s    %s    %s", roundBadge, timerRendered, cyanBadge.Render(m.question.Category))

	// Options rendering
	optionLetters := []string{"[A]", "[B]", "[C]", "[D]"}
	var optionsView strings.Builder
	for i, opt := range m.question.Options {
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D8E0"))

		if i == m.cursor {
			prefix = "👉 "
			style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00F0FF")).Background(lipgloss.Color("#2D1B69"))
		}

		optionsView.WriteString(style.Render(fmt.Sprintf("%s%s %s", prefix, optionLetters[i], opt)) + "\n")
	}

	// Question Card
	questionBody := fmt.Sprintf(
		"%s\n\n"+
			"❓ %s\n\n"+
			"%s\n"+
			"⚡ Base Value: %d pts (+ Speed Bonus)\n\n"+
			"📜 Live Battle Feed:\n%s",
		statusBar,
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render(m.question.Prompt),
		optionsView.String(),
		m.question.Points,
		feedBox.Render(strings.Join(m.feed, "\n")),
	)
	card := cardBox.Render(questionBody)

	// Leaderboard & Active Players Sidepanel
	var side strings.Builder
	side.WriteString(goldBadge.Render("🏆 ARENA LEADERBOARD") + "\n\n")

	for rank, p := range m.players {
		badge := "  "
		if p.ID == m.playerID {
			badge = "⭐"
		}
		streakStr := ""
		if p.Streak > 1 {
			streakStr = fmt.Sprintf(" 🔥x%d", p.Streak)
		}
		side.WriteString(fmt.Sprintf("%s %d. %-10s %s pts%s\n", badge, rank+1, p.Name, goldBadge.Render(fmt.Sprintf("%d", p.Score)), streakStr))
		if p.Taunt != "" {
			side.WriteString(fmt.Sprintf("    💭 %s\n", lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#888888")).Render(p.Taunt)))
		}
	}

	side.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#A4B0BE")).Render("Controls:\n• [1-4] or [A-D] Quick Buzz\n• [↑/↓] + Enter Select\n• [T] Taunt / Emote\n• [Q] Exit"))
	sidePanel := sideBox.Render(side.String())

	// Horizontal layout
	board := lipgloss.JoinHorizontal(lipgloss.Top, card, "  ", sidePanel)

	return fmt.Sprintf("%s\n%s\n\n%s\n", banner, sub, board)
}

// ---------------------------------------------------------
// 6. SSH SERVER LAUNCHER
// ---------------------------------------------------------

func sshHandler(s ssh.Session) {
	_, winCh, isPty := s.Pty()
	if !isPty {
		fmt.Fprintln(s, "Terminal (PTY) required.")
		return
	}

	name := s.User()
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("Hacker%d", rand.Intn(999))
	}

	playerID, lobbyCh := globalLobby.Join(name)
	defer globalLobby.Leave(playerID)

	m := model{
		playerID:   playerID,
		playerName: name,
		lobbyCh:    lobbyCh,
		players:    make([]PlayerInfo, 0),
		feed:       append([]string{}, globalLobby.feed...),
		roundNum:   globalLobby.roundNum,
		question:   globalLobby.currentQ,
		timeLeft:   globalLobby.timeLeft,
	}

	p := tea.NewProgram(
		m,
		tea.WithInput(s),
		tea.WithOutput(s),
		tea.WithAltScreen(),
	)

	go func() {
		for win := range winCh {
			p.Send(tea.WindowSizeMsg{Width: win.Width, Height: win.Height})
		}
	}()

	if _, err := p.Run(); err != nil {
		log.Printf("Session ended: %v", err)
	}
}

func main() {
	server := &ssh.Server{
		Addr:    ":" + port,
		Handler: sshHandler,
	}
	server.SetOption(ssh.HostKeyFile(".ssh_host_key"))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("🕹️ Cyber Rush SSH Server live on port %s...", port)
		log.Printf("👉 Connect: ssh -p %s <your_name>@localhost", port)
		if err := server.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down Cyber Rush server...")
}