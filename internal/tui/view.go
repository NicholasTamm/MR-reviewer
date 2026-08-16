package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/provider"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	selStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("63"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

func (m Model) View() tea.View {
	return tea.NewView(m.render())
}

func (m Model) render() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("mr-reviewer"))
	b.WriteString(mutedStyle.Render("  " + m.view.String()))
	b.WriteString("\n\n")
	switch m.view {
	case ViewDashboard:
		b.WriteString(m.viewDashboard())
	case ViewLink:
		b.WriteString(m.viewLink())
	case ViewReviewing:
		b.WriteString("  reviewing…\n")
		if m.status != "" {
			b.WriteString("  " + mutedStyle.Render(m.status) + "\n")
		}
	case ViewHITL:
		b.WriteString(m.viewHITL())
	case ViewConfirm:
		b.WriteString(m.viewConfirm())
	case ViewAuth:
		b.WriteString(m.viewAuth())
	case ViewConfig:
		b.WriteString(m.viewConfig())
	case ViewError:
		b.WriteString(errStyle.Render("  "+m.err) + "\n\n  enter back\n")
	}
	if m.status != "" && m.view != ViewReviewing && m.view != ViewError {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + m.status))
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) viewDashboard() string {
	var b strings.Builder
	search := m.search
	if m.input == inputSearch {
		search = m.search + "█"
	}
	b.WriteString("  / " + search + "\n\n")
	if !m.dashLoaded {
		b.WriteString(mutedStyle.Render("  loading merge requests…") + "\n")
	} else if len(m.flat) == 0 {
		b.WriteString(mutedStyle.Render("  no open merge requests") + "\n")
	} else {
		for i, item := range m.flat {
			if item.header {
				b.WriteString("\n  " + titleStyle.Render(item.path) + "\n")
				continue
			}
			line := fmt.Sprintf("  !%-4d %s", item.mr.IID, item.mr.Title)
			if item.mr.Draft {
				line += "  draft"
			}
			if i == m.cursor {
				b.WriteString(selStyle.Render(line) + "\n")
			} else {
				b.WriteString(line + "\n")
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  j/k move  enter review  / search  l link  c config  a auth  q quit"))
	b.WriteString("\n")
	return b.String()
}

func (m Model) viewLink() string {
	mark := func(f field, label, value string) string {
		line := fmt.Sprintf("  %-10s %s", label, value)
		if m.field == f {
			return selStyle.Render(line) + "\n"
		}
		return line + "\n"
	}
	url := m.url
	if m.input == inputURL {
		url += "█"
	}
	model := m.model
	if m.input == inputModel {
		model += "█"
	}
	var b strings.Builder
	b.WriteString(mark(fieldURL, "url", url))
	b.WriteString(mark(fieldProvider, "provider", m.provider))
	b.WriteString(mark(fieldModel, "model", model))
	b.WriteString(mark(fieldFocus, "focus", strings.Join(m.focus, ", ")))
	b.WriteString(mark(fieldMax, "max", fmt.Sprintf("%d", m.maxC)))
	mode := "review first"
	if m.autoPost {
		mode = "auto-post"
	}
	b.WriteString(mark(fieldAutoPost, "mode", mode))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  tab fields  ←/→ provider  space toggle  +/- max  enter run  esc back"))
	b.WriteString("\n")
	return b.String()
}

func (m Model) viewHITL() string {
	var b strings.Builder
	if m.meta.Title != "" {
		b.WriteString("  " + titleStyle.Render(m.meta.Title) + "\n")
		if m.meta.SourceBranch != "" {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  %s → %s", m.meta.SourceBranch, m.meta.TargetBranch)) + "\n")
		}
		b.WriteString("\n")
	}
	sum := m.summary
	if m.input == inputEditSummary {
		sum = m.editBuf + "█"
	}
	b.WriteString("  summary\n")
	for _, line := range strings.Split(sum, "\n") {
		b.WriteString("  " + line + "\n")
	}
	b.WriteString("\n")
	if len(m.comments) == 0 {
		b.WriteString(okStyle.Render("  no issues found") + "\n")
	}
	for i, c := range m.comments {
		mark := " "
		if c.Approved {
			mark = "+"
		} else {
			mark = "-"
		}
		body := c.Body
		if m.input == inputEditComment && i == m.cursor {
			body = m.editBuf + "█"
		}
		head := fmt.Sprintf("  %s %s:%d  %s", mark, c.File, c.Line, c.Severity)
		if i == m.cursor {
			b.WriteString(selStyle.Render(head) + "\n")
		} else {
			b.WriteString(head + "\n")
		}
		b.WriteString("    " + body + "\n\n")
	}
	b.WriteString(mutedStyle.Render(fmt.Sprintf("  %d comments · %d approved · p post  a/r approve/reject  e edit  s summary",
		len(m.comments), m.ApprovedCount())))
	b.WriteString("\n")
	return b.String()
}

func (m Model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(okStyle.Render("  Review posted") + "\n\n")
	b.WriteString(fmt.Sprintf("  %d comments posted\n", m.postedN))
	if m.meta.WebURL != "" {
		b.WriteString("  " + m.meta.WebURL + "\n")
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  n new  q quit"))
	b.WriteString("\n")
	return b.String()
}

func (m Model) viewAuth() string {
	var b strings.Builder
	for i, name := range m.authList {
		desc := "none"
		if m.store != nil {
			desc = auth.Describe(name, m.store)
		}
		if name == "google" {
			_ = provider.Canonical(name)
		}
		line := fmt.Sprintf("  %-10s %s", name, desc)
		if i == m.authCursor {
			b.WriteString(selStyle.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	if m.input == inputAPIKey {
		b.WriteString("\n  key: " + strings.Repeat("•", len(m.editBuf)) + "█\n")
	}
	if m.input == inputOAuthPaste {
		b.WriteString("\n  paste callback: " + m.editBuf + "█\n")
	}
	if m.pending != nil && m.pending.URL != "" {
		b.WriteString("\n  " + mutedStyle.Render(m.pending.URL) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  enter login  k GitLab PAT  d device (xAI/GitLab)  x logout  esc back"))
	b.WriteString("\n")
	return b.String()
}

func (m Model) viewConfig() string {
	mark := func(f configField, label, value string) string {
		if m.input == inputConfig && m.cfgField == f {
			value += "█"
		}
		line := fmt.Sprintf("  %-12s %s", label, value)
		if m.cfgField == f {
			return selStyle.Render(line) + "\n"
		}
		return line + "\n"
	}
	var b strings.Builder
	b.WriteString(mark(cfgFieldGitHub, "github api", m.cfgGitHub))
	b.WriteString(mark(cfgFieldGitLab, "gitlab", m.cfgGitLab))
	b.WriteString(mark(cfgFieldAnthropic, "anthropic", m.cfgAnthropic))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  tab fields  enter/type edit  s save  esc back"))
	b.WriteString("\n")
	return b.String()
}
