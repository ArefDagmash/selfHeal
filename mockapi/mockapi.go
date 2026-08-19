package mockapi

import (
	"fmt"
	"net/http"
)

// Handler returns the HTTP routes that stand in for "the system under test".
// Each /status/<code> route answers with exactly that status code, so the test
// suite can assert against predictable responses without depending on a flaky
// public service like httpbin.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status/200", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/status/404", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/status/500", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "error")
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// "/" is a tiny home page so UI tests can run fully offline against the
	// local system-under-test instead of a flaky public site.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, `<!doctype html><html><head><title>TestForge</title></head><body>`)
		fmt.Fprintln(w, `<h1>TestForge</h1>`)
		fmt.Fprintln(w, `<p>self-healing test dashboard demo</p>`)
		fmt.Fprintln(w, `</body></html>`)
	})
	// /shop is a tiny page whose "buy" button has been RENAMED from the old
	// class "submit" to "submit-btn" — exactly the kind of drift self-healing
	// should recover from.
	mux.HandleFunc("/shop", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, `<!doctype html><html><head><title>Shop</title></head><body>`)
		fmt.Fprintln(w, `<h1>Shop</h1>`)
		fmt.Fprintln(w, `<button class="submit-btn" id="buy">Buy now</button>`)
		fmt.Fprintln(w, `</body></html>`)
	})
	return mux
}

// Start begins serving the mock API on addr (e.g. ":8099") and blocks.
func Start(addr string) error {
	srv := &http.Server{Addr: addr, Handler: Handler()}
	return srv.ListenAndServe()
}
