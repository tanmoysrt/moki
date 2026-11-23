package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// POST /api/upload/{slug} takes a tar or tar.gz of a folder and saves it as a version.
// It is the upload tool for agents on another machine: one curl line sends a whole
// folder, and no file goes through the agent's context. Same token as /mcp.
//
//	tar -cz -C mockup . | curl -sS --data-binary @- -H "Authorization: Bearer $TOKEN" \
//	  "$MOKI/api/upload/team-page?base=v3"
//
// Query: base, version, delete (repeat it), check=1 for the screenshot check.
func (s *Server) pushRoutes() {
	s.mux.HandleFunc("POST /api/upload/{slug}", s.push)
}

const maxPush = 256 << 20

func (s *Server) push(w http.ResponseWriter, r *http.Request) {
	value, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	tok, ok := s.tokens.Find(strings.TrimSpace(value))
	if !ok {
		http.Error(w, `missing or unknown token. Send the moki token as "Authorization: Bearer mk_..."`, http.StatusUnauthorized)
		return
	}
	s.tokens.MarkUsed(tok)
	r.Body = http.MaxBytesReader(w, r.Body, maxPush)
	start := time.Now()
	var text string
	ctx := context.WithValue(r.Context(), noteKey{}, &text)
	out, err := s.pushVersion(ctx, r)
	activity.Call(tok.Name, "upload", text, time.Since(start), err)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeText(w, r, out)
}

// pushVersion unpacks the body into a temporary folder and uploads it as dir.
func (s *Server) pushVersion(ctx context.Context, r *http.Request) (string, error) {
	q := r.URL.Query()
	tmp := filepath.Join(os.TempDir(), "moki-push-"+rand.Text()[:10])
	defer os.RemoveAll(tmp)
	if err := untar(r.Body, tmp); err != nil {
		return "", err
	}
	check := q.Get("check") == "1"
	content, err := s.upload(ctx, uploadIn{
		Slug: r.PathValue("slug"), Dir: tmp, Base: q.Get("base"), Version: q.Get("version"),
		Delete: q["delete"], Check: &check,
	})
	if err != nil {
		return "", err
	}
	text := content[0].(*mcp.TextContent).Text
	return text + "\n\nCall screenshot to see it.\n", nil
}

// untar writes the regular files of a tar or tar.gz into dir. Links are skipped,
// and every path must stay inside dir.
func untar(body io.Reader, dir string) error {
	in := bufio.NewReader(body)
	if magic, _ := in.Peek(2); len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(in)
		if err != nil {
			return fmt.Errorf("the body is not a valid tar.gz: %v", err)
		}
		defer gz.Close()
		in = bufio.NewReader(gz)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	tr := tar.NewReader(in)
	files := 0
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("the body is not a valid tar: %v. Send tar -cz -C <folder> .", err)
		}
		if h.Typeflag != tar.TypeReg || isHidden(h.Name) {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("could not read %s: %v. The limit is %d MB", h.Name, err, maxPush>>20)
		}
		if err := writeFile(root, fileIn{Path: h.Name, Content: string(data)}); err != nil {
			return err
		}
		files++
	}
	if files == 0 {
		return fmt.Errorf("the tar has no files. Send tar -cz -C <folder> .")
	}
	return nil
}

// isHidden skips .git and files such as .DS_Store, as dir does for .git.
func isHidden(name string) bool {
	for part := range strings.SplitSeq(strings.TrimPrefix(name, "./"), "/") {
		if strings.HasPrefix(part, ".") && part != "." {
			return true
		}
	}
	return false
}
