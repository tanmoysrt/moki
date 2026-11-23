package main

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AgentNotes is the text that get_project shows: the approved version, then each open
// note with its id. Done notes are only counted, so the agent's context stays small.
func (n *NoteStore) AgentNotes(slug string) string {
	p := n.Read(slug)
	var b strings.Builder
	if p.Approved != "" {
		fmt.Fprintf(&b, "Approved: %s\n", p.Approved)
	}
	done, open := 0, 0
	for _, x := range p.Notes {
		if x.Done != "" {
			done++
			continue
		}
		if open++; open == 1 {
			b.WriteString("Open notes, oldest first. Answer them with answer_notes:\n")
		}
		writeAgentNote(&b, x)
	}
	if done > 0 {
		fmt.Fprintf(&b, "%s done. They are not shown.\n", count(done, "note"))
	}
	return b.String()
}

// agentNotesText serves the notes text of get_project over HTTP with the agent token,
// so an agent can wait for new notes or an approval in a shell loop in the background.
func (s *Server) agentNotesText(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.agentToken(w, r); !ok {
		return
	}
	slug := r.PathValue("slug")
	if checkSlug(slug) != nil || !isDir(filepath.Join(s.cfg.Dir, slug)) {
		http.NotFound(w, r)
		return
	}
	writeText(w, r, s.notes.AgentNotes(slug))
}

func writeAgentNote(b *strings.Builder, x Note) {
	fmt.Fprintf(b, "%s · %s", x.ID, x.Version)
	if x.By != "" {
		fmt.Fprintf(b, " · from %s, on the public link", x.By)
	}
	fmt.Fprintf(b, " · %s", x.Page)
	if x.Target != "" {
		fmt.Fprintf(b, " · %s", x.Target)
	}
	if x.Selector != "" {
		fmt.Fprintf(b, " (%s)", x.Selector)
	}
	fmt.Fprintf(b, "\n  %s\n", indent(x.Text))
	if x.Reply != "" {
		fmt.Fprintf(b, "  Your reply: %s\n", indent(x.Reply))
	}
}

func indent(s string) string {
	return strings.ReplaceAll(s, "\n", "\n  ")
}

// ---------- answer_notes ----------

type noteAnswer struct {
	ID    string `json:"id" jsonschema:"the note id from get_project"`
	Done  bool   `json:"done,omitempty" jsonschema:"true when a version fixed the note"`
	Reply string `json:"reply,omitempty" jsonschema:"one or two lines: what you changed, or a question back. The user sees it under the note"`
}

type answerIn struct {
	Slug    string       `json:"slug" jsonschema:"project slug"`
	Version string       `json:"version,omitempty" jsonschema:"the version that fixed the done notes. Default: the newest"`
	Notes   []noteAnswer `json:"notes" jsonschema:"one entry for each note you answer"`
}

const answerDesc = `Answer the user's notes after you upload the version that handles them. done: true marks a note
fixed in that version, and the viewer shows it under "Fixed in vN". reply is what you changed, or a question back;
a reply without done keeps the note open. Answer each note you worked on.`

func (s *Server) answerNotes(ctx context.Context, in answerIn) ([]mcp.Content, error) {
	if err := checkSlug(in.Slug); err != nil {
		return nil, err
	}
	p, _, err := s.store.Project(in.Slug)
	if err != nil {
		return nil, err
	}
	ver := cmp.Or(in.Version, p.Version)
	if checkVersion(ver) != nil || !isDir(s.store.VersionDir(in.Slug, ver)) {
		return nil, fmt.Errorf("%s has no version %s", in.Slug, ver)
	}
	for _, a := range in.Notes {
		if len(a.Reply) > maxNote {
			return nil, fmt.Errorf("the reply to %s is longer than 2000 bytes", a.ID)
		}
	}
	var done, replies, open int
	var unknown []string
	_, err = s.notes.Update(in.Slug, func(pn *ProjectNotes) {
		done, replies, unknown = applyAnswers(pn, in.Notes, ver)
		for _, n := range pn.Notes {
			if n.Done == "" {
				open++
			}
		}
	})
	if err != nil {
		return nil, err
	}
	s.hub.Publish(Event{Type: "notes", Slug: in.Slug})
	note(ctx, "%s · %d done in %s · %d replied", in.Slug, done, ver, replies)
	text := fmt.Sprintf("%s done in %s, %d with a reply. %s still open.", count(done, "note"), ver, replies, count(open, "note"))
	if len(unknown) > 0 {
		text += " No note has the id " + strings.Join(unknown, ", ") + ". Call get_project for the ids."
	}
	return textContent("%s", text), nil
}

func applyAnswers(pn *ProjectNotes, answers []noteAnswer, ver string) (done, replies int, unknown []string) {
	at := map[string]int{}
	for i, n := range pn.Notes {
		at[n.ID] = i
	}
	for _, a := range answers {
		i, ok := at[a.ID]
		if !ok {
			unknown = append(unknown, a.ID)
			continue
		}
		if a.Done {
			pn.Notes[i].Done = ver
			done++
		}
		if r := strings.TrimSpace(a.Reply); r != "" {
			pn.Notes[i].Reply = r
			replies++
		}
	}
	return done, replies, unknown
}
