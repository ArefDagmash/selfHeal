package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"

	"testforge/ai"
	"testforge/events"
	"testforge/mockapi"
	"testforge/runner"
	"testforge/server"
	"testforge/storage"
)

const mockAPIAddr = ":8099"

func main() {
	fmt.Println("testforge ready")

	// Start the local "system under test" in-process so this command is fully
	// self-contained and deterministic (no flaky public API). In Docker it can
	// also run as the separate cmd/mockapi service.
	go func() {
		if err := mockapi.Start(mockAPIAddr); err != nil {
			fmt.Printf("mockapi exited: %v\n", err)
		}
	}()
	waitForMockAPI()

	// Start the live event server and feed every event to connected dashboards
	// the moment it is produced. The port is configurable (WS_PORT) and defaults
	// to 18080 because 8080 is commonly taken by other tools (e.g. Spark).
	wsPort := os.Getenv("WS_PORT")
	if wsPort == "" {
		wsPort = "18080"
	}
	hub := server.NewHub()
	go func() {
		if err := hub.Start(":" + wsPort); err != nil {
			fmt.Printf("ws server exited: %v\n", err)
		}
	}()
	suite := runner.Suite{Emit: func(e events.TestEvent) { hub.Broadcast(e) }}

	// runOnce executes the full pipeline a single time: rebuild the case list
	// (re-sampling the error pool so showcase mode doesn't loop the same
	// failures forever), run the suite, attach AI suggestions, and persist the
	// run. Kept as a closure so the interactive ("live") mode can repeat it,
	// keeping the dashboard always showing something current.
	runOnce := func() []events.TestEvent {
		suite.Cases = buildCases()
		all := suite.Run()
		hub.BroadcastBoundary(events.NewRunBoundary(len(all))) // all TestEvents for this run are out

		aiClient := ai.New()
		for i, ev := range all {
			if ev.Status != events.StatusFailed {
				continue
			}
			tc := suite.Cases[i]
			ctx := context.Background()
			start := time.Now()
			suggestion, err := aiClient.Suggest(ctx, ai.FailureContext{
				Name:           ev.TestName,
				Type:           string(ev.Type),
				ErrorType:      string(ev.ErrorType),
				ErrorMessage:   ev.ErrorMessage,
				URL:            tc.URL,
				ExpectedStatus: tc.ExpectedStatus,
				Selector:       tc.Selector,
			})
			if err != nil {
				if aiClient.Enabled() {
					fmt.Printf("ai suggestion failed for '%s': %v\n", ev.TestName, err)
				}
				continue
			}
			ev.Metadata["ai_suggestion"] = suggestion
			fmt.Printf("AI suggestion (%s) generated for '%s' in %dms\n",
				aiClient.Provider(), ev.TestName, time.Since(start).Milliseconds())
			hub.Broadcast(ev) // refresh the dashboard's current-test detail line
		}

		if aiClient.Enabled() {
			summarizeRun(aiClient, hub, all)
		}

		store, err := storage.Open("testforge.db")
		if err != nil {
			fmt.Printf("storage error: %v\n", err)
			return all
		}
		defer store.Close()
		if err := store.SaveRun(all); err != nil {
			fmt.Printf("storage error: %v\n", err)
		}
		return all
	}

	if os.Getenv("CI") != "" {
		// CI runs once, enforces the pass-rate gate, then exits.
		all := runOnce()
		if thr := passThreshold(); thr > 0 {
			rate := passRate(all)
			if rate < thr {
				fmt.Printf("FAIL: pass rate %d%% is below threshold %d%%\n", rate, thr)
				os.Exit(1)
			}
			fmt.Printf("pass rate %d%% (threshold %d%%)\n", rate, thr)
		}
		fmt.Println("CI mode: exiting")
		return
	}

	// Interactive "live" mode: run the suite once per dashboard connection
	// (a page load or a refresh) instead of looping on a timer regardless of
	// whether anyone's watching — a tab sitting open shouldn't keep
	// re-running the suite (and re-billing AI calls) on its own.
	// LOOP_DELAY_SECONDS now acts as a minimum cooldown between
	// connection-triggered runs, so a burst of rapid refreshes can't queue
	// up several expensive AI-calling runs back to back.
	cooldown := loopDelay()
	var lastRun time.Time
	fmt.Printf("live mode: one run per dashboard connection (min %s apart) — Ctrl-C to stop\n", cooldown)
	fmt.Printf("websocket server on :%s/ws\n", wsPort)
	for range hub.NewClient {
		if wait := cooldown - time.Since(lastRun); wait > 0 {
			time.Sleep(wait)
		}
		runOnce()
		lastRun = time.Now()
	}
}

// summarizeRun asks the AI client for a whole-run recap (distinct from the
// per-failure one-liners above) and broadcasts it as a RunReport once it's
// ready. Runs after every event for this run has already gone out, so the
// dashboard's "AI Summary" node always lands last.
func summarizeRun(aiClient *ai.Client, hub *server.Hub, all []events.TestEvent) {
	results := make([]ai.RunResult, 0, len(all))
	passed, failed, healed := 0, 0, 0
	for _, ev := range all {
		rr := ai.RunResult{Name: ev.TestName, Status: string(ev.Status), ErrorType: string(ev.ErrorType), ErrorMessage: ev.ErrorMessage}
		if ev.Status == events.StatusHealed {
			rr.OldSelector = ev.Metadata["old_selector"]
			rr.NewSelector = ev.Metadata["new_selector"]
		}
		results = append(results, rr)
		switch ev.Status {
		case events.StatusPassed:
			passed++
		case events.StatusFailed:
			failed++
		case events.StatusHealed:
			healed++
		}
	}

	start := time.Now()
	summary, err := aiClient.Summarize(context.Background(), results)
	if err != nil {
		fmt.Printf("ai summary failed: %v\n", err)
		return
	}
	fmt.Printf("AI summary (%s) generated in %dms\n", aiClient.Provider(), time.Since(start).Milliseconds())
	hub.BroadcastReport(events.NewRunReport(summary, passed, failed, healed))
}

// baseCases are the stable, deterministic tests that always run and always
// stay green — this is what CI runs, so it never gets touched by randomness.
func baseCases() []runner.TestCase {
	return []runner.TestCase{
		{Name: "api_status_200", Type: events.TypeAPI,
			URL: "http://localhost" + mockAPIAddr + "/status/200", ExpectedStatus: 200},
		{Name: "api_status_404", Type: events.TypeAPI,
			URL: "http://localhost" + mockAPIAddr + "/status/404", ExpectedStatus: 404},
		{Name: "example_h1_visible", Type: events.TypeUI,
			URL: "http://localhost" + mockAPIAddr + "/", Selector: "h1"},
		{Name: "shop_submit_healed", Type: events.TypeUI,
			URL: "http://localhost" + mockAPIAddr + "/shop", Selector: ".submit"}, // old name; heals to .submit-btn
	}
}

// errorPool are the showcase failure scenarios (assertion, network, selector
// failures) — 7 of them, fixed for now while the dashboard's mindmap view is
// tuned for a wide 7-cell column (nothing about buildCases requires this
// count; it's just what's being tested against right now). Each call
// randomizes the parameters within every scenario (which wrong status, which
// bogus selector) so a live demo's 7 problems vary run to run.
func errorPool() []runner.TestCase {
	wrongStatuses := []int{403, 418, 451, 500}
	badSelectors := []string{"#no-such-element-local", ".ghost-button", "#missing-cta", "#checkout-widget", ".legacy-toggle"}
	pick := func(opts []int) int { return opts[rand.IntN(len(opts))] }
	pickStr := func(opts []string) string { return opts[rand.IntN(len(opts))] }

	return []runner.TestCase{
		{Name: "broken_api_wrong_status", Type: events.TypeAPI,
			URL: "http://localhost" + mockAPIAddr + "/status/200", ExpectedStatus: pick(wrongStatuses)},
		{Name: "broken_api_dead_host", Type: events.TypeAPI,
			URL: "http://localhost:1", ExpectedStatus: 200},
		{Name: "broken_api_not_found", Type: events.TypeAPI,
			URL: "http://localhost" + mockAPIAddr + "/status/404", ExpectedStatus: 200},
		{Name: "broken_api_server_error", Type: events.TypeAPI,
			URL: "http://localhost" + mockAPIAddr + "/status/500", ExpectedStatus: 200},
		{Name: "example_missing_element", Type: events.TypeUI,
			URL: "http://localhost" + mockAPIAddr + "/shop", Selector: pickStr(badSelectors)},
		{Name: "example_wrong_selector_home", Type: events.TypeUI,
			URL: "http://localhost" + mockAPIAddr + "/", Selector: pickStr(badSelectors)},
		{Name: "shop_missing_checkout", Type: events.TypeUI,
			URL: "http://localhost" + mockAPIAddr + "/shop", Selector: pickStr(badSelectors)},
	}
}

// buildCases assembles one run's test cases: the stable base suite, plus (in
// showcase mode) the full error pool, re-randomized on every call.
func buildCases() []runner.TestCase {
	cases := baseCases()
	if os.Getenv("SHOWCASE_FAILURES") != "" {
		cases = append(cases, errorPool()...)
	}
	return cases
}

// passRate returns the percentage of events that are passed or healed.
func passRate(all []events.TestEvent) int {
	if len(all) == 0 {
		return 100
	}
	pass := 0
	for _, e := range all {
		if e.Status == events.StatusPassed || e.Status == events.StatusHealed {
			pass++
		}
	}
	return pass * 100 / len(all)
}

// passThreshold reads PASS_THRESHOLD. The gate is active when PASS_THRESHOLD is
// set, or by default in CI (80%). Locally it is disabled unless explicitly set,
// so a showcase run with intentional failures doesn't fail the process.
func passThreshold() int {
	v := os.Getenv("PASS_THRESHOLD")
	if v == "" {
		if os.Getenv("CI") != "" {
			return 80
		}
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

// loopDelay is the minimum cooldown between connection-triggered runs in live
// mode (LOOP_DELAY_SECONDS, default 6).
func loopDelay() time.Duration {
	v := os.Getenv("LOOP_DELAY_SECONDS")
	if v == "" {
		return 6 * time.Second
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 6 * time.Second
	}
	return time.Duration(n) * time.Second
}

// waitForMockAPI polls the mock API's health endpoint so the suite doesn't race
// the server's startup.
func waitForMockAPI() {
	health := "http://localhost" + mockAPIAddr + "/healthz"
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get(health); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Println("warning: mockapi health check timed out")
}
