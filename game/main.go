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
const arenaWidth = 72

// -----------------------------------------------------------------
// 1. MECHA CYBORG HIGH-DEF ASCII SPRITES (6 Lines High, 14 Chars Wide)
// -----------------------------------------------------------------

type FighterSprite struct {
	Idle  []string
	Punch []string
	Kick  []string
	Block []string
	Hurt  []string
	Super []string
}

// Player 1 (CYBER-NINJA: Facing Right)
var p1Sprites = FighterSprite{
	Idle: []string{
		"   ▄████▄   ",
		"  █▒░▀█▀░▒█  ",
		" ▓█████████▓ ",
		" ░▒███████▒░ ",
		"   ██   ██   ",
		"  ▄██   ██▄  ",
	},
	Punch: []string{
		"   ▄████▄    ",
		"  █▒░▀█▀░▒█  ",
		" ▓█████████▓═╬►⚡",
		" ░▒███████▒░ ",
		"   ██   ██   ",
		"  ▄██   ██▄  ",
	},
	Kick: []string{
		"    ▄████▄   ",
		"   █▒░▀█▀░▒█ ",
		"  ▓████████▓ ",
		"  ░▒███████▒░",
		"   ██  ▄████▓►💥",
		"  ▄██        ",
	},
	Block: []string{
		"   ▄████▄    ",
		"  █▒░▀█▀░▒█  ",
		" ▓█████████▓ ",
		" ░▒███████▒░ ",
		"  ███║█║███  ",
		"  ▀██║█║██▀  ",
	},
	Hurt: []string{
		" 💥 ▄████▄   ",
		"   █>░▀█▀░<█ ",
		"   ▓███████▓ ",
		"   ░▒█████▒░ ",
		"    ██  ██   ",
		"   ▄██  ██▄  ",
	},
	Super: []string{
		" ⚡ ▄████▄  ⚡",
		"   █▓░▀█▀░▓█ ",
		" ▓█████████▓═╬►═════███⚡💥",
		" ░▒███████▒░═╬►═════███⚡💥",
		"   ██   ██   ",
		"  ▄██   ██▄  ",
	},
}

// Player 2 (WAR-MECH: Facing Left)
var p2Sprites = FighterSprite{
	Idle: []string{
		"   ▄████▄   ",
		"  █▒░▀█▀░▒█  ",
		" ▓█████████▓ ",
		" ░▒███████▒░ ",
		"   ██   ██   ",
		"  ▄██   ██▄  ",
	},
	Punch: []string{
		"    ▄████▄   ",
		"   █▒░▀█▀░▒█ ",
		"⚡◄╬═▓█████████▓",
		"  ░▒███████▒░",
		"    ██   ██  ",
		"   ▄██   ██▄ ",
	},
	Kick: []string{
		"   ▄████▄    ",
		"  █▒░▀█▀░▒█  ",
		"  ▓████████▓ ",
		" ░▒███████▒░ ",
		"💥◄▓████▄  ██ ",
		"         ██▄ ",
	},
	Block: []string{
		"    ▄████▄   ",
		"   █▒░▀█▀░▒█ ",
		"  ▓█████████▓",
		"  ░▒███████▒░",
		"   ███║█║███ ",
		"   ▀██║█║██▀ ",
	},
	Hurt: []string{
		"   ▄████▄  💥",
		"  █>░▀█▀░<█  ",
		"  ▓███████▓  ",
		"  ░▒█████▒░  ",
		"    ██  ██   ",
		"   ▄██  ██▄  ",
	},
	Super: []string{
		" ⚡ ▄████▄  ⚡",
		"  █▓░▀█▀░▓█  ",
		"💥⚡███═════◄╬═▓█████████▓",
		"💥⚡███═════◄╬═░▒███████▒░",
		"    ██   ██  ",
		"   ▄██   ██▄ ",
	},
}

const logoBanner = `
████████ ███████ ██████  ███    ███ ██ ███    ██  █████  ████████  ██████  ██████  
   ██    ██      ██   ██ ████  ████ ██ ████   ██ ██   ██    ██    ██    ██ ██   ██ 
   ██    █████   ██████  ██ ████ ██ ██ ██ ██  ██ ███████    ██    ██    ██ ██████  
   ██    ██      ██   ██ ██  ██  ██ ██ ██  ██ ██ ██   ██    ██    ██    ██ ██   ██ 
   ██    ███████ ██   ██ ██      ██ ██ ██   ████ ██   ██    ██     ██████  ██   ██ 
                     ═══ MECHA CYBER COMBAT TERMINAL ARENA ═══
`

// -----------------------------------------------------------------
// 2. RETRO ARCADE STYLES
// -----------------------------------------------------------------

var (
	logoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0055")).
			Bold(true)

	p1Color = lipgloss.NewStyle().Foreground(lipgloss.Color("#00F0FF")).Bold(true)
	p2Color = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF007F")).Bold(true)

	meterBorder = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#BD00FF")).
			Padding(0, 2)

	alertStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFE600"))

	spectatorBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A4B0BE")).
			Italic(true)
)

// -----------------------------------------------------------------
// 3. COMBAT STATE & MULTIPLAYER LOBBY
// -----------------------------------------------------------------

type ActionState int

const (
	StateIdle ActionState = iota
	StatePunch
	StateKick
	StateBlock
	StateHurt
	StateSuper
)

type Fighter struct {
	ID        int
	Name      string
	HP        int
	MaxHP     int
	Energy    int // 0 to 100 for Super
	Pos       int // Horizontal position
	State     ActionState
	StateTime int
	Combo     int
	Wins      int
}

type CombatState struct {
	P1           *Fighter
	P2           *Fighter
	Spectators   int
	Round        int
	RoundTime    int
	FightStarted bool
	AnnounceText string
	CombatLog    []string
}

type Lobby struct {
	mu          sync.Mutex
	subscribers map[int]chan CombatState
	nextID      int
	state       CombatState
}

var globalLobby = &Lobby{
	subscribers: make(map[int]chan CombatState),
	nextID:      1,
	state: CombatState{
		Round:        1,
		RoundTime:    99,
		FightStarted: false,
		AnnounceText: "WAITING FOR CHALLENGER (NEED 2 FIGHTERS)...",
		CombatLog:    []string{"⚡ Welcome to TERMINATOR: MECHA COMBAT!"},
	},
}

func init() {
	// Game Engine 10Hz Tick Loop
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		secondTick := 0
		for range ticker.C {
			globalLobby.gameTick(&secondTick)
		}
	}()
}

func (l *Lobby) gameTick(secondTick *int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	*secondTick++
	if *secondTick%10 == 0 && l.state.FightStarted && l.state.RoundTime > 0 {
		l.state.RoundTime--
		if l.state.RoundTime <= 0 {
			l.resolveTimeOut()
		}
	}

	if l.state.P1 != nil {
		if l.state.P1.StateTime > 0 {
			l.state.P1.StateTime--
			if l.state.P1.StateTime == 0 {
				l.state.P1.State = StateIdle
			}
		}
	}

	if l.state.P2 != nil {
		if l.state.P2.StateTime > 0 {
			l.state.P2.StateTime--
			if l.state.P2.StateTime == 0 {
				l.state.P2.State = StateIdle
			}
		}
	}

	l.broadcastUnlocked()
}

func (l *Lobby) Join(name string) (int, chan CombatState) {
	l.mu.Lock()
	defer l.mu.Unlock()

	id := l.nextID
	l.nextID++

	ch := make(chan CombatState, 8)
	l.subscribers[id] = ch

	if l.state.P1 == nil {
		l.state.P1 = &Fighter{
			ID:     id,
			Name:   name,
			HP:     100,
			MaxHP:  100,
			Pos:    6,
			State:  StateIdle,
			Energy: 20,
		}
		l.state.AnnounceText = fmt.Sprintf("🎮 %s JOINED AS P1 (CYBER-NINJA)! Waiting for P2...", name)
	} else if l.state.P2 == nil {
		l.state.P2 = &Fighter{
			ID:     id,
			Name:   name,
			HP:     100,
			MaxHP:  100,
			Pos:    arenaWidth - 20,
			State:  StateIdle,
			Energy: 20,
		}
		l.state.FightStarted = true
		l.state.RoundTime = 99
		l.state.AnnounceText = fmt.Sprintf("🔥 ROUND %d: %s VS %s — FIGHT!", l.state.Round, l.state.P1.Name, l.state.P2.Name)
	} else {
		l.state.Spectators++
		l.state.AnnounceText = fmt.Sprintf("👀 %s is spectating the battle!", name)
	}

	l.broadcastUnlocked()
	return id, ch
}

func (l *Lobby) Leave(id int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.subscribers, id)

	if l.state.P1 != nil && l.state.P1.ID == id {
		l.state.AnnounceText = fmt.Sprintf("🚪 %s (P1) forfeited the match!", l.state.P1.Name)
		l.state.P1 = nil
		l.state.FightStarted = false
	} else if l.state.P2 != nil && l.state.P2.ID == id {
		l.state.AnnounceText = fmt.Sprintf("🚪 %s (P2) forfeited the match!", l.state.P2.Name)
		l.state.P2 = nil
		l.state.FightStarted = false
	} else {
		if l.state.Spectators > 0 {
			l.state.Spectators--
		}
	}

	l.broadcastUnlocked()
}

func (l *Lobby) HandleInput(id int, key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.state.FightStarted || l.state.P1 == nil || l.state.P2 == nil {
		return
	}

	isP1 := l.state.P1.ID == id
	isP2 := l.state.P2.ID == id
	if !isP1 && !isP2 {
		return
	}

	attacker := l.state.P1
	defender := l.state.P2
	dir := 1
	if isP2 {
		attacker = l.state.P2
		defender = l.state.P1
		dir = -1
	}

	if attacker.State == StateHurt {
		return
	}

	dist := defender.Pos - attacker.Pos
	if dist < 0 {
		dist = -dist
	}

	switch key {
	// Movement
	case "left", "a":
		if attacker.Pos > 2 {
			attacker.Pos -= 2
		}
	case "right", "d":
		if (dir == 1 && attacker.Pos+14 < defender.Pos) || (dir == -1 && attacker.Pos < arenaWidth-16) {
			attacker.Pos += 2
		} else if dir == -1 && attacker.Pos-14 > defender.Pos {
			attacker.Pos -= 2
		}

	// Cyber Jab / Punch (J / 1)
	case "j", "1":
		attacker.State = StatePunch
		attacker.StateTime = 3
		if dist <= 16 {
			l.applyHit(attacker, defender, 8, 10, "CYBER JAB")
		}

	// Plasma Kick (K / 2)
	case "k", "2":
		attacker.State = StateKick
		attacker.StateTime = 4
		if dist <= 18 {
			l.applyHit(attacker, defender, 15, 15, "PLASMA KICK")
		}

	// Energy Barrier / Block (L / 3 / S)
	case "l", "3", "s", "down":
		attacker.State = StateBlock
		attacker.StateTime = 4

	// Mega Railgun Super Beam (Space / U / 4)
	case "space", "u", "4":
		if attacker.Energy >= 100 {
			attacker.State = StateSuper
			attacker.StateTime = 7
			attacker.Energy = 0
			if dist <= 35 {
				l.applyHit(attacker, defender, 35, 0, "💥 MEGA RAILGUN CANNON")
			}
			l.state.AnnounceText = fmt.Sprintf("⚡ %s FIRED THE MEGA RAILGUN CANNON!", attacker.Name)
		} else {
			l.state.AnnounceText = fmt.Sprintf("⚠️ %s: Super Energy not ready (%d/100)!", attacker.Name, attacker.Energy)
		}
	}

	l.broadcastUnlocked()
}

func (l *Lobby) applyHit(att *Fighter, def *Fighter, dmg int, energyGain int, attackName string) {
	att.Energy += energyGain
	if att.Energy > 100 {
		att.Energy = 100
	}

	if def.State == StateBlock {
		reducedDmg := dmg / 4
		def.HP -= reducedDmg
		att.Combo = 0
		def.Energy += 8
		l.state.CombatLog = append(l.state.CombatLog, fmt.Sprintf("🛡️ %s DEFLECTED %s's %s! (%d dmg)", def.Name, att.Name, attackName, reducedDmg))
	} else {
		def.HP -= dmg
		def.State = StateHurt
		def.StateTime = 4
		att.Combo++

		// Pushback
		if att.Pos < def.Pos && def.Pos < arenaWidth-16 {
			def.Pos += 3
		} else if att.Pos > def.Pos && def.Pos > 2 {
			def.Pos -= 3
		}

		logMsg := fmt.Sprintf("💥 %s struck %s with %s! (-%d HP)", att.Name, def.Name, attackName, dmg)
		if att.Combo > 1 {
			logMsg += fmt.Sprintf(" [COMBO x%d!]", att.Combo)
		}
		l.state.CombatLog = append(l.state.CombatLog, logMsg)
	}

	if len(l.state.CombatLog) > 4 {
		l.state.CombatLog = l.state.CombatLog[len(l.state.CombatLog)-4:]
	}

	if def.HP <= 0 {
		def.HP = 0
		att.Wins++
		l.state.AnnounceText = fmt.Sprintf("🏆 K.O.! %s DEFEATED %s IN ROUND %d!", att.Name, def.Name, l.state.Round)
		l.resetRound()
	}
}

func (l *Lobby) resolveTimeOut() {
	if l.state.P1 == nil || l.state.P2 == nil {
		return
	}
	if l.state.P1.HP > l.state.P2.HP {
		l.state.P1.Wins++
		l.state.AnnounceText = fmt.Sprintf("⏰ TIME UP! %s wins by health advantage!", l.state.P1.Name)
	} else if l.state.P2.HP > l.state.P1.HP {
		l.state.P2.Wins++
		l.state.AnnounceText = fmt.Sprintf("⏰ TIME UP! %s wins by health advantage!", l.state.P2.Name)
	} else {
		l.state.AnnounceText = "⏰ TIME UP! DRAW GAME!"
	}
	l.resetRound()
}

func (l *Lobby) resetRound() {
	l.state.Round++
	l.state.RoundTime = 99
	if l.state.P1 != nil {
		l.state.P1.HP = 100
		l.state.P1.Pos = 6
		l.state.P1.State = StateIdle
		l.state.P1.Combo = 0
	}
	if l.state.P2 != nil {
		l.state.P2.HP = 100
		l.state.P2.Pos = arenaWidth - 20
		l.state.P2.State = StateIdle
		l.state.P2.Combo = 0
	}
}

func (l *Lobby) broadcastUnlocked() {
	for _, ch := range l.subscribers {
		select {
		case ch <- l.state:
		default:
		}
	}
}

// -----------------------------------------------------------------
// 4. BUBBLE TEA CLIENT & GRAPHICAL ENGINE
// -----------------------------------------------------------------

type model struct {
	myID      int
	myName    string
	lobbyCh   chan CombatState
	state     CombatState
	termWidth int
}

func waitForCombatUpdate(ch chan CombatState) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m model) Init() tea.Cmd {
	return waitForCombatUpdate(m.lobbyCh)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CombatState:
		m.state = msg
		return m, waitForCombatUpdate(m.lobbyCh)

	case tea.WindowSizeMsg:
		m.termWidth = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		default:
			globalLobby.HandleInput(m.myID, msg.String())
		}
	}
	return m, nil
}

// -----------------------------------------------------------------
// 5. VIEW (HIGH-DEF SPRITES, HEALTH BARS, 2D ARENA)
// -----------------------------------------------------------------

func (m model) View() string {
	var b strings.Builder

	// Logo
	b.WriteString(logoStyle.Render(logoBanner))
	b.WriteString("\n")

	// P1 & P2 Info
	p1Name := "WAITING..."
	p1HP := 0
	p1Energy := 0
	p1Wins := 0
	if m.state.P1 != nil {
		p1Name = m.state.P1.Name
		p1HP = m.state.P1.HP
		p1Energy = m.state.P1.Energy
		p1Wins = m.state.P1.Wins
	}

	p2Name := "WAITING..."
	p2HP := 0
	p2Energy := 0
	p2Wins := 0
	if m.state.P2 != nil {
		p2Name = m.state.P2.Name
		p2HP = m.state.P2.HP
		p2Energy = m.state.P2.Energy
		p2Wins = m.state.P2.Wins
	}

	// 20-segment colored bar
	renderBar := func(val int, max int, fillChar string, color string) string {
		segments := (val * 20) / max
		if segments < 0 {
			segments = 0
		}
		if segments > 20 {
			segments = 20
		}
		filled := strings.Repeat(fillChar, segments)
		empty := strings.Repeat("░", 20-segments)
		return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(filled) +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#444444")).Render(empty)
	}

	p1Bar := renderBar(p1HP, 100, "█", "#00F0FF")
	p2Bar := renderBar(p2HP, 100, "█", "#FF007F")

	p1SuperBar := renderBar(p1Energy, 100, "⚡", "#FFE600")
	p2SuperBar := renderBar(p2Energy, 100, "⚡", "#FFE600")

	// Match HUD
	hudLeft := fmt.Sprintf("%s (W:%d)\nHP [%s] %03d\nSP [%s] %03d",
		p1Color.Render("P1: "+p1Name), p1Wins, p1Bar, p1HP, p1SuperBar, p1Energy)

	timerBlock := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFE600")).
		Background(lipgloss.Color("#2D1B69")).
		Padding(0, 2).
		Render(fmt.Sprintf("⏱️ %02d\nROUND %d", m.state.RoundTime, m.state.Round))

	hudRight := fmt.Sprintf("%s (W:%d)\nHP [%s] %03d\nSP [%s] %03d",
		p2Color.Render("P2: "+p2Name), p2Wins, p2Bar, p2HP, p2SuperBar, p2Energy)

	hud := lipgloss.JoinHorizontal(lipgloss.Center, hudLeft, "      ", timerBlock, "      ", hudRight)
	b.WriteString(meterBorder.Render(hud))
	b.WriteString("\n\n")

	// Announcement Banner
	b.WriteString(alertStyle.Render(fmt.Sprintf("  🔔 %s", m.state.AnnounceText)) + "\n\n")

	// 7-line high stage
	stageLines := [7]string{
		strings.Repeat(" ", arenaWidth),
		strings.Repeat(" ", arenaWidth),
		strings.Repeat(" ", arenaWidth),
		strings.Repeat(" ", arenaWidth),
		strings.Repeat(" ", arenaWidth),
		strings.Repeat(" ", arenaWidth),
		strings.Repeat("═", arenaWidth), // Floor line
	}

	// Stamp P1
	if m.state.P1 != nil {
		sprite := p1Sprites.Idle
		switch m.state.P1.State {
		case StatePunch:
			sprite = p1Sprites.Punch
		case StateKick:
			sprite = p1Sprites.Kick
		case StateBlock:
			sprite = p1Sprites.Block
		case StateHurt:
			sprite = p1Sprites.Hurt
		case StateSuper:
			sprite = p1Sprites.Super
		}
		stampSprite7(&stageLines, sprite, m.state.P1.Pos, p1Color)
	}

	// Stamp P2
	if m.state.P2 != nil {
		sprite := p2Sprites.Idle
		switch m.state.P2.State {
		case StatePunch:
			sprite = p2Sprites.Punch
		case StateKick:
			sprite = p2Sprites.Kick
		case StateBlock:
			sprite = p2Sprites.Block
		case StateHurt:
			sprite = p2Sprites.Hurt
		case StateSuper:
			sprite = p2Sprites.Super
		}
		stampSprite7(&stageLines, sprite, m.state.P2.Pos, p2Color)
	}

	// Render Arena Box
	arenaContent := strings.Join([]string{
		stageLines[0], stageLines[1], stageLines[2],
		stageLines[3], stageLines[4], stageLines[5], stageLines[6],
	}, "\n")

	b.WriteString(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#BD00FF")).
		Padding(1, 2).
		Width(arenaWidth + 6).
		Render(arenaContent))
	b.WriteString("\n\n")

	// Combat Log & Controls
	logContent := "📜 COMBAT LOG:\n" + strings.Join(m.state.CombatLog, "\n")
	controlsContent := "🎮 CONTROLS:\n" +
		"• Move: [A] Left / [D] Right (or ← / →)\n" +
		"• Attack: [J] Cyber Jab  |  [K] Plasma Kick\n" +
		"• Defense: [L] Energy Barrier (Block)\n" +
		"• SUPER: [SPACE] Mega Railgun (100 SP)"

	bottomBar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(45).Render(logContent),
		"  │  ",
		lipgloss.NewStyle().Width(42).Render(controlsContent),
	)
	b.WriteString(bottomBar + "\n\n")

	// Current Role
	role := spectatorBadge.Render("👁️ You are currently Spectating.")
	if m.state.P1 != nil && m.state.P1.ID == m.myID {
		role = p1Color.Render("⚔️ You are PLAYER 1 [Cyber-Ninja]")
	} else if m.state.P2 != nil && m.state.P2.ID == m.myID {
		role = p2Color.Render("⚔️ You are PLAYER 2 [War-Mech]")
	}
	b.WriteString(fmt.Sprintf("%s   [Press 'Q' to quit]\n", role))

	return b.String()
}

func stampSprite7(stage *[7]string, sprite []string, pos int, style lipgloss.Style) {
	for row := 0; row < 6; row++ {
		line := []rune(stage[row])
		spriteRunes := []rune(style.Render(sprite[row]))

		start := pos
		if start < 0 {
			start = 0
		}
		for i, r := range spriteRunes {
			if start+i < len(line) {
				line[start+i] = r
			}
		}
		stage[row] = string(line)
	}
}

// -----------------------------------------------------------------
// 6. SSH SERVER BOOTSTRAP
// -----------------------------------------------------------------

func sshHandler(s ssh.Session) {
	_, winCh, isPty := s.Pty()
	if !isPty {
		fmt.Fprintln(s, "Terminal (PTY) required to play.")
		return
	}

	name := s.User()
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("Cyborg%d", rand.Intn(999))
	}

	playerID, lobbyCh := globalLobby.Join(name)
	defer globalLobby.Leave(playerID)

	m := model{
		myID:      playerID,
		myName:    name,
		lobbyCh:   lobbyCh,
		state:     globalLobby.state,
		termWidth: 80,
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
	server.SetOption(ssh.HostKeyFile("host_key"))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("🥊 MECHA COMBAT ARENA live on port %s...", port)
		log.Printf("👉 Connect: ssh -p %s <your_name>@localhost", port)
		if err := server.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down Combat Arena...")
}