package main

import (
	"context"
	"deleter/internal/auth"
	"deleter/internal/config"
	"deleter/internal/twitter"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

// ASCII Art Logo
const logo = `
██████╗ ███████╗██╗     ███████╗████████╗███████╗██████╗ 
██╔══██╗██╔════╝██║     ██╔════╝╚══██╔══╝██╔════╝██╔══██╗
██║  ██║█████╗  ██║     █████╗     ██║   █████╗  ██████╔╝
██║  ██║██╔══╝  ██║     ██╔══╝     ██║   ██╔══╝  ██╔══██╗
██████╔╝███████╗███████╗███████╗   ██║   ███████╗██║  ██║
╚═════╝ ╚══════╝╚══════╝╚══════╝   ╚═╝   ╚══════╝╚═╝  ╚═╝
`

// Styles - compact
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B")).
			MarginTop(0).
			MarginBottom(0)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4ECDC4")).
			MarginBottom(0)

	menuItemStyle = lipgloss.NewStyle().
			PaddingLeft(1).
			PaddingRight(1).
			PaddingTop(0).
			PaddingBottom(0).
			Inline(true)

	selectedMenuItemStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				PaddingRight(1).
				PaddingTop(0).
				PaddingBottom(0).
				Foreground(lipgloss.Color("#000000")).
				Background(lipgloss.Color("#4ECDC4")).
				Bold(true).
				Inline(true)

	descriptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Italic(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFE66D"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2ECC71"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E74C3C"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4ECDC4")).
			Padding(0, 1).
			MarginTop(0).
			MarginBottom(1)

	logEntryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

	logDeletedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2ECC71"))

	logErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E74C3C"))
)

// Screen states
type screen int

const (
	screenWelcome     screen = iota
	screenSetup              // Initial setup wizard
	screenSetupStep1         // Extract.js instruction
	screenSetupStep2         // Paste extracted data
	screenSetupStep3         // Enter auth_token
	screenSetupStep4         // Enter kdt
	screenSetupVerify        // Verify and save
	screenMainMenu
	screenCleanData
	screenAddKeywords
	screenSetDate
	screenCleaning
	screenResults
	screenSession
)

// Menu items
type menuItem struct {
	title       string
	description string
}

// Setup data collection
type setupData struct {
	userID               string
	ct0                  string
	guestID              string
	queryIDUserTweets    string
	queryIDDeleteRetweet string
	authToken            string
	kdt                  string
}

// Model for Bubble Tea
type model struct {
	screen       screen
	menuIndex    int
	menuItems    []menuItem
	cfg          *config.Config
	client       *twitter.Client
	stats        twitter.Stats
	textInput    textinput.Model
	keywords     []string
	deleteDate   string
	logs         []twitter.LogEntry
	logChan      chan twitter.LogEntry
	cleaningDone bool
	cleanCancel  context.CancelFunc
	sessionMgr   *auth.SessionManager
	sessionDays  int
	fromSession  bool // loaded from session or .env
	setup        setupData
	setupStep    int
	err          error
	width        int
	height       int
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Enter keywords: GIVEAWAY, RUNE, NFT"
	ti.Focus()
	ti.Width = 50

	sm := auth.NewSessionManager(".")

	return model{
		screen:      screenWelcome,
		menuIndex:   0,
		menuItems:   []menuItem{},
		textInput:   ti,
		keywords:    []string{},
		deleteDate:  "",
		logs:        []twitter.LogEntry{},
		logChan:     make(chan twitter.LogEntry, 100),
		sessionMgr:  sm,
		sessionDays: 0,
		fromSession: false,
		setup:       setupData{},
		setupStep:   0,
	}
}

func (m model) Init() tea.Cmd {
	// Load config
	_ = godotenv.Load()

	// Пробуем загрузить из сессии
	cfg, err := config.LoadFromSession(m.sessionMgr)
	if err == nil && cfg != nil {
		// Получаем инфо о сессии
		session, _ := m.sessionMgr.Load()
		if session != nil {
			return func() tea.Msg {
				return configLoadedMsg{cfg: cfg, fromSession: true, sessionDays: session.DaysUntilExpiry()}
			}
		}
	}

	// Fallback на .env
	cfg, err = config.LoadFromEnv()
	if err != nil {
		// Нет конфига - переходим к setup
		return func() tea.Msg { return needSetupMsg{} }
	}
	return func() tea.Msg { return configLoadedMsg{cfg: cfg, fromSession: false, sessionDays: 0} }
}

type needSetupMsg struct{}

type configLoadedMsg struct {
	cfg         *config.Config
	fromSession bool
	sessionDays int
}

type configErrMsg struct {
	err error
}

type sessionSavedMsg struct{}
type sessionClearedMsg struct{}

type cleaningDoneMsg struct {
	stats twitter.Stats
	err   error
}

type interruptMsg struct{}

type logMsg twitter.LogEntry

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q":
			if m.screen == screenCleaning && m.cleanCancel != nil {
				m.cleanCancel()
				m.cleaningDone = true
				m.screen = screenCleanData
				return m, nil
			}
			if m.screen != screenCleaning {
				return m, tea.Quit
			}
		case "esc":
			switch m.screen {
			case screenCleaning:
				if m.cleanCancel != nil {
					m.cleanCancel()
				}
				m.cleaningDone = true
				m.screen = screenCleanData
				return m, nil
			case screenMainMenu:
				return m, tea.Quit
			case screenSetup:
				return m, tea.Quit
			case screenSetupStep1:
				m.screen = screenSetup
				return m, nil
			case screenSetupStep2:
				m.screen = screenSetupStep1
				return m, nil
			case screenSetupStep3:
				m.screen = screenSetupStep2
				m.textInput.SetValue("")
				m.textInput.Focus()
				return m, nil
			case screenSetupStep4:
				m.screen = screenSetupStep3
				m.textInput.SetValue(m.setup.authToken)
				m.textInput.Focus()
				return m, nil
			case screenSetupVerify:
				m.screen = screenSetupStep4
				return m, nil
			case screenCleanData, screenAddKeywords, screenSetDate:
				m.screen = screenMainMenu
				m.menuIndex = 0
				m.menuItems = mainMenuItems()
				return m, nil
			case screenResults:
				m.screen = screenMainMenu
				m.menuIndex = 0
				m.menuItems = mainMenuItems()
				return m, nil
			}
		case "up", "k":
			if m.menuIndex > 0 {
				m.menuIndex--
			}
		case "down", "j":
			if m.menuIndex < len(m.menuItems)-1 {
				m.menuIndex++
			}
		case "enter":
			return m.handleSelection()
		}

	case twitter.LogEntry:
		m.logs = append(m.logs, msg)
		// Keep only last 20 logs
		if len(m.logs) > 20 {
			m.logs = m.logs[1:]
		}
		return m, m.waitForLog()

	case needSetupMsg:
		m.screen = screenSetup
		return m, nil

	case configLoadedMsg:
		m.cfg = msg.cfg
		m.keywords = msg.cfg.Keywords
		m.deleteDate = msg.cfg.DeleteBeforeDate
		m.fromSession = msg.fromSession
		m.sessionDays = msg.sessionDays
		m.client = twitter.NewClient(msg.cfg)
		m.screen = screenWelcome
		m.menuItems = welcomeMenuItems()
		// Если загрузились из .env, сразу сохраняем в сессию
		if !m.fromSession {
			_ = m.cfg.SaveToSession(m.sessionMgr)
		}
		return m, nil

	case sessionSavedMsg:
		// Обновляем статус
		session, _ := m.sessionMgr.Load()
		if session != nil {
			m.sessionDays = session.DaysUntilExpiry()
			m.fromSession = true
		}
		return m, nil

	case sessionClearedMsg:
		m.fromSession = false
		m.sessionDays = 0
		m.cfg = nil
		m.client = nil
		// After clearing session, redirect to setup wizard
		m.screen = screenSetup
		return m, nil

	case configErrMsg:
		m.err = msg.err
		return m, tea.Quit

	case logMsg:
		entry := twitter.LogEntry(msg)
		// Check for completion signals
		if entry.Status == "__DONE__" {
			m.cleaningDone = true
			m.screen = screenResults
			return m, nil
		}
		if entry.Status == "__ERROR__" {
			m.err = fmt.Errorf("cleaning error: %s", entry.Error)
			return m, tea.Quit
		}
		// Regular log entry
		m.logs = append(m.logs, entry)
		return m, m.waitForLog()

	case cleaningDoneMsg:
		m.stats = msg.stats
		m.cleaningDone = true
		m.screen = screenResults
		return m, nil

	case interruptMsg:
		// Handle Ctrl+C during cleaning
		if m.screen == screenCleaning && m.cleanCancel != nil {
			m.cleanCancel()
			m.cleaningDone = true
			m.screen = screenCleanData
			return m, nil
		}
		return m, tea.Quit
	}

	// Handle text input for various screens
	if m.screen == screenAddKeywords || m.screen == screenSetDate ||
		m.screen == screenSetupStep2 || m.screen == screenSetupStep3 || m.screen == screenSetupStep4 {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) handleSelection() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenSetup:
		m.screen = screenSetupStep1
		return m, nil

	case screenSetupStep1:
		m.screen = screenSetupStep2
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.textInput.Placeholder = "Paste data from extract.js here"
		return m, nil

	case screenSetupStep2:
		// Parse extracted data: user_id|ct0|guest_id|query_id_user_tweets|query_id_delete_retweet
		input := m.textInput.Value()
		parts := strings.Split(input, "|")
		if len(parts) >= 5 {
			m.setup.userID = parts[0]
			m.setup.ct0 = parts[1]
			m.setup.guestID = parts[2]
			m.setup.queryIDUserTweets = parts[3]
			m.setup.queryIDDeleteRetweet = parts[4]
			// Also save to cookies struct for session
			m.setup.authToken = "" // Will be entered in next step
			m.setup.kdt = ""
		}
		m.screen = screenSetupStep3
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.textInput.Placeholder = "Paste auth_token here"
		return m, nil

	case screenSetupStep3:
		m.setup.authToken = strings.TrimSpace(m.textInput.Value())
		m.screen = screenSetupStep4
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.textInput.Placeholder = "Paste kdt here (or leave empty)"
		return m, nil

	case screenSetupStep4:
		m.setup.kdt = strings.TrimSpace(m.textInput.Value())
		m.screen = screenSetupVerify
		return m, nil

	case screenSetupVerify:
		// Save all data to session
		session := &auth.Session{
			UserID:               m.setup.userID,
			QueryIDUserTweets:    m.setup.queryIDUserTweets,
			QueryIDDeleteRetweet: m.setup.queryIDDeleteRetweet,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		}
		// Build cookies map
		session.Cookies = map[string]interface{}{
			"auth_token": m.setup.authToken,
			"ct0":        m.setup.ct0,
			"twid":       "u=" + m.setup.userID,
			"kdt":        m.setup.kdt,
			"guest_id":   m.setup.guestID,
		}
		// Build headers
		session.Headers = map[string]interface{}{
			"accept":                    "*/*",
			"accept-language":           "en",
			"authorization":             "Bearer AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA",
			"content-type":              "application/json",
			"origin":                    "https://x.com",
			"referer":                   "https://x.com/i/user/" + m.setup.userID,
			"sec-ch-ua":                 `"Not_A Brand";v="8", "Chromium";v="120"`,
			"sec-ch-ua-mobile":          "?0",
			"sec-ch-ua-platform":        `"macOS"`,
			"sec-fetch-dest":            "empty",
			"sec-fetch-mode":            "cors",
			"sec-fetch-site":            "same-origin",
			"user-agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
			"x-csrf-token":              m.setup.ct0,
			"x-twitter-active-user":     "yes",
			"x-twitter-auth-type":       "OAuth2Session",
			"x-twitter-client-language": "en",
		}

		if err := m.sessionMgr.Save(session); err != nil {
			m.err = err
			return m, nil
		}

		// Load the newly created session
		cfg, err := config.LoadFromSession(m.sessionMgr)
		if err != nil {
			m.err = err
			return m, nil
		}

		m.cfg = cfg
		m.keywords = cfg.Keywords
		m.deleteDate = cfg.DeleteBeforeDate
		m.fromSession = true
		m.sessionDays = 14
		m.client = twitter.NewClient(cfg)
		m.screen = screenWelcome
		m.menuItems = welcomeMenuItems()
		return m, nil

	case screenWelcome:
		// If no session, redirect to setup wizard
		if !m.fromSession || m.cfg == nil {
			m.screen = screenSetup
		} else {
			m.screen = screenMainMenu
			m.menuIndex = 0
			m.menuItems = mainMenuItems()
		}

	case screenMainMenu:
		switch m.menuIndex {
		case 0: // Clean Data
			m.screen = screenCleanData
			m.menuIndex = 0
			m.menuItems = cleanDataMenuItems()
		case 1: // Session Management
			m.screen = screenSession
			m.menuIndex = 0
			m.menuItems = sessionMenuItems(m.fromSession)
		case 2: // Exit
			return m, tea.Quit
		}

	case screenSession:
		switch m.menuIndex {
		case 0: // Clear Session / Add New Session
			if m.fromSession {
				// Clear session - will redirect to setup wizard
				_ = m.sessionMgr.Delete()
				return m, func() tea.Msg { return sessionClearedMsg{} }
			} else {
				// No session - go to setup wizard to add new
				m.screen = screenSetup
				return m, nil
			}
		case 1: // Back
			m.screen = screenMainMenu
			m.menuIndex = 0
			m.menuItems = mainMenuItems()
		}

	case screenCleanData:
		switch m.menuIndex {
		case 0: // Start Cleaning
			// Block cleaning without valid session
			if !m.fromSession || m.cfg == nil {
				m.screen = screenSetup
				return m, nil
			}
			m.screen = screenCleaning
			m.logs = []twitter.LogEntry{}
			return m, tea.Batch(m.startCleaning(), m.waitForLog(), m.listenForSignals())
		case 1: // Add Keywords
			m.screen = screenAddKeywords
			m.textInput.SetValue(strings.Join(m.keywords, ","))
			m.textInput.Focus()
		case 2: // Set Date Filter
			m.screen = screenSetDate
			m.textInput.SetValue(m.deleteDate)
			m.textInput.Placeholder = "YYYY-MM-DD (empty = no filter)"
			m.textInput.Focus()
		case 3: // Back
			m.screen = screenMainMenu
			m.menuIndex = 0
			m.menuItems = mainMenuItems()
		}

	case screenAddKeywords:
		// Save keywords
		input := m.textInput.Value()
		if input != "" {
			m.keywords = parseKeywords(input)
			m.cfg.Keywords = m.keywords
		}
		m.screen = screenCleanData
		m.menuIndex = 0
		m.menuItems = cleanDataMenuItems()
		return m, nil

	case screenSetDate:
		// Save date filter
		input := m.textInput.Value()
		m.deleteDate = strings.TrimSpace(input)
		m.cfg.DeleteBeforeDate = m.deleteDate
		m.screen = screenCleanData
		m.menuIndex = 0
		m.menuItems = cleanDataMenuItems()
		return m, nil

	case screenResults:
		m.screen = screenCleanData
		m.menuIndex = 0
		m.menuItems = cleanDataMenuItems()
	}

	return m, nil
}

func (m *model) startCleaning() tea.Cmd {
	// Update config with current settings
	m.cfg.Keywords = m.keywords
	m.cfg.DeleteBeforeDate = m.deleteDate

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	m.cleanCancel = cancel

	// Run in goroutine to not block UI
	go func() {
		stats := &twitter.Stats{}
		err := m.client.CleanFeedWithLogs(ctx, stats, m.logChan)
		// Send completion signal
		if err != nil {
			m.logChan <- twitter.LogEntry{Status: "__ERROR__", Error: err.Error()}
		} else {
			m.logChan <- twitter.LogEntry{Status: "__DONE__"}
		}
	}()

	// Return immediately, start waiting for logs
	return m.waitForLog()
}

func (m model) waitForLog() tea.Cmd {
	return func() tea.Msg {
		return <-m.logChan
	}
}

func (m model) listenForSignals() tea.Cmd {
	return func() tea.Msg {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		return interruptMsg{}
	}
}

func (m model) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n", m.err))
	}

	var s strings.Builder

	// Always show logo
	s.WriteString(titleStyle.Render(logo))
	s.WriteString("\n")

	switch m.screen {
	case screenSetup:
		s.WriteString(m.viewSetup())
	case screenSetupStep1:
		s.WriteString(m.viewSetupStep1())
	case screenSetupStep2:
		s.WriteString(m.viewSetupStep2())
	case screenSetupStep3:
		s.WriteString(m.viewSetupStep3())
	case screenSetupStep4:
		s.WriteString(m.viewSetupStep4())
	case screenSetupVerify:
		s.WriteString(m.viewSetupVerify())
	case screenWelcome:
		s.WriteString(m.viewWelcome())
	case screenMainMenu:
		s.WriteString(m.viewMainMenu())
	case screenCleanData:
		s.WriteString(m.viewCleanData())
	case screenAddKeywords:
		s.WriteString(m.viewAddKeywords())
	case screenSetDate:
		s.WriteString(m.viewSetDate())
	case screenCleaning:
		s.WriteString(m.viewCleaning())
	case screenResults:
		s.WriteString(m.viewResults())
	case screenSession:
		s.WriteString(m.viewSession())
	}

	return s.String()
}

func (m model) viewWelcome() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Welcome to DELETER"))
	s.WriteString("\n")

	desc := `Twitter/X timeline cleaner - removes retweets matching keywords.`

	s.WriteString(descriptionStyle.Render(desc))
	s.WriteString("\n")

	// Show current config and session status
	if m.fromSession && m.cfg != nil {
		dateInfo := m.deleteDate
		if dateInfo == "" {
			dateInfo = "no filter"
		}
		sessionInfo := fmt.Sprintf("✓ Session active (%d days left)", m.sessionDays)
		info := fmt.Sprintf("User: %s | Keywords: %s | Date: %s | %s",
			truncate(m.cfg.UserID, 15), strings.Join(m.keywords, ", "), dateInfo, sessionInfo)
		s.WriteString(boxStyle.Render(info))
		s.WriteString("\n")
	} else {
		s.WriteString(boxStyle.Render("⚠️  No active session - setup required"))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("Press ENTER to start setup wizard"))
		s.WriteString("\n")
	}

	s.WriteString("Press ENTER to continue or Q to quit\n")

	return s.String()
}

func (m model) viewMainMenu() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Main Menu"))
	s.WriteString("\n")

	// Show session status
	if m.fromSession && m.cfg != nil {
		s.WriteString(infoStyle.Render(fmt.Sprintf("✓ Session active | User: %s | %d days left", truncate(m.cfg.UserID, 10), m.sessionDays)))
		s.WriteString("\n")
	} else {
		s.WriteString(errorStyle.Render("⚠️  No session - select 'Session' to setup"))
		s.WriteString("\n")
	}
	s.WriteString("\n")

	for i, item := range m.menuItems {
		if i == m.menuIndex {
			s.WriteString(selectedMenuItemStyle.Render("▶ " + item.title))
		} else {
			s.WriteString(menuItemStyle.Render("  " + item.title))
		}
		s.WriteString("\n")
		s.WriteString(descriptionStyle.Render("    " + item.description))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(infoStyle.Render("↑/↓ or j/k: Navigate  •  Enter: Select  •  ESC/Q: Exit"))

	return s.String()
}

func (m model) viewCleanData() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Clean Data - Personal Account"))
	s.WriteString("\n")

	// Show warning if no session
	if !m.fromSession || m.cfg == nil {
		s.WriteString(errorStyle.Render("⚠️  No active session! 'Start Cleaning' will launch setup wizard."))
		s.WriteString("\n")
	}

	dateInfo := m.deleteDate
	if dateInfo == "" {
		dateInfo = "no date filter"
	}
	info := fmt.Sprintf("Keywords: %s | Before: %s", strings.Join(m.keywords, ", "), dateInfo)
	s.WriteString(boxStyle.Render(info))
	s.WriteString("\n")

	for i, item := range m.menuItems {
		if i == m.menuIndex {
			s.WriteString(selectedMenuItemStyle.Render("▶ " + item.title))
		} else {
			s.WriteString(menuItemStyle.Render("  " + item.title))
		}
		s.WriteString("\n")
	}

	s.WriteString(infoStyle.Render("↑/↓: Navigate  •  Enter: Select  •  ESC: Back  •  Q: Exit"))

	return s.String()
}

func (m model) viewAddKeywords() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Add Keywords"))
	s.WriteString("\n")

	s.WriteString("Keywords (comma-separated):\n")
	s.WriteString(m.textInput.View())
	s.WriteString("\n")
	s.WriteString(infoStyle.Render("Enter: Save  •  ESC: Cancel"))

	return s.String()
}

func (m model) viewSetDate() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Set Date Filter"))
	s.WriteString("\n")

	s.WriteString("Delete retweets BEFORE this date (YYYY-MM-DD):\n")
	s.WriteString("Leave empty to disable date filter\n")
	s.WriteString(m.textInput.View())
	s.WriteString("\n")
	s.WriteString(infoStyle.Render("Enter: Save  •  ESC: Cancel"))

	return s.String()
}

func (m model) viewCleaning() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Cleaning in Progress"))
	s.WriteString("\n")

	// Show last logs (compact single line format)
	if len(m.logs) > 0 {
		s.WriteString("\n")
		start := len(m.logs) - 15
		if start < 0 {
			start = 0
		}
		for _, entry := range m.logs[start:] {
			// Colorful status in brackets
			var statusStyled string
			switch entry.Status {
			case "DELETED":
				statusStyled = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71")).Bold(true).Render("[DELETED]")
			case "ERROR":
				statusStyled = lipgloss.NewStyle().Foreground(lipgloss.Color("#E74C3C")).Bold(true).Render("[ERROR]")
			default:
				statusStyled = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Render("[" + entry.Status + "]")
			}

			line := fmt.Sprintf("%s %s %s: %s",
				logEntryStyle.Render(entry.Timestamp),
				statusStyled,
				logEntryStyle.Render(truncate(entry.TweetID, 15)),
				entry.Preview)
			s.WriteString(line)
			s.WriteString("\n")
		}
	}

	if !m.cleaningDone {
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("Processing... Press Q or ESC to stop and return to menu"))
	}

	return s.String()
}

func (m model) viewSession() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Session Management"))
	s.WriteString("\n")

	if m.fromSession {
		s.WriteString(boxStyle.Render(fmt.Sprintf("✓ Session saved | %d days until expiry", m.sessionDays)))
		s.WriteString(fmt.Sprintf("\nYour credentials are stored locally and will expire in %d days.\n", m.sessionDays))
		s.WriteString(infoStyle.Render("Select 'Clear Session' to remove credentials and setup new ones."))
	} else {
		s.WriteString(boxStyle.Render("⚠️  No active session"))
		s.WriteString("\nTwitter credentials are required to use this application.\n")
		s.WriteString(infoStyle.Render("Select 'Add New Session' to launch the setup wizard."))
	}

	s.WriteString("\n")

	for i, item := range m.menuItems {
		if i == m.menuIndex {
			s.WriteString(selectedMenuItemStyle.Render("▶ " + item.title))
		} else {
			s.WriteString(menuItemStyle.Render("  " + item.title))
		}
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(infoStyle.Render("↑/↓: Navigate  •  Enter: Select  •  ESC: Back  •  Q: Exit"))

	return s.String()
}

func (m model) viewResults() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Cleaning Complete!"))
	s.WriteString("\n\n")

	results := fmt.Sprintf(`Statistics:

  Scanned:   %d tweets
  Retweets:  %d found
  Matched:   %d with keywords
  Deleted:   %d removed
  Errors:    %d`,
		m.stats.Scanned, m.stats.Retweets, m.stats.Matched,
		m.stats.Deleted, m.stats.Errors)

	if m.stats.Errors == 0 && m.stats.Deleted > 0 {
		s.WriteString(boxStyle.Render(results))
	} else if m.stats.Errors > 0 {
		s.WriteString(boxStyle.Render(results))
	} else {
		s.WriteString(boxStyle.Render(results))
	}

	s.WriteString("\n\n")
	s.WriteString(infoStyle.Render("Enter: Return to menu  •  ESC/Q: Exit"))

	return s.String()
}

// Menu definitions
func welcomeMenuItems() []menuItem {
	return []menuItem{
		{"Continue", "Proceed to main menu"},
	}
}

func mainMenuItems() []menuItem {
	return []menuItem{
		{"Clean Data", "Remove retweets from your timeline by keywords"},
		{"Session", "Manage saved authentication session"},
		{"Exit", "Quit the application"},
	}
}

func sessionMenuItems(saved bool) []menuItem {
	if saved {
		return []menuItem{
			{"Clear Session", "Remove saved credentials and setup new session"},
			{"Back", "Return to main menu"},
		}
	}
	// No session - show option to add new
	return []menuItem{
		{"Add New Session", "Setup Twitter credentials using the wizard"},
		{"Back", "Return to main menu"},
	}
}

func cleanDataMenuItems() []menuItem {
	return []menuItem{
		{"Start Cleaning", "Begin scanning and deleting retweets"},
		{"Add Keywords", "Add new keywords to filter"},
		{"Set Date Filter", "Delete only retweets before specific date"},
		{"Back", "Return to main menu"},
	}
}

func parseKeywords(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Setup wizard views

func (m model) viewSetup() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Initial Setup Required"))
	s.WriteString("\n\n")

	s.WriteString("No saved session found. Let's set up your Twitter credentials.\n\n")

	s.WriteString(boxStyle.Render(
		"Required parameters:\n" +
			"  • User ID (from twid cookie)\n" +
			"  • ct0 (CSRF token)\n" +
			"  • guest_id\n" +
			"  • Query IDs (UserTweets, DeleteRetweet)\n" +
			"  • auth_token (HttpOnly - manual input)\n" +
			"  • kdt (HttpOnly - manual input)",
	))
	s.WriteString("\n\n")

	s.WriteString(infoStyle.Render("We'll use a browser script to extract most data automatically."))
	s.WriteString("\n\n")

	s.WriteString("Press ENTER to start setup or Q to quit\n")

	return s.String()
}

func (m model) viewSetupStep1() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Step 1: Extract Data from Browser"))
	s.WriteString("\n\n")

	instr := `1. Open x.com in your browser and login
2. Press F12 → Console tab
3. Copy the contents of extract.js file
4. Paste into console and press Enter
5. Copy the output line (auto-copied to clipboard)
6. Return here and paste it in the next step`

	s.WriteString(boxStyle.Render(instr))
	s.WriteString("\n\n")

	s.WriteString(infoStyle.Render("extract.js is located in: " + getProjectPath()))
	s.WriteString("\n\n")

	s.WriteString("Press ENTER when ready to paste the extracted data\n")
	s.WriteString("Or press ESC to go back\n")

	return s.String()
}

func (m model) viewSetupStep2() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Step 2: Paste Extracted Data"))
	s.WriteString("\n\n")

	s.WriteString("Paste the line copied from the browser console:\n")
	s.WriteString("(Format: user_id|ct0|guest_id|query_id_user_tweets|query_id_delete_retweet)")
	s.WriteString("\n\n")

	s.WriteString(m.textInput.View())
	s.WriteString("\n\n")

	if m.setup.userID != "" {
		s.WriteString(successStyle.Render("✓ Data parsed successfully"))
		s.WriteString("\n")
	}

	s.WriteString(infoStyle.Render("Press ENTER to continue • ESC to go back"))

	return s.String()
}

func (m model) viewSetupStep3() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Step 3: Enter auth_token"))
	s.WriteString("\n\n")

	instr := `The auth_token is an HttpOnly cookie and cannot be extracted by JavaScript.

How to get it:
1. In DevTools, click Application tab
2. In left sidebar: Storage → Cookies → https://x.com
3. Find "auth_token" in the table
4. Double-click the Value cell and copy it
5. Paste below`

	s.WriteString(boxStyle.Render(instr))
	s.WriteString("\n\n")

	s.WriteString("Paste auth_token:\n")
	s.WriteString(m.textInput.View())
	s.WriteString("\n\n")

	s.WriteString(infoStyle.Render("Press ENTER to continue • ESC to go back"))

	return s.String()
}

func (m model) viewSetupStep4() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Step 4: Enter kdt (optional)"))
	s.WriteString("\n\n")

	instr := `kdt is another HttpOnly cookie (optional but recommended).

Get it the same way as auth_token:
Application → Cookies → https://x.com → "kdt"

You can leave this empty and press ENTER to skip.`

	s.WriteString(boxStyle.Render(instr))
	s.WriteString("\n\n")

	s.WriteString("Paste kdt (or leave empty):\n")
	s.WriteString(m.textInput.View())
	s.WriteString("\n\n")

	s.WriteString(infoStyle.Render("Press ENTER to continue • ESC to go back"))

	return s.String()
}

func (m model) viewSetupVerify() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Step 5: Verify and Save"))
	s.WriteString("\n\n")

	s.WriteString("Review your data:\n\n")

	data := fmt.Sprintf(
		"User ID: %s\n"+
			"ct0: %s...\n"+
			"guest_id: %s\n"+
			"Query ID (UserTweets): %s...\n"+
			"Query ID (DeleteRetweet): %s...\n"+
			"auth_token: %s...\n"+
			"kdt: %s",
		m.setup.userID,
		truncate(m.setup.ct0, 20),
		m.setup.guestID,
		truncate(m.setup.queryIDUserTweets, 20),
		truncate(m.setup.queryIDDeleteRetweet, 20),
		truncate(m.setup.authToken, 20),
		truncate(m.setup.kdt, 20),
	)

	s.WriteString(boxStyle.Render(data))
	s.WriteString("\n\n")

	s.WriteString(infoStyle.Render("Press ENTER to save • ESC to go back and correct"))

	return s.String()
}

func getProjectPath() string {
	wd, _ := os.Getwd()
	return wd
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
