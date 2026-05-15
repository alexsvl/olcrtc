// Package supervisor runs ordered session profiles with failover.
package supervisor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openlibrecommunity/olcrtc/internal/app/session"
)

const DefaultRetryDelay = 2 * time.Second

var (
	// ErrNoProfiles is returned when the supervisor is started without profiles.
	ErrNoProfiles = errors.New("supervisor: no profiles configured")
	// ErrMaxCyclesExceeded is returned after MaxCycles complete profile-list passes.
	ErrMaxCyclesExceeded = errors.New("supervisor: max failover cycles exceeded")
)

// Profile is one runnable session configuration in an ordered failover list.
type Profile struct {
	Name   string
	Config session.Config
}

// Runner starts one session profile and blocks until it ends or fails.
type Runner func(ctx context.Context, cfg session.Config) error

// Config controls ordered failover behavior.
type Config struct {
	Profiles   []Profile
	RetryDelay time.Duration
	MaxCycles  int

	OnProfileStart func(profile Profile, cycle int)
	OnProfileEnd   func(profile Profile, cycle int, err error)
}

// Run starts profiles in order. If a profile exits while ctx is still active,
// the supervisor waits RetryDelay and advances to the next profile.
func Run(ctx context.Context, cfg Config, run Runner) error {
	if len(cfg.Profiles) == 0 {
		return ErrNoProfiles
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = DefaultRetryDelay
	}

	var lastErr error
	for cycle := 1; ; cycle++ {
		for i, profile := range cfg.Profiles {
			if ctx.Err() != nil {
				return nil
			}
			if cfg.OnProfileStart != nil {
				cfg.OnProfileStart(profile, cycle)
			}

			err := run(ctx, profile.Config)
			if ctx.Err() != nil {
				return nil
			}
			if err != nil {
				lastErr = fmt.Errorf("profile %q: %w", profile.Name, err)
			} else {
				lastErr = fmt.Errorf("profile %q ended", profile.Name)
			}
			if cfg.OnProfileEnd != nil {
				cfg.OnProfileEnd(profile, cycle, err)
			}

			if cfg.MaxCycles > 0 && cycle >= cfg.MaxCycles && i == len(cfg.Profiles)-1 {
				return fmt.Errorf("%w after %d cycle(s): %w", ErrMaxCyclesExceeded, cycle, lastErr)
			}
			if err := waitRetryDelay(ctx, cfg.RetryDelay); err != nil {
				return nil
			}
		}
	}
}

func waitRetryDelay(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
