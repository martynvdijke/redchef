package handlers

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestGenerateIDConcurrentUniqueness ensures concurrent callers never receive
// the same ID (guards the data race between the upload handler and the
// asynchronous media processing goroutine).
func TestGenerateIDConcurrentUniqueness(t *testing.T) {
	const callers = 32
	const perCaller = 500

	ids := make(chan int64, callers*perCaller)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perCaller; j++ {
				ids <- generateID()
			}
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[int64]struct{}, callers*perCaller)
	for id := range ids {
		if _, dup := seen[id]; dup {
			t.Fatalf("generateID returned duplicate ID %d", id)
		}
		seen[id] = struct{}{}
	}
}

// TestGenerateIDNoCollisionAfterRestart simulates a process restart: the old
// process generated IDs, then the counter is re-seeded from the current time
// exactly like init() does on startup. IDs minted after the "restart" must not
// collide with any ID minted before it (this was the original bug — the counter
// restarted from 0 and new uploads overwrote existing media files).
func TestGenerateIDNoCollisionAfterRestart(t *testing.T) {
	before := make([]int64, 0, 1000)
	for i := 0; i < 1000; i++ {
		before = append(before, generateID())
	}

	// Simulate process restart: re-seed exactly as init() does.
	idMu.Lock()
	idCounter = time.Now().UnixNano()
	idMu.Unlock()

	for i := 0; i < 1000; i++ {
		id := generateID()
		for _, old := range before {
			if id == old {
				t.Fatalf("generateID reused ID %d after simulated restart", id)
			}
		}
	}
}

// TestProcessImageUsesPostID verifies the processed photo is stored under the
// post ID so different posts can never map to the same file, even across
// process restarts.
func TestProcessImageUsesPostID(t *testing.T) {
	dir := t.TempDir()
	uploadDir = dir

	// Create a wide source image so the resize path is exercised.
	srcPath := filepath.Join(dir, "raw.jpg")
	img := image.NewRGBA(image.Rect(0, 0, 3000, 200))
	for x := 0; x < 3000; x++ {
		img.Set(x, 0, color.RGBA{R: uint8(x % 256), G: 128, B: 64, A: 255})
	}
	f, err := os.Create(srcPath)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatalf("encode source: %v", err)
	}
	f.Close()
	defer os.Remove(srcPath)

	const postID = 987654321
	name, err := processImage(postID, srcPath, ".jpg")
	if err != nil {
		t.Fatalf("processImage: %v", err)
	}

	if want := "987654321.jpg"; name != want {
		t.Fatalf("processed filename = %q, want %q", name, want)
	}
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Fatalf("expected output file %s: %v", name, err)
	}
}
