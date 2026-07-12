// Command agent-smith runs the platform maturity audit agent HTTP server.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tazzledazzle/build-agent-smith/internal/agent"
	"github.com/tazzledazzle/build-agent-smith/internal/api"
	"github.com/tazzledazzle/build-agent-smith/internal/config"
	"github.com/tazzledazzle/build-agent-smith/internal/demo"
	"github.com/tazzledazzle/build-agent-smith/internal/domain"
	"github.com/tazzledazzle/build-agent-smith/internal/output"
	"github.com/tazzledazzle/build-agent-smith/internal/store"
)

type logNotifier struct{}

func (logNotifier) PostDigest(_ context.Context, text string) error {
	log.Printf("slack digest:\n%s", text)
	return nil
}

// agentRunner adapts agent.Agent to api.Runner and persists results.
type agentRunner struct {
	agent  *agent.Agent
	writer *output.Writer
}

func (r *agentRunner) Run(ctx context.Context, scope domain.Scope, repos []domain.RepoConfig, target string) (*domain.AuditState, error) {
	state, err := r.agent.Run(ctx, agent.RunRequest{
		Scope:          scope,
		Repos:          repos,
		TargetRepoName: target,
	})
	if err != nil {
		return nil, err
	}
	if err := r.writer.Write(ctx, state); err != nil {
		return nil, err
	}
	return state, nil
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	manifestPath := flag.String("manifest", "configs/repos.json", "path to repo manifest")
	flag.Parse()

	manifest, err := config.LoadManifest(*manifestPath)
	if err != nil {
		log.Fatalf("manifest: %v", err)
	}

	mem := &store.Memory{}
	runner := &agentRunner{
		agent:  agent.New(demo.Sources{}),
		writer: output.New(mem, logNotifier{}),
	}

	handler := api.NewHandler(runner, manifest.Repos)
	server := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("audit agent listening on %s (%d repos)", *addr, len(manifest.Repos))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
