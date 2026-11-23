package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const maxVideo = 60 // seconds

// mp4 steps the video frame by frame with moki.seek(t), takes each frame, and pipes
// the frames to ffmpeg. draw(t) depends only on t, so no frame is dropped or doubled.
func (r *renderer) mp4() ([]byte, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg is not installed on the moki machine. Install it to make MP4 files")
	}
	fps := r.info.Fps
	if fps < 1 || fps > 60 {
		fps = 30
	}
	if r.info.Duration <= 0 || r.info.Duration > maxVideo {
		return nil, fmt.Errorf("the video is %.1f s long. MP4 export takes videos up to %d s", r.info.Duration, maxVideo)
	}
	dir, err := os.MkdirTemp("", "moki-mp4-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "video.mp4")
	var stderr bytes.Buffer
	cmd := exec.CommandContext(r.ctx, "ffmpeg", "-loglevel", "error", "-f", "image2pipe", "-framerate", fmt.Sprint(fps), "-i", "-",
		"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-crf", "20",
		"-movflags", "+faststart", "-y", out)
	cmd.Stderr = &stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	n := int(math.Ceil(r.info.Duration * float64(fps)))
	err = r.frames(n, fps, func(jpg []byte) error { _, err := in.Write(jpg); return err })
	in.Close()
	if werr := cmd.Wait(); err == nil && werr != nil {
		err = fmt.Errorf("ffmpeg: %v %s", werr, stderr.String())
	}
	if err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}

func (r *renderer) frames(n, fps int, write func([]byte) error) error {
	shot := page.CaptureScreenshotParams{Format: page.CaptureScreenshotFormatJpeg, Quality: ptr(int64(92)), OptimizeForSpeed: ptr(true)}
	for i := range n {
		js := fmt.Sprintf(`moki.seek(%f), true`, float64(i)/float64(fps))
		if _, err := chromedp.Run(r.ctx, chromedp.Evaluate[bool](js)); err != nil {
			return err
		}
		res, err := chromedp.Call(r.ctx, page.CaptureScreenshot, shot)
		if err != nil {
			return err
		}
		if err := write(res.Data); err != nil {
			return err
		}
		r.prog(float64(i+1) / float64(n))
	}
	return nil
}
