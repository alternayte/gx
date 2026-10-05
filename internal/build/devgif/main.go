// Command devgif records the dev loop of a new app as an animated GIF for
// the README (REQ-DOC-05). It makes an app with the scaffold of `gx init`,
// runs `gx dev` on it, opens the page in headless Chrome, changes a signal,
// edits a .gx file and takes a frame at each step. `just readme-gif` runs
// it. It needs Chrome and, on a cold cache, the Tailwind download.
//
//	go run ./internal/build/devgif [--out docs/assets/dev-loop.gif]
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"

	"github.com/alternayte/gx/internal/devserver"
	"github.com/alternayte/gx/internal/scaffold"
)

func main() {
	out := flag.String("out", "docs/assets/dev-loop.gif", "the GIF file to write")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "devgif:", err)
		os.Exit(1)
	}
}

// caption puts one line of text in a bar at the bottom of the page.
const caption = `(() => {
  let el = document.getElementById("devgif-caption");
  if (!el) {
    el = document.createElement("div");
    el.id = "devgif-caption";
    el.style.cssText = "position:fixed;left:0;right:0;bottom:0;padding:14px 18px;background:#111827;color:#f9fafb;font:15px ui-monospace,SFMono-Regular,Menlo,monospace;z-index:99999";
    document.body.append(el);
  }
  el.textContent = %q;
})()`

func run(out string) error {
	repo, err := os.Getwd()
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "gx-devgif-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	app := filepath.Join(tmp, "acme")
	if _, err := scaffold.Init(scaffold.Options{
		Dir: app, Adapter: "datastar", Version: "0.1.0", Replace: repo,
		Registry: filepath.Join(repo, "registry"),
	}); err != nil {
		return err
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	addr := l.Addr().String()
	_ = l.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	devDone := make(chan error, 1)
	go func() { devDone <- devserver.Run(ctx, devserver.Options{Dir: app, Addr: addr, Log: io.Discard}) }()
	url := "http://" + addr + "/"
	if err := waitFor(url, "Welcome to acme", devDone); err != nil {
		return err
	}

	alloc, cancelAlloc := chromedp.NewExecAllocator(ctx, chromedp.DefaultExecAllocatorOptions[:]...)
	defer cancelAlloc()
	tab, cancelTab := chromedp.NewContext(alloc)
	defer cancelTab()
	tab, cancelTimeout := context.WithTimeout(tab, 3*time.Minute)
	defer cancelTimeout()

	var frames []image.Image
	frame := func(text string) chromedp.Action {
		return chromedp.ActionFunc(func(ctx context.Context) error {
			var shot []byte
			if err := chromedp.Run(ctx,
				chromedp.Evaluate(fmt.Sprintf(caption, text), nil),
				chromedp.Sleep(150*time.Millisecond),
				chromedp.CaptureScreenshot(&shot),
			); err != nil {
				return err
			}
			img, err := png.Decode(bytes.NewReader(shot))
			if err != nil {
				return err
			}
			frames = append(frames, img)
			return nil
		})
	}
	// waitText waits until a selector holds a text.
	waitText := func(selector, want string) chromedp.Action {
		return chromedp.ActionFunc(func(ctx context.Context) error {
			deadline := time.Now().Add(90 * time.Second)
			for {
				var got string
				js := fmt.Sprintf(`(document.querySelector(%q) || {}).textContent || ""`, selector)
				if err := chromedp.Evaluate(js, &got).Do(ctx); err != nil {
					return err
				}
				if strings.Contains(got, want) {
					return nil
				}
				if time.Now().After(deadline) {
					return fmt.Errorf("%s holds %q, want %q", selector, got, want)
				}
				time.Sleep(100 * time.Millisecond)
			}
		})
	}
	// count clicks the first button until the counter shows n. The first
	// click can come before the adapter script is ready, so a click that
	// does not count is made again.
	count := func(n int) chromedp.Action {
		return chromedp.ActionFunc(func(ctx context.Context) error {
			want := fmt.Sprint(n)
			deadline := time.Now().Add(30 * time.Second)
			for {
				if err := chromedp.Evaluate(`document.querySelector("section button").click()`, nil).Do(ctx); err != nil {
					return err
				}
				for i := 0; i < 10; i++ {
					var got string
					if err := chromedp.Evaluate(`document.querySelector("section span").textContent`, &got).Do(ctx); err != nil {
						return err
					}
					if got == want {
						return nil
					}
					time.Sleep(50 * time.Millisecond)
				}
				if time.Now().After(deadline) {
					return fmt.Errorf("the counter did not reach %d", n)
				}
			}
		})
	}
	edit := chromedp.ActionFunc(func(context.Context) error {
		path := filepath.Join(app, "home", "Home.gx")
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		next := strings.Replace(string(src), "Welcome to acme", "Hello from acme", 1)
		if next == string(src) {
			return fmt.Errorf("home/Home.gx has no heading to edit")
		}
		return os.WriteFile(path, []byte(next), 0o644)
	})
	if err := chromedp.Run(tab,
		chromedp.EmulateViewport(900, 520),
		// The light theme, whatever the system has.
		emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: "light"}}),
		chromedp.Navigate(url),
		waitText("h1", "Welcome to acme"),
		frame("$ go run ./cmd/gx dev"),
		count(1), count(2), count(3),
		frame("Click three times. The count is a signal in the browser."),
		frame("Edit home/Home.gx: \"Welcome to acme\" becomes \"Hello from acme\". Save."),
		edit,
		waitText("h1", "Hello from acme"),
		waitText("section span", "3"),
		frame("The page updates in place. The count is still 3."),
	); err != nil {
		return err
	}

	anim := &gif.GIF{}
	delays := []int{180, 220, 260, 400} // hundredths of a second
	for i, img := range frames {
		paletted := image.NewPaletted(img.Bounds(), palette.Plan9)
		draw.FloydSteinberg.Draw(paletted, img.Bounds(), img, image.Point{})
		anim.Image = append(anim.Image, paletted)
		anim.Delay = append(anim.Delay, delays[i%len(delays)])
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, anim); err != nil {
		return err
	}
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s: %d frames, %d bytes\n", out, len(frames), buf.Len())
	return nil
}

// waitFor reads url until the page holds want. The first build runs
// Tailwind, so it can take a minute.
func waitFor(url, want string, devDone <-chan error) error {
	deadline := time.Now().Add(4 * time.Minute)
	for {
		select {
		case err := <-devDone:
			return fmt.Errorf("gx dev stopped: %v", err)
		default:
		}
		resp, err := http.Get(url)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if strings.Contains(string(body), want) {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the app did not answer at %s", url)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
