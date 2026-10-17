package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/tonylook/managents/helper/internal/app"
	"github.com/tonylook/managents/helper/internal/detect"
	"github.com/tonylook/managents/helper/internal/detect/claude"
	"github.com/tonylook/managents/helper/internal/httpserve"
	"github.com/tonylook/managents/helper/internal/process"
)

// serveCommand publishes Claude Code session state over HTTP for network
// displays such as the Roku channel. See docs/http-api.md.
func serveCommand(ctx context.Context, inv *invocation) error {
	flags := inv.flags()
	addr := flags.String("addr", ":8787",
		"address to listen on; YOUR_LAN_IP:8787 keeps it off other interfaces")
	logLevel := flags.String("log-level", "info", "debug, info, warn or error")
	claudeWindow := claudeContextFlag(flags)
	titles := flags.Bool("titles", true,
		"send each session's title (as the Claude app shows it) along with its folder name")
	if err := inv.parse(flags); err != nil {
		return err
	}
	token := os.Getenv("MANAGENTS_TOKEN")
	if token == "" && !httpserve.IsLoopback(*addr) {
		return inv.usageError(flags,
			"set MANAGENTS_TOKEN before listening on %s, for example: export MANAGENTS_TOKEN=$(openssl rand -hex 16)", *addr)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	logger, err := newLogger(inv.stderr, *logLevel)
	if err != nil {
		return err
	}

	claudeSource := claude.NewSource(home, process.System{})
	claudeSource.ContextWindow = *claudeWindow
	claudeSource.Titles = *titles
	server := httpserve.New(token)

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	httpServer := &http.Server{Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.Serve(listener) }()

	runCtx, stopRunner := context.WithCancel(ctx)
	defer stopRunner()
	runnerDone := make(chan error, 1)
	go func() {
		runnerDone <- app.NewRunner([]detect.Source{claudeSource}, server, logger).Run(runCtx)
	}()
	logger.Info("serving Claude session state", "addr", listener.Addr().String(),
		"auth", token != "", "version", currentVersion())

	var result error
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			result = err
		}
	case <-ctx.Done():
	}
	stopRunner()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	<-runnerDone
	return result
}
