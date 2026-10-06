// Command todo is the composition root: it opens the board store at the
// configured path, builds the api handler over the store and the ui handlers
// with the api's base URL, mounts everything on one listener, and owns that
// lifecycle. It holds no business rule, no template, and no status-code
// mapping.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"todo/api"
	"todo/board"
	"todo/ui"
)

// shutdownDrainTimeout bounds how long shutdown waits for in-flight requests.
// After the bound the remaining connections are closed (their requests fail)
// and the store still closes, so completed operations stay durable.
const shutdownDrainTimeout = 5 * time.Second

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
	boardDBPath := fs.String("board-db", "kanban.db", "board data file path")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("malformed flags: %w", err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	// Fixed wiring order: open the board store → build the api handler over
	// the store → build the ui handlers with the api's base URL → register
	// routes → serve. The listener bind precedes the ui constructors so the
	// base URL is the address actually bound (exact even for ephemeral
	// ports); no lazy wiring. The board store is the only data file: the
	// todo store and its --db flag retired with the last todo endpoint
	// (api/10). An absent board file opens as an empty board (board/01).
	boardStore, err := board.Open(*boardDBPath)
	if err != nil {
		return err
	}
	defer boardStore.Close()

	apiHandler := api.NewHandler(boardStore)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", *addr, err)
	}

	baseURL := "http://" + ln.Addr().String()
	uiHandler := ui.NewHandler(baseURL)

	mux := http.NewServeMux()
	mux.Handle("/board", apiHandler)  // JSON contract: board read
	mux.Handle("/cards", apiHandler)  // JSON contract: card create (KW2, api/02)
	mux.Handle("/cards/", apiHandler) // JSON contract: /cards/{id} items (change at KW3/api/05, delete at KW4/api/10)
	mux.Handle("/", uiHandler)        // page, fragments, static

	fmt.Fprintf(stderr, "todo: serving on http://%s (board db %s)\n", ln.Addr(), *boardDBPath)

	// Graceful shutdown: SIGINT/SIGTERM stops accepting, in-flight requests
	// drain within a bounded timeout, the store closes after them (the
	// deferred Close below), and the process exits 0.
	srv := &http.Server{Handler: mux}
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve(ln) }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveDone:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	drainCtx, cancelDrain := context.WithTimeout(context.Background(), shutdownDrainTimeout)
	defer cancelDrain()
	if err := srv.Shutdown(drainCtx); err != nil {
		srv.Close() // bound reached: remaining requests fail, store still closes
	}
	<-serveDone // the listener is stopped
	return nil
}
