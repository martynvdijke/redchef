package handlers

import (
	"fmt"
	"image"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
)

// ── Image Processing ──

var imageQuality int

func init() {
	q := os.Getenv("IMAGE_QUALITY")
	if q == "" {
		q = "85"
	}
	imageQuality, _ = strconv.Atoi(q)
	if imageQuality < 10 || imageQuality > 100 {
		imageQuality = 85
	}
}

// processImage resizes/compresses the uploaded photo and stores the result
// under the post ID, which is unique and monotonic in the database, so media
// filenames can never collide across uploads or process restarts.
func processImage(postID int64, srcPath, ext string) (string, error) {
	outFilename := fmt.Sprintf("%d%s", postID, ext)
	outPath := filepath.Join(uploadDir, outFilename)

	img, err := imaging.Open(srcPath, imaging.AutoOrientation(true))
	if err != nil {
		return "", fmt.Errorf("open image: %w", err)
	}

	// Resize if wider than 1920px
	bounds := img.Bounds()
	if bounds.Dx() > 1920 {
		img = imaging.Resize(img, 1920, 0, imaging.Lanczos)
	}

	// Use original extension; for JPEG/WebP, apply quality compression
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		err = imaging.Save(img, outPath, imaging.JPEGQuality(imageQuality))
	case ".png":
		// Save as PNG if it has transparency, otherwise convert to JPEG for smaller size
		if hasTransparency(img) {
			err = imaging.Save(img, outPath)
		} else {
			outFilename = strings.TrimSuffix(outFilename, ext) + ".jpg"
			outPath = filepath.Join(uploadDir, outFilename)
			err = imaging.Save(img, outPath, imaging.JPEGQuality(imageQuality))
		}
	case ".gif":
		// Static GIF: convert to JPEG
		outFilename = strings.TrimSuffix(outFilename, ext) + ".jpg"
		outPath = filepath.Join(uploadDir, outFilename)
		err = imaging.Save(img, outPath, imaging.JPEGQuality(imageQuality))
	case ".webp":
		// Go imaging doesn't support WebP output natively, convert to JPEG
		outFilename = strings.TrimSuffix(outFilename, ext) + ".jpg"
		outPath = filepath.Join(uploadDir, outFilename)
		err = imaging.Save(img, outPath, imaging.JPEGQuality(imageQuality))
	default:
		err = imaging.Save(img, outPath, imaging.JPEGQuality(imageQuality))
	}
	if err != nil {
		return "", fmt.Errorf("save image: %w", err)
	}

	log.Printf("[media] Image processed: %s (%dx%d, quality=%d)", outFilename, img.Bounds().Dx(), img.Bounds().Dy(), imageQuality)
	return outFilename, nil
}

func hasTransparency(img image.Image) bool {
	switch nrgba := img.(type) {
	case *image.NRGBA:
		for y := 0; y < nrgba.Bounds().Dy(); y++ {
			for x := 0; x < nrgba.Bounds().Dx(); x++ {
				_, _, _, a := nrgba.At(x, y).RGBA()
				if a < 65535 {
					return true
				}
			}
		}
	}
	return false
}

// ── Video Processing ──

func processVideo(postID int64, srcPath string) (string, string, error) {
	outFilename := fmt.Sprintf("%d.mp4", postID)
	outPath := filepath.Join(uploadDir, outFilename)

	thumbFilename := fmt.Sprintf("%d_thumb.jpg", postID)
	thumbPath := filepath.Join(uploadDir, thumbFilename)

	// Transcode to H.264/AAC, max 1080p, 8 Mbps
	cmd := exec.Command("ffmpeg",
		"-i", srcPath,
		"-vf", "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease",
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "23",
		"-maxrate", "8M",
		"-bufsize", "16M",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-y",
		outPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("ffmpeg transcode failed: %w\n%s", err, string(output))
	}

	// Extract thumbnail at 2 seconds
	thumbCmd := exec.Command("ffmpeg",
		"-i", srcPath,
		"-ss", "00:00:02",
		"-vframes", "1",
		"-vf", "scale=640:-1",
		"-y",
		thumbPath,
	)

	thumbOutput, err := thumbCmd.CombinedOutput()
	if err != nil {
		log.Printf("[media] Thumbnail extraction warning: %v\n%s", err, string(thumbOutput))
		// Use first frame as fallback
		thumbCmd = exec.Command("ffmpeg",
			"-i", outPath,
			"-vframes", "1",
			"-vf", "scale=640:-1",
			"-y",
			thumbPath,
		)
		thumbCmd.Run()
	}

	log.Printf("[media] Video processed: %s (thumbnail: %s)", outFilename, thumbFilename)
	return outFilename, thumbFilename, nil
}
