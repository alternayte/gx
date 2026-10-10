// Package chrome runs one headless Chrome through chromedp for the tools
// that look at a rendered page: the screenshots and the audits of the dev
// MCP server (REQ-AI-04) and the audits of `gx fuzz` (REQ-AI-11). The audit
// uses the vendored axe-core. No node.
package chrome

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// axeSource is axe-core 4.13.0 (MPL-2.0, see axe.LICENSE). The audit runs
// it inside the page; the test pins its hash.
//
//go:embed axe.min.js
var axeSource string

// AxeVersion and AxeSHA256 pin the embedded axe-core build.
const (
	AxeVersion = "4.13.0"
	AxeSHA256  = "c24f097bd2f451d4f933e8bc7d8d539f8672a2ebcb5cc9f9f3eec8ca9470a0c1"
)

// SumAxe returns the hex sha256 of an axe-core build.
func SumAxe(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// AuditNode is one element that fails a rule.
type AuditNode struct {
	Target  []string `json:"target"`
	HTML    string   `json:"html"`
	Summary string   `json:"summary"`
}

// Violation is one failed axe rule.
type Violation struct {
	ID      string      `json:"id"`
	Impact  string      `json:"impact"`
	Help    string      `json:"help"`
	HelpURL string      `json:"helpUrl"`
	Nodes   []AuditNode `json:"nodes"`
}

// AuditOutput is the result of one audit.
type AuditOutput struct {
	Violations []Violation `json:"violations"`
	// Passes is the number of rules that passed.
	Passes int    `json:"passes"`
	Axe    string `json:"axe"`
}

// auditScript runs axe and returns JSON text. %s is the JSON of a CSS
// selector, or of the empty string for the document.
const auditScript = `(async () => {
  const selector = %s;
  const r = await axe.run(selector ? document.querySelector(selector) : document);
  return JSON.stringify({
    passes: r.passes.length,
    violations: r.violations.map((v) => ({
      id: v.id, impact: v.impact || "", help: v.help, helpUrl: v.helpUrl,
      nodes: v.nodes.map((n) => ({ target: n.target.map(String), html: n.html, summary: n.failureSummary || "" })),
    })),
  });
})()`

// Browser is one headless Chrome. It starts on the first tab. The zero
// value is ready to use.
type Browser struct {
	mu      sync.Mutex
	browser context.Context
	closers []context.CancelFunc
}

// Close ends Chrome.
func (b *Browser) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := len(b.closers) - 1; i >= 0; i-- {
		b.closers[i]()
	}
	b.browser, b.closers = nil, nil
}

// tab opens a new tab of the shared headless Chrome.
func (b *Browser) tab(ctx context.Context) (context.Context, context.CancelFunc, error) {
	b.mu.Lock()
	if b.browser == nil {
		alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:],
			// The first start of Chrome on a machine with load can take
			// longer than the 20 s default.
			chromedp.WSURLReadTimeout(90*time.Second))...)
		browser, cancelBrowser := chromedp.NewContext(alloc)
		if err := chromedp.Run(browser); err != nil {
			cancelBrowser()
			cancelAlloc()
			b.mu.Unlock()
			return nil, nil, fmt.Errorf("start headless Chrome: %w. Install Chrome or Chromium", err)
		}
		b.browser = browser
		b.closers = append(b.closers, cancelAlloc, cancelBrowser)
	}
	browser := b.browser
	b.mu.Unlock()
	tab, cancelTab := chromedp.NewContext(browser)
	tab, cancelTimeout := context.WithTimeout(tab, 60*time.Second)
	stop := context.AfterFunc(ctx, cancelTab)
	return tab, func() {
		stop()
		cancelTimeout()
		cancelTab()
	}, nil
}

// Screenshot returns a PNG of the full page at target. A width or a height
// of zero is 1280 or 800.
func (b *Browser) Screenshot(ctx context.Context, target string, width, height int) ([]byte, error) {
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 800
	}
	tab, cancel, err := b.tab(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	var shot []byte
	if err := chromedp.Run(tab,
		chromedp.EmulateViewport(int64(width), int64(height)),
		chromedp.Navigate(target),
		chromedp.WaitReady("body", chromedp.ByQuery),
		// Quality 100 gives a PNG.
		chromedp.FullScreenshot(&shot, 100),
	); err != nil {
		return nil, fmt.Errorf("screenshot in Chrome: %w", err)
	}
	return shot, nil
}

// Audit runs axe on the page at target. A selector limits the audit to one
// element: the rules for a whole page, such as the rule for a level-one
// heading, then do not apply. An empty selector audits the document.
func (b *Browser) Audit(ctx context.Context, target, selector string) (AuditOutput, error) {
	sel, err := json.Marshal(selector)
	if err != nil {
		return AuditOutput{}, err
	}
	tab, cancel, err := b.tab(ctx)
	if err != nil {
		return AuditOutput{}, err
	}
	defer cancel()
	var raw string
	await := func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }
	if err := chromedp.Run(tab,
		chromedp.EmulateViewport(1280, 800),
		chromedp.Navigate(target),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(axeSource, nil),
		chromedp.Evaluate(fmt.Sprintf(auditScript, sel), &raw, await),
	); err != nil {
		return AuditOutput{}, fmt.Errorf("audit in Chrome: %w", err)
	}
	out := AuditOutput{Axe: AxeVersion}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return AuditOutput{}, fmt.Errorf("axe result: %w", err)
	}
	if out.Violations == nil {
		out.Violations = []Violation{}
	}
	return out, nil
}
