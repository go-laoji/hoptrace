package server

import (
	"context"
	"time"

	"hoptrace/internal/biz"

	"github.com/go-kratos/kratos/v3/log"
)

// Poller 实现 kratos transport.Server, 随应用生命周期启停后台周期轮询。
type Poller struct {
	poll *biz.PollUsecase
	jobs biz.JobRepo
	stop chan struct{}
	done chan struct{}
}

func NewPoller(poll *biz.PollUsecase, jobs biz.JobRepo) *Poller {
	return &Poller{
		poll: poll,
		jobs: jobs,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (p *Poller) Start(ctx context.Context) error {
	defer close(p.done)
	if err := p.jobs.FailRunning(ctx); err != nil {
		log.Warn("cleanup stale discovery jobs failed", "err", err)
	}
	opts := p.poll.Options()
	if opts.DisablePoller {
		log.Info("background poller disabled")
		<-p.stop
		return nil
	}

	tick := opts.PollInterval / 4
	if tick < 15*time.Second {
		tick = 15 * time.Second
	}
	if tick > time.Minute {
		tick = time.Minute
	}
	log.Info("background poller started", "poll_interval", opts.PollInterval, "check_every", tick)

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-p.stop:
			return nil
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(context.Background(), opts.PollInterval)
			n, err := p.poll.RunDue(runCtx)
			cancel()
			if err != nil {
				log.Warn("poller run failed", "err", err)
			} else if n > 0 {
				log.Info("poller cycle completed", "polled_devices", n)
			}
		}
	}
}

func (p *Poller) Stop(ctx context.Context) error {
	select {
	case <-p.stop:
	default:
		close(p.stop)
	}
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
