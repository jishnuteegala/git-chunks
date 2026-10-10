package cli

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func pushWithRetry(ctx context.Context, repo, remote, branch string, retries int, logger *Logger, sleep func(context.Context, time.Duration) error) error {
	var err error
	for attempt := 0; ; attempt++ {
		if _, err = git(repo, "push", remote, "HEAD:refs/heads/"+branch); err == nil {
			return nil
		}
		if attempt >= retries {
			return fmt.Errorf("push failed after %d attempt(s): %w", attempt+1, err)
		}
		if !isTransientPushError(err) {
			return fmt.Errorf("push failed: %w", err)
		}
		delay := time.Duration(1<<attempt) * time.Second
		logger.Error("push failed (attempt %d/%d), retrying in %s: %v", attempt+1, retries+1, delay, err)
		if err := sleep(ctx, delay); err != nil {
			return err
		}
	}
}

func randomSpacing(r *rand.Rand, min, max time.Duration) time.Duration {
	if min == max {
		return min
	}
	return min + time.Duration(r.Int63n(int64(max-min)+1))
}

type etaEstimate struct {
	rate   float64
	weight float64
}

func (e *etaEstimate) add(duration time.Duration, bytes int64) {
	if bytes <= 0 {
		bytes = 1
	}
	weight := float64(bytes)
	sample := float64(duration) / weight
	if e.weight == 0 {
		e.rate, e.weight = sample, weight
		return
	}
	e.rate = (e.rate*e.weight*0.5 + sample*weight) / (e.weight*0.5 + weight)
	e.weight = e.weight*0.5 + weight
}

func (e etaEstimate) message(now time.Time, remaining [][]File, min, max time.Duration, spacing bool) string {
	if e.weight == 0 {
		return "ETA pending"
	}
	var bytes int64
	for _, chunk := range remaining {
		bytes += chunkSize(chunk)
	}
	eta := time.Duration(e.rate * float64(bytes))
	if spacing && len(remaining) > 0 {
		eta += (min + max) / 2 * time.Duration(len(remaining))
	}
	return fmt.Sprintf("ETA %s (completes ~%s)", eta.Round(time.Second), now.Add(eta).Format("15:04"))
}

func isTransientPushError(err error) bool {
	text := strings.ToLower(err.Error())
	for _, phrase := range []string{"timed out", "connection reset", "could not resolve host", "failed to connect", "remote end hung up", "unexpected disconnect", "temporarily unavailable", "http 5"} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}
