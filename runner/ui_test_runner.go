package runner

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"testforge/events"

	"github.com/chromedp/chromedp"
	"golang.org/x/net/html"
)

// RunUITest navigates a headless Chrome instance to url and checks whether the
// given CSS selector resolves to at least one element on the page. It returns
// a passed/failed TestEvent. This is the building block that later phases will
// extend with retry and self-healing when the selector can't be found.
func RunUITest(name, url, selector string) events.TestEvent {
	start := time.Now()
	ev := events.NewTestEvent(name, events.TypeUI, events.StatusRunning)
	ev.Selector = selector

	// Run headless Chrome. Allocate a fresh browser per test so a crash in one
	// test can't poison the next; cheap enough for a portfolio demo.
	// The Chrome binary path is configurable (CHROME_PATH) so the same code
	// runs on a dev machine and inside a container that ships chromium.
	chromePath := os.Getenv("CHROME_PATH")
	if chromePath == "" {
		chromePath = "/usr/bin/google-chrome"
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Headless,
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", true),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 20*time.Second)
	defer cancelTimeout()

	var found bool
	var outer string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &outer),
		chromedp.Evaluate(`document.querySelectorAll(`+jsString(selector)+`).length > 0`, &found),
	)

	ev.DurationMs = time.Since(start).Milliseconds()

	if err != nil {
		ev.Status = events.StatusFailed
		ev.ErrorMessage = "navigation/eval failed: " + err.Error()
		// Page load failures surface as navigation timeouts or network errors
		// from Chrome; selector evaluation itself shouldn't error here.
		if isTimeoutError(err) {
			ev.ErrorType = events.ErrTimeout
		} else {
			ev.ErrorType = events.ErrNetwork
		}
		return ev
	}
	if found {
		ev.Status = events.StatusPassed
		ev.ErrorType = events.ErrNone
		return ev
	}

	// Selector not found: attempt self-healing by scoring the page's elements
	// against the original selector and retrying with the best candidate.
	ev.Status = events.StatusFailed
	ev.ErrorType = events.ErrSelectorNotFound
	ev.ErrorMessage = "selector not found: " + selector

	if doc, perr := html.Parse(strings.NewReader(outer)); perr == nil {
		sp := parseSelector(selector)
		best, ok := scoreCandidates(doc, sp)
		if ok {
			fmt.Printf("[HEAL] %s: old=%s best='%s' score=%d\n",
				name, selector, candidateSelector(best, sp), best.score)
		}
		if ok && best.score >= healThreshold {
			newSel := candidateSelector(best, sp)
			var healed bool
			_ = chromedp.Run(ctx,
				chromedp.Evaluate(`document.querySelectorAll(`+jsString(newSel)+`).length > 0`, &healed),
			)
			if healed {
				ev.Status = events.StatusHealed
				ev.ErrorType = events.ErrNone
				ev.Selector = newSel
				ev.ErrorMessage = ""
				ev.Metadata["old_selector"] = selector
				ev.Metadata["new_selector"] = newSel
				ev.Metadata["heal_score"] = strconv.Itoa(best.score)
				fmt.Printf("[HEALED] %s: '%s' -> '%s' (score: %d)\n",
					name, selector, newSel, best.score)
				return ev
			}
		}
	}
	return ev
}

// jsString wraps a Go string as a safely-quoted JavaScript string literal
// (single quotes, with internal quotes escaped) for use inside Evaluate.
func jsString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "\\'") + "'"
}
