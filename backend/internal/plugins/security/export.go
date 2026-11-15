package security

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
	"time"
)

// Report exports (GET /export?format=md|csv). Both list every finding,
// accepted ones marked as such; Markdown adds the scores, the remediation
// of each failing check and what was skipped.

func resourceText(r Resource) string {
	return r.Kind + " " + joinName(r.Namespace, r.Name)
}

func scoreText(s *int) string {
	if s == nil {
		return "n/a"
	}
	return fmt.Sprintf("%d/100", *s)
}

// mdEscaper makes text inert inside a Markdown line or table cell: the
// cluster name, object names, an accepted risk's reason and Kubernetes
// errors are not the report's own, so they must not render as HTML,
// links, images, code or emphasis, nor break out of their cell or line.
var mdEscaper = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;",
	"\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "~", "\\~",
	"[", "\\[", "]", "\\]", "(", "\\(", ")", "\\)", "!", "\\!", "|", "\\|",
	"\r", " ", "\n", " ",
)

func mdText(s string) string { return mdEscaper.Replace(s) }

func acceptedText(a *Acceptance) string {
	if a == nil {
		return "open"
	}
	text := "accepted by " + a.By
	if a.ExpiresAt != nil {
		text += " until " + a.ExpiresAt.UTC().Format(time.DateOnly)
	}
	return text + ": " + a.Reason
}

func exportMarkdown(rep *Report) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Security posture: %s\n\n", mdText(rep.Cluster))
	fmt.Fprintf(&b, "Generated %s", rep.GeneratedAt.UTC().Format(time.RFC3339))
	if rep.Options.IncludeSystem {
		b.WriteString(", system namespaces included")
	} else {
		fmt.Fprintf(&b, ", system namespaces excluded (%s)", mdText(strings.Join(rep.SystemNamespaces, ", ")))
	}
	b.WriteString(".\n\n")

	fmt.Fprintf(&b, "**Score: %s**\n\n", scoreText(rep.Score.Overall))
	b.WriteString("| Category | Score | Open findings | Accepted |\n|---|---|---|---|\n")
	for _, c := range rep.Score.ByCategory {
		fmt.Fprintf(&b, "| %s | %s | %d | %d |\n", c.Category, scoreText(c.Score), c.Failed, c.Accepted)
	}
	fmt.Fprintf(&b, "\nOpen findings: %d critical, %d high, %d medium, %d low; %d accepted.\n\n",
		rep.Counts.Critical, rep.Counts.High, rep.Counts.Medium, rep.Counts.Low, rep.Counts.Accepted)

	b.WriteString("## Findings\n\n")
	if len(rep.Findings) == 0 {
		b.WriteString("No findings.\n\n")
	} else {
		b.WriteString("| Severity | Check | Resource | Message | Frameworks | Status |\n|---|---|---|---|---|---|\n")
		for _, f := range rep.Findings {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", f.Severity, mdText(f.Title), mdText(resourceText(f.Resource)),
				mdText(f.Message), mdText(strings.Join(f.Frameworks, ", ")), mdText(acceptedText(f.Accepted)))
		}
		b.WriteString("\n## Remediation\n\n")
		seen := map[string]bool{}
		for _, f := range rep.Findings {
			if !seen[f.CheckID] {
				seen[f.CheckID] = true
				fmt.Fprintf(&b, "- **%s** (%s): %s\n", mdText(f.Title), f.CheckID, mdText(f.Remediation))
			}
		}
		b.WriteString("\n")
	}
	if len(rep.Skipped) > 0 {
		b.WriteString("## Skipped checks\n\n")
		for _, s := range rep.Skipped {
			fmt.Fprintf(&b, "- %s: %s\n", s.CheckID, mdText(s.Reason))
		}
		b.WriteString("\n")
	}
	if len(rep.Warnings) > 0 {
		b.WriteString("## Warnings\n\n")
		for _, w := range rep.Warnings {
			fmt.Fprintf(&b, "- %s\n", mdText(w))
		}
		b.WriteString("\n")
	}
	return b.Bytes()
}

// csvCell defuses a cell a spreadsheet would run as a formula (=, +, -,
// @, tab, CR): an accepted risk's reason is free text.
func csvCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func exportCSV(rep *Report) ([]byte, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	rows := [][]string{{
		"severity", "check_id", "title", "category", "frameworks", "kind", "namespace", "name",
		"message", "remediation", "status", "accepted_by", "accepted_reason", "accepted_until",
	}}
	for _, f := range rep.Findings {
		status, by, reason, until := "open", "", "", ""
		if a := f.Accepted; a != nil {
			status, by, reason = "accepted", a.By, a.Reason
			if a.ExpiresAt != nil {
				until = a.ExpiresAt.UTC().Format(time.RFC3339)
			}
		}
		row := []string{
			f.Severity, f.CheckID, f.Title, f.Category, strings.Join(f.Frameworks, "; "),
			f.Resource.Kind, f.Resource.Namespace, f.Resource.Name, f.Message, f.Remediation,
			status, by, reason, until,
		}
		for i := range row {
			row[i] = csvCell(row[i])
		}
		rows = append(rows, row)
	}
	if err := w.WriteAll(rows); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
