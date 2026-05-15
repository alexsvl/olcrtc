package supervisor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openlibrecommunity/olcrtc/internal/app/session"
)

var errRunnerBoom = errors.New("boom")

func TestRunRequiresProfiles(t *testing.T) {
	err := Run(context.Background(), Config{}, func(context.Context, session.Config) error { return nil })
	if !errors.Is(err, ErrNoProfiles) {
		t.Fatalf("Run() error = %v, want %v", err, ErrNoProfiles)
	}
}

func TestRunAdvancesProfilesAndStopsAtMaxCycles(t *testing.T) {
	profiles := []Profile{
		{Name: "first", Config: session.Config{Auth: "wbstream"}},
		{Name: "second", Config: session.Config{Auth: "jitsi"}},
	}
	var started []string
	var ended []string
	err := Run(context.Background(), Config{
		Profiles:   profiles,
		RetryDelay: -1,
		MaxCycles:  1,
		OnProfileStart: func(profile Profile, cycle int) {
			started = append(started, profile.Name)
			if cycle != 1 {
				t.Fatalf("cycle = %d, want 1", cycle)
			}
		},
		OnProfileEnd: func(profile Profile, _ int, err error) {
			ended = append(ended, profile.Name)
			if !errors.Is(err, errRunnerBoom) {
				t.Fatalf("profile %s err = %v, want %v", profile.Name, err, errRunnerBoom)
			}
		},
	}, func(_ context.Context, cfg session.Config) error {
		if cfg.Auth == "" {
			t.Fatal("runner received empty auth")
		}
		return errRunnerBoom
	})
	if !errors.Is(err, ErrMaxCyclesExceeded) {
		t.Fatalf("Run() error = %v, want %v", err, ErrMaxCyclesExceeded)
	}
	if got, want := started, []string{"first", "second"}; !equalStrings(got, want) {
		t.Fatalf("started = %v, want %v", got, want)
	}
	if got, want := ended, []string{"first", "second"}; !equalStrings(got, want) {
		t.Fatalf("ended = %v, want %v", got, want)
	}
}

func TestRunReturnsNilOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	err := Run(ctx, Config{
		Profiles:   []Profile{{Name: "one"}},
		RetryDelay: time.Hour,
	}, func(context.Context, session.Config) error {
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
