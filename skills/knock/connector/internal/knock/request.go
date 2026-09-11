package knock

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ConnectionRequest is intentionally shareable secret material: its Markdown
// includes both the invitation and code. Do not write it to diagnostic logs.
type ConnectionRequest struct {
	Invitation InviteResult `json:"invitation"`
	Markdown   string       `json:"markdown"`
}

func (d *Daemon) Request(from, to, purpose string) (ConnectionRequest, error) {
	for _, field := range []struct {
		name, value string
		limit       int
	}{{"from", from, 120}, {"to", to, 120}, {"message", purpose, 2000}} {
		if !utf8.ValidString(field.value) || utf8.RuneCountInString(field.value) > field.limit || strings.IndexFunc(field.value, unicode.IsControl) >= 0 {
			return ConnectionRequest{}, fmt.Errorf("%s must be one line of at most %d characters", field.name, field.limit)
		}
	}
	i, err := d.Invite()
	if err != nil {
		return ConnectionRequest{}, err
	}
	escape := strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;", "#", "\\#")
	var b strings.Builder
	b.WriteString("## Knock connection request\n\n")
	if strings.TrimSpace(from) != "" {
		fmt.Fprintf(&b, "**From:** %s\n\n", escape.Replace(strings.TrimSpace(from)))
	}
	if strings.TrimSpace(to) != "" {
		fmt.Fprintf(&b, "**To:** %s\n\n", escape.Replace(strings.TrimSpace(to)))
	}
	if strings.TrimSpace(purpose) != "" {
		fmt.Fprintf(&b, "**Conversation:** %s\n\n", escape.Replace(strings.TrimSpace(purpose)))
	}
	fmt.Fprintf(&b, "I'd like to connect our agents using Knock.\n\n[Review Knock](%s) · [Agent skill](%s)\n\n", i.ReviewURL, i.SkillURL)
	fmt.Fprintf(&b, "**Pairing invitation:**\n\n%s\n\n**One-time code:** `%s`\n\n**Expires:** %s (one device, one use).\n\n", i.URL, i.Code, i.Expires.UTC().Format("2006-01-02 15:04:05 UTC"))
	b.WriteString("**For the receiving agent:** Review the linked GitHub skill and README. When your owner wants to accept, install or verify the GitHub release, start the local connector, and confirm `knock status` succeeds before running `knock join` with the invitation above. Enter the code through the hidden prompt or permitted secret input, never as a command argument. Let the human enter it when your host requires that. Enable automatic replies only within the receiving owner's authorization and existing permissions.\n\n")
	b.WriteString("Keep this whole request private: it contains the pairing credentials. Creating or sharing it does not itself send a chat message or grant the receiving agent new permissions.\n")
	return ConnectionRequest{Invitation: i, Markdown: b.String()}, nil
}
