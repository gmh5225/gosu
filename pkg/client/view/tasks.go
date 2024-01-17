package view

import (
	"fmt"
	"sort"
	"time"

	"github.com/can1357/gosu/pkg/session"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

func bytesstr(n float64) string {
	if n < 1024 {
		return fmt.Sprintf("%.2fb", n)
	} else if n /= 1024; n < 1024 {
		return fmt.Sprintf("%.2fkb", n)
	} else if n /= 1024; n < 1024 {
		return fmt.Sprintf("%.2fmb", n)
	}
	return fmt.Sprintf("%.2fgb", n/1024)
}
func timestr(d time.Duration) string {
	if d > time.Second {
		d = d.Round(time.Second)
	}

	if d < time.Minute {
		return d.String()
	} else if d < time.Hour {
		return d.Round(time.Second).String()
	} else if d < 24*time.Hour {
		return d.Round(time.Minute).String()
	} else {
		return d.Round(time.Hour).String()
	}
}

func displayRpcTaskStateRecursive(out *[]table.Row, task session.RpcTaskInfo, prefix string) {
	uid := task.Namespace

	var entry table.Row
	if task.Report.IsZero() {
		entry = table.Row{
			prefix + uid,
			"🧊",
			"",
			"0", //fmt.Sprintf("%v", task.Restarts), // TODO
			task.Status.Icon + " " + task.Status.Code,
			"",
			"",
			"",
		}
	} else {
		process := &task.Report
		entry = table.Row{
			prefix + uid,
			fmt.Sprintf("%v", process.Pid),
			timestr(time.Since(task.Report.CreateTime)),
			"0", //fmt.Sprintf("%v", task.Restarts), // TODO
			task.Status.Icon + " " + task.Status.Code,
			fmt.Sprintf("%.2f%%", process.Cpu),
			fmt.Sprintf("%v", bytesstr(process.Mem)),
			process.Username,
		}
	}

	if len(task.Children) == 0 {
		*out = append(*out, entry)
	} else {
		for idx, child := range task.Children {
			if idx == len(task.Children)-1 {
				displayRpcTaskStateRecursive(out, child, "") //prefix+"└─ ")
			} else {
				displayRpcTaskStateRecursive(out, child, "") //prefix+"├─ ")
			}
		}
	}
}
func torows(jobs []session.RpcJobInfo) (rows []table.Row) {
	for _, job := range jobs {
		if job.Main.Namespace == "" {
			job.Main.Namespace = job.ID
		}
		displayRpcTaskStateRecursive(&rows, job.Main, "")
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i][4] != rows[j][4] {
			return (rows[i][4]) > (rows[j][4])
		} else {
			return rows[i][0] < rows[j][0]
		}
	})
	return
}

type Tasklist struct {
	table      table.Model
	fetch      func() ([]session.RpcJobInfo, error)
	jobs       []session.RpcJobInfo
	fetchError error
	rows       []table.Row
	lastFetch  time.Time
}

type tickMsg time.Time

func (m *Tasklist) Init() tea.Cmd {
	return m.doTick()
}
func (m *Tasklist) doTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
func (m *Tasklist) updateData() {
	if time.Since(m.lastFetch) > 1*time.Second {
		m.lastFetch = time.Now()
		m.jobs, m.fetchError = m.fetch()
	}
	if m.fetchError != nil {
		m.rows = []table.Row{
			{
				lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("9")).
					Render("💀 No connection"),
			},
		}
	} else {
		m.rows = torows(m.jobs)
	}
	m.table.SetRows(m.rows)
	m.table.SetHeight(len(m.rows))
}

func (m *Tasklist) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tickMsg:
		m.updateData()
		return m, m.doTick()
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

var errorBoxStyle = lipgloss.NewStyle().
	Align(lipgloss.Center).
	Bold(true).
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("9")).
	BorderTop(true).
	BorderBottom(true).
	BorderLeft(true).
	BorderRight(true).
	Padding(0, 2).
	Width(96)

func (m *Tasklist) View() string {
	res := baseStyle.Render(m.table.View())
	if m.fetchError != nil {
		res += "\n"
		res += errorBoxStyle.
			Render(m.fetchError.Error())
	} else {
		res += "\n"
	}
	return res
}

func NewTasklist(fetch func() ([]session.RpcJobInfo, error)) *Tasklist {
	columns := []table.Column{
		{Title: "name", Width: 25},
		{Title: "pid", Width: 8},
		{Title: "uptime", Width: 10},
		{Title: "↺", Width: 3},
		{Title: "status", Width: 13},
		{Title: "cpu", Width: 8},
		{Title: "mem", Width: 8},
		{Title: "user", Width: 8},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	t.SetStyles(s)
	list := &Tasklist{table: t, fetch: fetch}
	list.updateData()
	return list
}
