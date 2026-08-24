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
	case ViewProjects:
		b.WriteString(m.viewProjects())
	case ViewReviews:
		b.WriteString(m.viewReviews())
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
	case ViewOnboarding:
		b.WriteString(m.viewOnboarding())
	case ViewError:
		b.WriteString(errStyle.Render("  "+m.err) + "\n\n  enter back\n")
	}
	if m.status != "" && m.view != ViewReviewing && m.view != ViewError && m.view != ViewProjects && m.view != ViewReviews && !(m.view == ViewAuth && (m.authCancel != nil || m.authFailed)) {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + m.status))
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) viewDashboard() string {
	var b strings.Builder
	b.WriteString("  Browse " + titleStyle.Render(strings.ToUpper(m.platform)) + "\n\n")
	b.WriteString(mutedStyle.Render("  tab switch platform  enter projects  l link  c config  a auth  q quit"))
	b.WriteString("\n")
	return b.String()
}

func (m Model) viewProjects() string {
	var b strings.Builder
	b.WriteString("  " + titleStyle.Render(strings.ToUpper(m.platform)) + " projects\n\n")
	if !m.catalogLoaded {
		b.WriteString(mutedStyle.Render("  loading projects…") + "\n")
	} else if m.status != "" {
		b.WriteString(errStyle.Render("  "+m.status) + "\n")
	} else if len(m.projects) == 0 {
		b.WriteString(mutedStyle.Render("  no accessible projects") + "\n")
	} else {
		for i, project := range m.projects {
			line := "  " + project.Path
			if i == m.cursor {
				b.WriteString(selStyle.Render(line) + "\n")
			} else {
				b.WriteString(line + "\n")
			}
		}
	}
	b.WriteString("\n" + mutedStyle.Render("  j/k move  enter select  r retry  esc platform") + "\n")
	return b.String()
}

func (m Model) viewReviews() string {
	var b strings.Builder
	kind := "merge requests"
	if m.platform == "github" {
		kind = "pull requests"
	}
	b.WriteString("  " + titleStyle.Render(m.project.Path) + " " + kind + "\n\n")
	if !m.catalogLoaded {
		b.WriteString(mutedStyle.Render("  loading "+kind+"…") + "\n")
	} else if m.status != "" {
		b.WriteString(errStyle.Render("  "+m.status) + "\n")
	} else if len(m.reviews) == 0 {
		b.WriteString(mutedStyle.Render("  no open "+kind) + "\n")
	} else {
		for i, review := range m.reviews {
			line := fmt.Sprintf("  #%-4d %s", review.Number, review.Title)
			if review.Draft {
				line += "  draft"
			}
			if i == m.cursor {
				b.WriteString(selStyle.Render(line) + "\n")
			} else {
				b.WriteString(line + "\n")
			}
		}
	}
	b.WriteString("\n" + mutedStyle.Render("  j/k move  enter configure review  r retry  esc projects") + "\n")
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
	if m.urlLocked {
		title := m.reviewTitle
		if title == "" {
			title = url
		}
		b.WriteString("  " + titleStyle.Render(title) + "\n")
	} else {
		b.WriteString(mark(fieldURL, "url", url))
	}
	if !m.providerConfigured() {
		line := fmt.Sprintf("  %-10s %s", "provider", m.provider)
		if m.field == fieldProvider {
			b.WriteString(mutedStyle.Render(selStyle.Render(line)) + "\n")
		} else {
			b.WriteString(mutedStyle.Render(line) + "\n")
		}
		b.WriteString(mark(fieldModel, "model", "-"))
	} else {
		b.WriteString(mark(fieldProvider, "provider", m.provider))
		b.WriteString(mark(fieldModel, "model", model))
	}
	b.WriteString(mark(fieldFocus, "focus", strings.Join(m.focus, ", ")))
	b.WriteString(mark(fieldMax, "max", fmt.Sprintf("%d", m.maxC)))
	mode := "review first"
	if m.autoPost {
		mode = "auto-post"
	}
	b.WriteString(mark(fieldAutoPost, "mode", mode))
	b.WriteString("\n")
	if !m.providerConfigured() {
		b.WriteString(mutedStyle.Render("  unconfigured  a add credentials for " + m.provider + "  esc back"))
	} else {
		b.WriteString(mutedStyle.Render("  tab/↑↓ fields  ←/→ cycle  type model  space toggle  enter run  esc back"))
	}
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
	if m.authCancel != nil || m.authFailed {
		provider := strings.ToUpper(m.authProv)
		if m.authFailed {
			b.WriteString(errStyle.Render("  "+provider+" authorization failed") + "\n\n")
			b.WriteString("  " + m.status + "\n\n")
			b.WriteString(mutedStyle.Render("  r retry  esc return") + "\n")
			return b.String()
		}
		b.WriteString(titleStyle.Render("  "+provider+" authorization in progress") + "\n\n")
		if m.authCode != nil {
			uri := m.authCode.VerificationURIComplete
			if uri == "" {
				uri = m.authCode.VerificationURI
			}
			b.WriteString("  1. Open this URL in your browser:\n  " + uri + "\n\n")
			b.WriteString("  2. Enter this code:\n  " + selStyle.Render(" "+m.authCode.UserCode+" ") + "\n\n")
		} else if m.pending != nil {
			b.WriteString("  Complete authorization in your browser:\n  " + m.pending.URL + "\n\n")
			if m.input == inputOAuthPaste {
				b.WriteString("  Paste the callback URL: " + m.editBuf + "█\n\n")
			}
		}
		b.WriteString(mutedStyle.Render("  "+m.status+"  c/esc cancel") + "\n")
		return b.String()
	}
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
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  enter login  k platform PAT  d device (xAI/GitLab/GitHub)  x logout  esc back"))
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

func (m Model) viewOnboarding() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("  First-run setup") + "\n\n")
	if m.input == inputOnboardingSecret {
		label := "API key"
		if m.onboardingStep == onboardingSecret {
			label = "personal access token"
		}
		b.WriteString("  " + label + ": " + strings.Repeat("•", len(m.editBuf)) + "█\n\n")
		b.WriteString(mutedStyle.Render("  enter save  esc cancel") + "\n")
		return b.String()
	}
	choices := m.onboardingChoices()
	label := "Select an AI provider"
	if m.onboardingStep == onboardingPlatform {
		label = "Select a Git platform"
		choices = []string{"github", "gitlab"}
	}
	b.WriteString("  " + label + "\n\n")
	for i, choice := range choices {
		line := "  " + choice
		if i == m.onboardingCursor {
			b.WriteString(selStyle.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  j/k move  enter select  existing credentials are reused") + "\n")
	return b.String()
}
