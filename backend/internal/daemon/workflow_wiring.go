package daemon

import (
	"context"
	"log/slog"
	"time"

	workflowsvc "github.com/aoagents/agent-orchestrator/backend/internal/service/workflow"
)

func startWorkflows(ctx context.Context, service *workflowsvc.Service, log *slog.Logger) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			if err := service.Tick(ctx, time.Now()); err != nil && ctx.Err() == nil {
				log.Warn("workflow tick", "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return done
}
