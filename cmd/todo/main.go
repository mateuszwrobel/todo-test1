// Command todo is the composition root: it opens the store at the configured
// path, builds the api handler over the store and the ui handlers with the
// api's base URL, mounts everything on one listener, and owns that lifecycle.
// It holds no business rule, no template, and no status-code mapping.
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"todo/api"
	"todo/todos"
	"todo/ui"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "todo:", err)
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("todo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8080", "listen address for page and JSON contract together")
	dbPath := fs.String("db", "todos.db", "data file path for the store")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("malformed flags: %w", err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	// Fixed wiring order: open the store → build the api handler over the
	// store → build the ui handlers with the api's base URL → register
	// routes → serve. The listener bind precedes the ui constructors so the
	// base URL is the address actually bound (exact even for ephemeral
	// ports); no lazy wiring.
	store, err := todos.Open(*dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	apiHandler := api.NewHandler(store)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", *addr, err)
	}

	baseURL := "http://" + ln.Addr().String()
	uiHandler := ui.NewHandler(baseURL)

	mux := http.NewServeMux()
	mux.Handle("/todos", apiHandler) // JSON contract
	mux.Handle("/", uiHandler)       // page, fragments, static

	fmt.Fprintf(stderr, "todo: serving on http://%s (db %s)\n", ln.Addr(), *dbPath)
	if err := http.Serve(ln, mux); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
