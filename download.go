package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type downloadIn struct {
	Slug    string `json:"slug" jsonschema:"project slug"`
	Version string `json:"version,omitempty" jsonschema:"for example v3. Without it, the whole project"`
	Dir     string `json:"dir,omitempty" jsonschema:"absolute folder on the moki machine to write the files into, instead of returning an archive"`
}

const maxDownload = 20 << 20

func (s *Server) download(ctx context.Context, in downloadIn) ([]mcp.Content, error) {
	if err := checkSlug(in.Slug); err != nil {
		return nil, err
	}
	if _, _, err := s.store.Project(in.Slug); err != nil {
		return nil, err
	}
	rel := in.Slug
	if in.Version != "" {
		if err := checkVersion(in.Version); err != nil {
			return nil, err
		}
		rel = filepath.Join(in.Slug, in.Version)
		if _, err := os.Stat(filepath.Join(s.cfg.Dir, rel)); err != nil {
			return nil, fmt.Errorf("version %s of %q does not exist. Call get_project to see the versions", in.Version, in.Slug)
		}
	}
	if in.Dir != "" {
		dir, err := absPath(in.Dir)
		if err != nil {
			return nil, err
		}
		files, err := copyTree(filepath.Join(s.cfg.Dir, rel), filepath.Join(dir, rel))
		if err != nil {
			return nil, err
		}
		note(ctx, "%s · %s to %s", rel, count(len(files), "file"), tildePath(dir))
		return textContent("Wrote %d files into %s:\n%s", len(files), filepath.Join(dir, rel), joinLines(files)), nil
	}
	b, err := tarGz(s.cfg.Dir, rel)
	if err != nil {
		return nil, err
	}
	if len(b) > maxDownload {
		return nil, fmt.Errorf("the archive is %d MB, above the 20 MB limit. Use dir to write the files on the moki machine", len(b)>>20)
	}
	note(ctx, "%s · %s", rel, byteSize(len(b)))
	name := filepath.Base(rel) + ".tar.gz"
	if in.Version != "" {
		name = in.Slug + "-" + in.Version + ".tar.gz"
	}
	return []mcp.Content{
		&mcp.TextContent{Text: fmt.Sprintf("%s: %d bytes. The layout is %s/...", name, len(b), filepath.ToSlash(rel))},
		&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{URI: "moki://download/" + name, MIMEType: "application/gzip", Blob: b}},
	}, nil
}

// tarGz packs dir/rel with paths that start at rel, for example teams/v3/index.html.
func tarGz(dir, rel string) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	err := filepath.Walk(filepath.Join(dir, rel), func(p string, fi os.FileInfo, err error) error {
		if err != nil || !fi.Mode().IsRegular() {
			return err
		}
		name, _ := filepath.Rel(dir, p)
		if err := tw.WriteHeader(&tar.Header{Name: filepath.ToSlash(name), Mode: 0o644, Size: fi.Size(), ModTime: fi.ModTime()}); err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		if buf.Len() > maxDownload {
			return fmt.Errorf("the archive is above the 20 MB limit. Use dir to write the files on the moki machine")
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	tw.Close()
	gz.Close()
	return buf.Bytes(), nil
}
