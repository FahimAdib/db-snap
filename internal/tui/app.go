package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"db-snap/internal/config"
	"db-snap/internal/db"
	"db-snap/internal/history"
	"db-snap/internal/model"
	"db-snap/internal/policy"
	"db-snap/internal/snapshot"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tab int

const (
	tabProfiles tab = iota
	tabSnapshots
	tabRules
	tabHistory
	tabSettings
)

type focusArea int

const (
	focusNavbar focusArea = iota
	focusContent
)

type formKind string

const (
	formProfile        formKind = "profile"
	formSnapshotCreate formKind = "snapshot-create"
	formSnapshotTag    formKind = "snapshot-tag"
	formSnapshotExport formKind = "snapshot-export"
	formSnapshotImport formKind = "snapshot-import"
	formRule           formKind = "rule"
	formPolicy         formKind = "policy"
)

type formState struct {
	kind   formKind
	title  string
	labels []string
	inputs []textinput.Model
	focus  int
	meta   map[string]string
}

type appModel struct {
	version string
	svc     snapshot.Service

	width  int
	height int

	tab     tab
	focus   focusArea
	cursor  int
	busy    bool
	status  string
	lastErr error

	tabs []string

	profiles        []model.DBProfile
	selectedProfile string
	snapshots       []model.SnapshotManifest
	rules           model.RulePack
	history         []model.AuditEvent
	policy          model.SafetyPolicy

	restoreOpts  model.RestoreOptions
	restorePlan  *snapshot.RestorePlan
	activeForm   *formState
	spinner      spinner.Model
	spinnerFrame string
}

type loadedMsg struct {
	profiles        []model.DBProfile
	selectedProfile string
	snapshots       []model.SnapshotManifest
	rules           model.RulePack
	history         []model.AuditEvent
	policy          model.SafetyPolicy
	err             error
}

type opMsg struct {
	status          string
	err             error
	refresh         bool
	selectedProfile string
	plan            *snapshot.RestorePlan
}

var (
	bgStyle           = lipgloss.NewStyle().Background(lipgloss.Color("#282828")).Foreground(lipgloss.Color("#ebdbb2"))
	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ebdbb2")).Background(lipgloss.Color("#3c3836")).Padding(0, 1)
	sidebarBaseStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1).Width(24).Background(lipgloss.Color("#282828"))
	sidebarFocusStyle = sidebarBaseStyle.Copy().BorderForeground(lipgloss.Color("#fabd2f"))
	sidebarBlurStyle  = sidebarBaseStyle.Copy().BorderForeground(lipgloss.Color("#665c54"))
	panelBaseStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1).Background(lipgloss.Color("#282828"))
	panelFocusStyle   = panelBaseStyle.Copy().BorderForeground(lipgloss.Color("#fabd2f"))
	panelBlurStyle    = panelBaseStyle.Copy().BorderForeground(lipgloss.Color("#665c54"))
	footerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#a89984")).Padding(0, 1)
	tabActiveStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#282828")).Background(lipgloss.Color("#83a598")).Bold(true).Padding(0, 1)
	tabIdleStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#d5c4a1")).Padding(0, 1)
	rowActiveStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2")).Background(lipgloss.Color("#504945")).Padding(0, 1)
	rowIdleStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#d5c4a1")).Padding(0, 1)
	sectionStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#fabd2f"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934")).Bold(true)
	statusBusyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8ec07c"))
	modalStyle        = lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(lipgloss.Color("#d3869b")).Background(lipgloss.Color("#3c3836")).Padding(1).Width(90)
	focusedLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8ec07c")).Bold(true)
)

func Run(version string, svc snapshot.Service) error {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))

	m := appModel{
		version: version,
		svc:     svc,
		tab:     tabProfiles,
		focus:   focusNavbar,
		tabs:    []string{"Profiles", "Snapshots", "Rules", "History", "Settings"},
		status:  "loading...",
		restoreOpts: model.RestoreOptions{
			AutoFill:       true,
			DryRun:         false,
			ForceUnsafe:    false,
			TruncateBefore: true,
		},
		spinner: sp,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m appModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.refreshCmd(""))
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		m.spinnerFrame = m.spinner.View()
		return m, cmd
	case loadedMsg:
		m.busy = false
		if msg.err != nil {
			m.lastErr = msg.err
			m.status = "failed to load state"
			return m, nil
		}
		m.lastErr = nil
		m.profiles = msg.profiles
		m.selectedProfile = msg.selectedProfile
		m.snapshots = msg.snapshots
		m.rules = msg.rules
		m.history = msg.history
		m.policy = msg.policy
		m.status = "ready"
		m.clampCursor()
		return m, nil
	case opMsg:
		m.busy = false
		if msg.status != "" {
			m.status = msg.status
		}
		if msg.err != nil {
			m.lastErr = msg.err
			if msg.status == "" {
				m.status = "operation failed"
			}
			return m, nil
		}
		m.lastErr = nil
		if msg.plan != nil {
			m.restorePlan = msg.plan
		}
		if msg.selectedProfile != "" {
			m.selectedProfile = msg.selectedProfile
		}
		if msg.refresh {
			return m, m.refreshCmd(m.selectedProfile)
		}
		return m, nil
	case tea.KeyMsg:
		if m.activeForm != nil {
			return m.updateForm(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "1", "2", "3", "4", "5":
			idx, _ := strconv.Atoi(msg.String())
			idx--
			if idx >= 0 && idx < len(m.tabs) {
				m.tab = tab(idx)
				m.cursor = 0
				m.restorePlan = nil
				m.clampCursor()
			}
			return m, nil
		case "tab", "shift+tab":
			if m.focus == focusNavbar {
				m.focus = focusContent
			} else {
				m.focus = focusNavbar
			}
			return m, nil
		case "up", "k":
			if m.focus == focusNavbar {
				if int(m.tab) > 0 {
					m.tab = tab(int(m.tab) - 1)
					m.cursor = 0
					m.restorePlan = nil
					m.clampCursor()
				}
				return m, nil
			}
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.focus == focusNavbar {
				if int(m.tab) < len(m.tabs)-1 {
					m.tab = tab(int(m.tab) + 1)
					m.cursor = 0
					m.restorePlan = nil
					m.clampCursor()
				}
				return m, nil
			}
			if m.cursor < m.currentListMax() {
				m.cursor++
			}
			return m, nil
		case "enter":
			if m.focus == focusNavbar {
				m.focus = focusContent
				m.clampCursor()
				return m, nil
			}
			return m.handleTabKey(msg)
		case "R", "r":
			m.busy = true
			m.status = "refreshing..."
			return m, m.refreshCmd(m.selectedProfile)
		}
		if m.focus == focusNavbar {
			return m, nil
		}
		return m.handleTabKey(msg)
	}
	return m, nil
}

func (m appModel) handleTabKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.tab {
	case tabProfiles:
		return m.handleProfilesKey(msg)
	case tabSnapshots:
		return m.handleSnapshotsKey(msg)
	case tabRules:
		return m.handleRulesKey(msg)
	case tabSettings:
		return m.handleSettingsKey(msg)
	default:
		return m, nil
	}
}

func (m appModel) handleProfilesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "n", "a":
		m.activeForm = m.newProfileForm(nil)
		return m, nil
	case "e":
		if p := m.currentProfile(); p != nil {
			m.activeForm = m.newProfileForm(p)
		}
		return m, nil
	case "d", "x":
		p := m.currentProfile()
		if p == nil {
			return m, nil
		}
		m.busy = true
		return m, m.runOp(func() opMsg {
			if err := config.DeleteProfile(p.Name); err != nil {
				return opMsg{status: "profile delete failed", err: err}
			}
			_ = config.DeletePassword(p.Name)
			nextSel := ""
			if m.selectedProfile != p.Name {
				nextSel = m.selectedProfile
			}
			return opMsg{status: "profile deleted", refresh: true, selectedProfile: nextSel}
		})
	case "s", "enter":
		p := m.currentProfile()
		if p == nil {
			return m, nil
		}
		m.selectedProfile = p.Name
		m.busy = true
		m.status = "profile selected"
		return m, m.refreshCmd(p.Name)
	case "t":
		p := m.currentProfile()
		if p == nil {
			return m, nil
		}
		m.busy = true
		return m, m.runOp(func() opMsg {
			pw, _ := config.LoadPassword(p.Name)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			if err := db.Ping(ctx, *p, pw); err != nil {
				return opMsg{status: "profile test failed", err: err}
			}
			return opMsg{status: "profile connection ok"}
		})
	}
	return m, nil
}

func (m appModel) handleSnapshotsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.selectedProfile == "" {
		return m, nil
	}
	switch msg.String() {
	case "c", "a":
		m.activeForm = m.newSnapshotCreateForm()
		return m, nil
	case "d", "x":
		s := m.currentSnapshot()
		if s == nil {
			return m, nil
		}
		m.busy = true
		return m, m.runOp(func() opMsg {
			if err := config.DeleteSnapshot(m.selectedProfile, s.ID); err != nil {
				return opMsg{status: "snapshot delete failed", err: err}
			}
			return opMsg{status: "snapshot deleted", refresh: true}
		})
	case "g", "e":
		s := m.currentSnapshot()
		if s != nil {
			m.activeForm = m.newSnapshotTagForm(*s)
		}
		return m, nil
	case "o":
		s := m.currentSnapshot()
		if s != nil {
			m.activeForm = m.newSnapshotExportForm(*s)
		}
		return m, nil
	case "i":
		m.activeForm = m.newSnapshotImportForm()
		return m, nil
	case "u":
		m.restoreOpts.AutoFill = !m.restoreOpts.AutoFill
		return m, nil
	case "y":
		m.restoreOpts.DryRun = !m.restoreOpts.DryRun
		return m, nil
	case "f":
		m.restoreOpts.ForceUnsafe = !m.restoreOpts.ForceUnsafe
		return m, nil
	case "t":
		m.restoreOpts.TruncateBefore = !m.restoreOpts.TruncateBefore
		return m, nil
	case "p", " ":
		s := m.currentSnapshot()
		if s == nil {
			return m, nil
		}
		m.busy = true
		return m, m.runOp(func() opMsg {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
			defer cancel()
			plan, err := m.svc.PlanRestore(ctx, m.selectedProfile, s.ID)
			if err != nil {
				return opMsg{status: "restore plan failed", err: err}
			}
			return opMsg{status: "restore plan ready", plan: &plan}
		})
	case "enter":
		s := m.currentSnapshot()
		if s == nil {
			return m, nil
		}
		m.busy = true
		opts := m.restoreOpts
		opts.Profile = m.selectedProfile
		opts.SnapshotID = s.ID
		return m, m.runOp(func() opMsg {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
			defer cancel()
			if err := m.svc.Restore(ctx, opts, nil); err != nil {
				return opMsg{status: "restore failed", err: err}
			}
			return opMsg{status: "restore complete", refresh: true}
		})
	}
	return m, nil
}

func (m appModel) handleRulesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.selectedProfile == "" {
		return m, nil
	}
	switch msg.String() {
	case "n", "a":
		m.activeForm = m.newRuleForm(nil)
		return m, nil
	case "e", "enter":
		r := m.currentRule()
		if r != nil {
			m.activeForm = m.newRuleForm(r)
		}
		return m, nil
	case "d", "x":
		r := m.currentRule()
		if r == nil {
			return m, nil
		}
		m.busy = true
		return m, m.runOp(func() opMsg {
			rp, err := config.LoadRulePack(m.selectedProfile)
			if err != nil {
				return opMsg{status: "rules load failed", err: err}
			}
			next := make([]model.ColumnRule, 0, len(rp.Rules))
			for _, existing := range rp.Rules {
				if existing.Schema == r.Schema && existing.Table == r.Table && existing.Column == r.Column {
					continue
				}
				next = append(next, existing)
			}
			rp.Rules = next
			if err := config.SaveRulePack(rp); err != nil {
				return opMsg{status: "rule delete failed", err: err}
			}
			return opMsg{status: "rule deleted", refresh: true}
		})
	}
	return m, nil
}

func (m appModel) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "e", "enter":
		m.activeForm = m.newPolicyForm()
		return m, nil
	case "w":
		m.busy = true
		return m, m.runOp(func() opMsg {
			cfg, err := config.LoadConfig()
			if err != nil {
				return opMsg{status: "policy load failed", err: err}
			}
			cfg.Policy.WarnEveryRestore = !cfg.Policy.WarnEveryRestore
			if err := config.SaveConfig(cfg); err != nil {
				return opMsg{status: "policy update failed", err: err}
			}
			return opMsg{status: "warn-every-restore toggled", refresh: true}
		})
	}
	return m, nil
}

func (m appModel) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.activeForm
	if f == nil {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.activeForm = nil
		m.status = "form cancelled"
		return m, nil
	case "tab", "down":
		f.focus = (f.focus + 1) % len(f.inputs)
		m.activeForm = setFormFocus(f)
		return m, nil
	case "shift+tab", "up":
		f.focus = (f.focus - 1 + len(f.inputs)) % len(f.inputs)
		m.activeForm = setFormFocus(f)
		return m, nil
	case "ctrl+s":
		m.activeForm = nil
		m.busy = true
		m.lastErr = nil
		m.status = "submitting form..."
		return m, m.submitFormCmd(*f)
	case "enter":
		if f.focus == len(f.inputs)-1 {
			m.activeForm = nil
			m.busy = true
			m.lastErr = nil
			m.status = "submitting form..."
			return m, m.submitFormCmd(*f)
		}
		f.focus++
		m.activeForm = setFormFocus(f)
		return m, nil
	}

	input := f.inputs[f.focus]
	var cmd tea.Cmd
	input, cmd = input.Update(msg)
	f.inputs[f.focus] = input
	m.activeForm = f
	return m, cmd
}

func (m appModel) submitFormCmd(f formState) tea.Cmd {
	values := make([]string, len(f.inputs))
	for i := range f.inputs {
		values[i] = strings.TrimSpace(f.inputs[i].Value())
	}

	switch f.kind {
	case formProfile:
		return m.runOp(func() opMsg {
			port, err := strconv.Atoi(values[2])
			if err != nil {
				return opMsg{status: "invalid profile port", err: err}
			}
			p := model.DBProfile{
				Name:     values[0],
				Host:     values[1],
				Port:     port,
				Database: values[3],
				User:     values[4],
				SSLMode:  values[5],
				Tags:     splitCSV(values[6]),
			}
			if p.Name == "" || p.Database == "" || p.User == "" {
				return opMsg{status: "profile requires name, database, user", err: fmt.Errorf("missing required fields")}
			}
			cfg, err := config.LoadConfig()
			if err != nil {
				return opMsg{status: "config load failed", err: err}
			}
			if _, err := policy.ValidateTarget(p.Host, cfg.Policy); err != nil {
				return opMsg{status: "profile blocked by safety policy", err: err}
			}
			now := time.Now().UTC().Format(time.RFC3339)
			if existing, err := config.LoadProfile(f.meta["old_name"]); err == nil {
				p.CreatedAt = existing.CreatedAt
			} else {
				p.CreatedAt = now
			}
			p.UpdatedAt = now
			if old := f.meta["old_name"]; old != "" && old != p.Name {
				_ = config.DeleteProfile(old)
				_ = config.DeletePassword(old)
			}
			if err := config.SaveProfile(p); err != nil {
				return opMsg{status: "profile save failed", err: err}
			}
			if pw := values[7]; pw != "" {
				if err := config.SavePassword(p.Name, pw); err != nil {
					return opMsg{status: "password save failed", err: err}
				}
			}
			return opMsg{status: "profile saved", refresh: true, selectedProfile: p.Name}
		})
	case formSnapshotCreate:
		return m.runOp(func() opMsg {
			if m.selectedProfile == "" {
				return opMsg{status: "select profile first", err: fmt.Errorf("no profile selected")}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
			defer cancel()
			deterministic := parseBool(values[3], false)
			item, err := m.svc.Create(ctx, snapshot.CreateOptions{
				Profile:       m.selectedProfile,
				Tags:          splitCSV(values[0]),
				IncludeTables: splitCSV(values[1]),
				ExcludeTables: splitCSV(values[2]),
				Deterministic: deterministic,
			})
			if err != nil {
				return opMsg{status: "snapshot create failed", err: err}
			}
			return opMsg{status: "snapshot created: " + item.ID, refresh: true}
		})
	case formSnapshotTag:
		return m.runOp(func() opMsg {
			snapshotID := f.meta["snapshot_id"]
			mft, err := config.LoadManifest(m.selectedProfile, snapshotID)
			if err != nil {
				return opMsg{status: "snapshot load failed", err: err}
			}
			mft.Tags = splitCSV(values[0])
			if err := config.SaveManifest(m.selectedProfile, mft); err != nil {
				return opMsg{status: "snapshot tags update failed", err: err}
			}
			return opMsg{status: "snapshot tags updated", refresh: true}
		})
	case formSnapshotExport:
		return m.runOp(func() opMsg {
			snapshotID := f.meta["snapshot_id"]
			out := values[0]
			if err := m.svc.Export(m.selectedProfile, snapshotID, out); err != nil {
				return opMsg{status: "snapshot export failed", err: err}
			}
			return opMsg{status: "snapshot exported", refresh: true}
		})
	case formSnapshotImport:
		return m.runOp(func() opMsg {
			if values[0] == "" {
				return opMsg{status: "import file path required", err: fmt.Errorf("missing path")}
			}
			id, err := m.svc.Import(m.selectedProfile, values[0])
			if err != nil {
				return opMsg{status: "snapshot import failed", err: err}
			}
			return opMsg{status: "snapshot imported: " + id, refresh: true}
		})
	case formRule:
		return m.runOp(func() opMsg {
			rp, err := config.LoadRulePack(m.selectedProfile)
			if err != nil {
				return opMsg{status: "rules load failed", err: err}
			}
			rule := model.ColumnRule{Schema: values[0], Table: values[1], Column: values[2], Type: values[3], Value: values[4], Persistent: true}
			if rule.Schema == "" {
				rule.Schema = "public"
			}
			if rule.Table == "" || rule.Column == "" || rule.Value == "" {
				return opMsg{status: "rule requires table, column, value", err: fmt.Errorf("missing required fields")}
			}
			next := make([]model.ColumnRule, 0, len(rp.Rules)+1)
			replaced := false
			for _, existing := range rp.Rules {
				if existing.Schema == rule.Schema && existing.Table == rule.Table && existing.Column == rule.Column {
					next = append(next, rule)
					replaced = true
					continue
				}
				next = append(next, existing)
			}
			if !replaced {
				next = append(next, rule)
			}
			rp.Rules = next
			if err := config.SaveRulePack(rp); err != nil {
				return opMsg{status: "rule save failed", err: err}
			}
			return opMsg{status: "rule saved", refresh: true}
		})
	case formPolicy:
		return m.runOp(func() opMsg {
			cfg, err := config.LoadConfig()
			if err != nil {
				return opMsg{status: "policy load failed", err: err}
			}
			cfg.Policy.AllowCIDRs = splitCSV(values[0])
			cfg.Policy.DenyHostKeywords = splitCSV(values[1])
			cfg.Policy.WarnEveryRestore = parseBool(values[2], true)
			if err := config.SaveConfig(cfg); err != nil {
				return opMsg{status: "policy save failed", err: err}
			}
			return opMsg{status: "policy saved", refresh: true}
		})
	default:
		return nil
	}
}

func (m appModel) refreshCmd(preferred string) tea.Cmd {
	return func() tea.Msg {
		profiles, err := config.ListProfiles()
		if err != nil {
			return loadedMsg{err: err}
		}
		sel := pickSelectedProfile(profiles, preferred)
		if sel == "" {
			sel = pickSelectedProfile(profiles, m.selectedProfile)
		}

		snaps := []model.SnapshotManifest{}
		rules := model.RulePack{Profile: sel, Rules: []model.ColumnRule{}}
		if sel != "" {
			snaps, err = config.ListSnapshots(sel)
			if err != nil {
				return loadedMsg{err: err}
			}
			rules, err = config.LoadRulePack(sel)
			if err != nil {
				return loadedMsg{err: err}
			}
		}

		h, err := history.List()
		if err != nil {
			return loadedMsg{err: err}
		}
		cfg, err := config.LoadConfig()
		if err != nil {
			return loadedMsg{err: err}
		}
		return loadedMsg{
			profiles:        profiles,
			selectedProfile: sel,
			snapshots:       snaps,
			rules:           rules,
			history:         h,
			policy:          cfg.Policy,
		}
	}
}

func (m appModel) runOp(fn func() opMsg) tea.Cmd {
	return func() tea.Msg {
		return fn()
	}
}

func (m appModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading db-snap..."
	}

	status := m.status
	if m.busy {
		status = statusBusyStyle.Render(m.spinnerFrame + " " + status)
	}
	if m.lastErr != nil {
		status = errorStyle.Render("error: " + m.lastErr.Error())
	}

	header := headerStyle.Width(m.width).Render(fmt.Sprintf(" db-snap %s  •  profile: %s  •  %s ", m.version, fallback(m.selectedProfile, "<none>"), status))
	bodyWidth := m.width - 26
	if bodyWidth < 40 {
		bodyWidth = 40
	}
	sideStyle := sidebarBlurStyle
	panelStyle := panelBlurStyle
	if m.focus == focusNavbar {
		sideStyle = sidebarFocusStyle
	} else {
		panelStyle = panelFocusStyle
	}
	side := sideStyle.Height(max(10, m.height-6)).Render(m.renderTabs())
	panel := panelStyle.Width(bodyWidth - 3).Render(m.renderContent())
	body := lipgloss.JoinHorizontal(lipgloss.Top, side, panel)
	footer := footerStyle.Width(m.width).Render("Tab: switch focus (navbar/content) • ↑/↓: vertical nav • Enter: primary • 1-5: jump section • r: refresh • q: quit")

	out := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	if m.activeForm != nil {
		out = lipgloss.JoinVertical(lipgloss.Left, out, "", m.renderForm())
	}
	return bgStyle.Render(out)
}

func (m appModel) renderTabs() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("Sections") + "\n\n")
	for i, t := range m.tabs {
		if int(m.tab) == i {
			b.WriteString(tabActiveStyle.Render("▶ " + t))
		} else {
			b.WriteString(tabIdleStyle.Render("  " + t))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m appModel) renderContent() string {
	switch m.tab {
	case tabProfiles:
		return m.renderProfiles()
	case tabSnapshots:
		return m.renderSnapshots()
	case tabRules:
		return m.renderRules()
	case tabHistory:
		return m.renderHistory()
	case tabSettings:
		return m.renderSettings()
	default:
		return ""
	}
}

func (m appModel) renderProfiles() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("Profiles") + "\n")
	if len(m.profiles) == 0 {
		b.WriteString("\nNo profiles yet.\n")
		b.WriteString("Press a to create one.\n")
	} else {
		for i, p := range m.profiles {
			line := fmt.Sprintf("%s  %s:%d/%s  ssl=%s", p.Name, p.Host, p.Port, p.Database, p.SSLMode)
			if p.Name == m.selectedProfile {
				line += "  [selected]"
			}
			if i == m.cursor {
				b.WriteString(rowActiveStyle.Render(line) + "\n")
			} else {
				b.WriteString(rowIdleStyle.Render(line) + "\n")
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Actions") + "\n")
	b.WriteString("a add • e edit • x delete • enter select • t test connection\n")
	return b.String()
}

func (m appModel) renderSnapshots() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("Snapshots + Restore") + "\n")
	if m.selectedProfile == "" {
		b.WriteString("\nSelect a profile first in Profiles.\n")
		return b.String()
	}
	if len(m.snapshots) == 0 {
		b.WriteString("\nNo snapshots for selected profile.\n")
	} else {
		for i, s := range m.snapshots {
			line := fmt.Sprintf("%s  %s  tags=[%s]", s.ID, s.CreatedAt.Format(time.RFC3339), strings.Join(s.Tags, ","))
			if i == m.cursor {
				b.WriteString(rowActiveStyle.Render(line) + "\n")
			} else {
				b.WriteString(rowIdleStyle.Render(line) + "\n")
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Restore options") + "\n")
	b.WriteString(fmt.Sprintf("Auto-fill (u): %v\n", m.restoreOpts.AutoFill))
	b.WriteString(fmt.Sprintf("Dry-run (y): %v\n", m.restoreOpts.DryRun))
	b.WriteString(fmt.Sprintf("Force unsafe/private (f): %v\n", m.restoreOpts.ForceUnsafe))
	b.WriteString(fmt.Sprintf("Truncate before restore (t): %v\n", m.restoreOpts.TruncateBefore))
	b.WriteString("\nSnapshot actions: a create • e edit tags • x delete • o export • i import\n")
	b.WriteString("Restore actions: u toggle autofill • y dry-run • f force • t truncate • space plan • enter restore\n")
	if m.restorePlan != nil {
		b.WriteString("\n")
		b.WriteString(sectionStyle.Render("Latest plan") + "\n")
		b.WriteString(fmt.Sprintf("compatible: %v, warnings: %d, unresolved required fields: %d\n", m.restorePlan.Compatible, len(m.restorePlan.Warnings), len(m.restorePlan.RequiresPrompt)))
		for _, w := range m.restorePlan.Warnings {
			b.WriteString("- " + w + "\n")
		}
	}
	return b.String()
}

func (m appModel) renderRules() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("Rules & Mappings") + "\n")
	if m.selectedProfile == "" {
		b.WriteString("\nSelect a profile first in Profiles.\n")
		return b.String()
	}
	if len(m.rules.Rules) == 0 {
		b.WriteString("\nNo rules configured.\n")
	} else {
		for i, r := range m.rules.Rules {
			line := fmt.Sprintf("%s.%s.%s  type=%s  value=%s", r.Schema, r.Table, r.Column, r.Type, r.Value)
			if i == m.cursor {
				b.WriteString(rowActiveStyle.Render(line) + "\n")
			} else {
				b.WriteString(rowIdleStyle.Render(line) + "\n")
			}
		}
	}
	b.WriteString("\nActions: a add • e edit • x delete\n")
	return b.String()
}

func (m appModel) renderHistory() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("History / Audit") + "\n")
	if len(m.history) == 0 {
		b.WriteString("\nNo audit history yet.\n")
		return b.String()
	}
	maxItems := m.historyVisibleCount()
	for i := 0; i < maxItems; i++ {
		e := m.history[len(m.history)-1-i]
		line := fmt.Sprintf("%s  %s  %s  %s  %s", e.CreatedAt.Format(time.RFC3339), e.Type, e.Profile, e.Snapshot, e.Status)
		if i == m.cursor {
			b.WriteString(rowActiveStyle.Render(line) + "\n")
		} else {
			b.WriteString(rowIdleStyle.Render(line) + "\n")
		}
	}
	return b.String()
}

func (m appModel) renderSettings() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("Settings / Safety Policy") + "\n\n")
	b.WriteString(fmt.Sprintf("Allow CIDRs: %s\n", strings.Join(m.policy.AllowCIDRs, ",")))
	b.WriteString(fmt.Sprintf("Deny host keywords: %s\n", strings.Join(m.policy.DenyHostKeywords, ",")))
	b.WriteString(fmt.Sprintf("Warn every restore: %v\n", m.policy.WarnEveryRestore))
	b.WriteString("\nActions: enter edit policy • w toggle warn-every-restore\n")
	return b.String()
}

func (m appModel) renderForm() string {
	f := m.activeForm
	if f == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(sectionStyle.Render(f.title) + "\n\n")
	for i := range f.inputs {
		label := f.labels[i]
		input := f.inputs[i].View()
		if i == f.focus {
			label = focusedLabelStyle.Render(label)
		}
		b.WriteString(fmt.Sprintf("%s\n%s\n\n", label, input))
	}
	b.WriteString("Enter next/submit • Tab/Shift+Tab navigate • Ctrl+S submit • Esc cancel")
	return modalStyle.Render(b.String())
}

func (m appModel) newProfileForm(existing *model.DBProfile) *formState {
	labels := []string{"Name", "Host", "Port", "Database", "User", "SSL Mode", "Tags (csv)", "Password (optional; leave empty to keep)"}
	defaults := []string{"", "localhost", "5432", "", "", "disable", "", ""}
	meta := map[string]string{}
	if existing != nil {
		defaults = []string{
			existing.Name,
			existing.Host,
			strconv.Itoa(existing.Port),
			existing.Database,
			existing.User,
			existing.SSLMode,
			strings.Join(existing.Tags, ","),
			"",
		}
		meta["old_name"] = existing.Name
	}
	inputs := make([]textinput.Model, len(labels))
	for i := range labels {
		ti := textinput.New()
		ti.SetValue(defaults[i])
		ti.Prompt = "› "
		ti.Width = 76
		if i == 7 {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		inputs[i] = ti
	}
	f := &formState{kind: formProfile, title: "Profile Editor", labels: labels, inputs: inputs, focus: 0, meta: meta}
	return setFormFocus(f)
}

func (m appModel) newSnapshotCreateForm() *formState {
	labels := []string{"Tags (csv)", "Include tables (csv, optional)", "Exclude tables (csv, optional)", "Deterministic (true/false)"}
	defaults := []string{"", "", "", "false"}
	inputs := make([]textinput.Model, len(labels))
	for i := range labels {
		ti := textinput.New()
		ti.Prompt = "› "
		ti.Width = 76
		ti.SetValue(defaults[i])
		inputs[i] = ti
	}
	f := &formState{kind: formSnapshotCreate, title: "Create Snapshot", labels: labels, inputs: inputs, focus: 0, meta: map[string]string{}}
	return setFormFocus(f)
}

func (m appModel) newSnapshotTagForm(s model.SnapshotManifest) *formState {
	labels := []string{"Tags (csv)"}
	defaults := []string{strings.Join(s.Tags, ",")}
	inputs := make([]textinput.Model, len(labels))
	for i := range labels {
		ti := textinput.New()
		ti.Prompt = "› "
		ti.Width = 76
		ti.SetValue(defaults[i])
		inputs[i] = ti
	}
	f := &formState{kind: formSnapshotTag, title: "Edit Snapshot Tags", labels: labels, inputs: inputs, focus: 0, meta: map[string]string{"snapshot_id": s.ID}}
	return setFormFocus(f)
}

func (m appModel) newSnapshotExportForm(s model.SnapshotManifest) *formState {
	labels := []string{"Export file path (.tar.gz)"}
	defaults := []string{fmt.Sprintf("%s-%s.tar.gz", m.selectedProfile, s.ID)}
	inputs := make([]textinput.Model, len(labels))
	for i := range labels {
		ti := textinput.New()
		ti.Prompt = "› "
		ti.Width = 76
		ti.SetValue(defaults[i])
		inputs[i] = ti
	}
	f := &formState{kind: formSnapshotExport, title: "Export Snapshot", labels: labels, inputs: inputs, focus: 0, meta: map[string]string{"snapshot_id": s.ID}}
	return setFormFocus(f)
}

func (m appModel) newSnapshotImportForm() *formState {
	labels := []string{"Import file path (.tar.gz)"}
	defaults := []string{""}
	inputs := make([]textinput.Model, len(labels))
	for i := range labels {
		ti := textinput.New()
		ti.Prompt = "› "
		ti.Width = 76
		ti.SetValue(defaults[i])
		inputs[i] = ti
	}
	f := &formState{kind: formSnapshotImport, title: "Import Snapshot", labels: labels, inputs: inputs, focus: 0, meta: map[string]string{}}
	return setFormFocus(f)
}

func (m appModel) newRuleForm(existing *model.ColumnRule) *formState {
	labels := []string{"Schema", "Table", "Column", "Type", "Value (SQL literal/expression)"}
	defaults := []string{"public", "", "", "static", ""}
	if existing != nil {
		defaults = []string{existing.Schema, existing.Table, existing.Column, existing.Type, existing.Value}
	}
	inputs := make([]textinput.Model, len(labels))
	for i := range labels {
		ti := textinput.New()
		ti.Prompt = "› "
		ti.Width = 76
		ti.SetValue(defaults[i])
		inputs[i] = ti
	}
	f := &formState{kind: formRule, title: "Rule Editor", labels: labels, inputs: inputs, focus: 0, meta: map[string]string{}}
	return setFormFocus(f)
}

func (m appModel) newPolicyForm() *formState {
	labels := []string{"Allow CIDRs (csv)", "Deny host keywords (csv)", "Warn every restore (true/false)"}
	defaults := []string{strings.Join(m.policy.AllowCIDRs, ","), strings.Join(m.policy.DenyHostKeywords, ","), strconv.FormatBool(m.policy.WarnEveryRestore)}
	inputs := make([]textinput.Model, len(labels))
	for i := range labels {
		ti := textinput.New()
		ti.Prompt = "› "
		ti.Width = 76
		ti.SetValue(defaults[i])
		inputs[i] = ti
	}
	f := &formState{kind: formPolicy, title: "Safety Policy Editor", labels: labels, inputs: inputs, focus: 0, meta: map[string]string{}}
	return setFormFocus(f)
}

func setFormFocus(f *formState) *formState {
	for i := range f.inputs {
		if i == f.focus {
			f.inputs[i].Focus()
			continue
		}
		f.inputs[i].Blur()
	}
	return f
}

func (m *appModel) clampCursor() {
	maxIdx := m.currentListMax()
	if m.cursor > maxIdx {
		m.cursor = max(0, maxIdx)
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m appModel) currentListMax() int {
	switch m.tab {
	case tabProfiles:
		return max(0, len(m.profiles)-1)
	case tabSnapshots:
		return max(0, len(m.snapshots)-1)
	case tabRules:
		return max(0, len(m.rules.Rules)-1)
	case tabHistory:
		return max(0, m.historyVisibleCount()-1)
	default:
		return 0
	}
}

func (m appModel) historyVisibleCount() int {
	return min(len(m.history), max(8, m.height-12))
}

func (m appModel) currentProfile() *model.DBProfile {
	if len(m.profiles) == 0 || m.cursor < 0 || m.cursor >= len(m.profiles) {
		return nil
	}
	p := m.profiles[m.cursor]
	return &p
}

func (m appModel) currentSnapshot() *model.SnapshotManifest {
	if len(m.snapshots) == 0 || m.cursor < 0 || m.cursor >= len(m.snapshots) {
		return nil
	}
	s := m.snapshots[m.cursor]
	return &s
}

func (m appModel) currentRule() *model.ColumnRule {
	if len(m.rules.Rules) == 0 || m.cursor < 0 || m.cursor >= len(m.rules.Rules) {
		return nil
	}
	r := m.rules.Rules[m.cursor]
	return &r
}

func pickSelectedProfile(profiles []model.DBProfile, preferred string) string {
	if preferred != "" {
		for _, p := range profiles {
			if p.Name == preferred {
				return preferred
			}
		}
	}
	if len(profiles) > 0 {
		return profiles[0].Name
	}
	return ""
}

func splitCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return []string{}
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func parseBool(v string, fallbackValue bool) bool {
	if strings.TrimSpace(v) == "" {
		return fallbackValue
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return fallbackValue
	}
	return b
}

func fallback(v, f string) string {
	if strings.TrimSpace(v) == "" {
		return f
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
