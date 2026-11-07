// Package health_utils runs a set of named health checks concurrently
// and reports aggregate pass or fail status.
package health_utils

import (
	"context"
	"errors"
	"sync"
)

// Check is a function that reports whether a dependency is healthy.
// It returns nil when healthy, or a non-nil error describing the problem.
type Check func(ctx context.Context) error

// Result is the outcome of a single named health check.
type Result struct {
	Name string
	Err  error
}

// Report is the aggregate outcome of running a set of checks.
type Report struct {
	// Results holds one entry per check, in the order the checks were
	// registered. The order is stable so callers can render deterministic
	// output even though the checks themselves run concurrently.
	Results []Result
}

// Healthy reports whether every check passed. An empty report is healthy:
// there is nothing to fail.
func (r Report) Healthy() bool {
	for _, res := range r.Results {
		if res.Err != nil {
			return false
		}
	}
	return true
}

// Failed returns the names of the checks that failed, in registration order.
func (r Report) Failed() []string {
	var failed []string
	for _, res := range r.Results {
		if res.Err != nil {
			failed = append(failed, res.Name)
		}
	}
	return failed
}

// Checker is a collection of named health checks.
type Checker struct {
	names  []string
	checks []Check
}

// NewChecker returns an empty Checker.
func NewChecker() *Checker {
	return &Checker{}
}

// Add registers a check under a name. Names are not required to be unique;
// results are reported per registration. A nil check is treated as always
// healthy, which keeps callers from having to guard optional dependencies.
func (c *Checker) Add(name string, check Check) {
	c.names = append(c.names, name)
	c.checks = append(c.checks, check)
}

// Run executes all registered checks concurrently and returns an aggregate
// Report. Results are ordered by registration, not completion, so the report
// is deterministic even though execution is not.
//
// If ctx is cancelled, checks that observe the context are expected to return;
// Run itself does not abandon the wait for them. This keeps the report complete
// rather than partial, which is what most health endpoints want.
func (c *Checker) Run(ctx context.Context) Report {
	errs := make([]error, len(c.checks))
	var wg sync.WaitGroup
	wg.Add(len(c.checks))

	for i, check := range c.checks {
		i, check := i, check
		go func() {
			defer wg.Done()
			if check == nil {
				return
			}
			errs[i] = check(ctx)
		}()
	}

	wg.Wait()

	report := Report{Results: make([]Result, len(c.checks))}
	for i, name := range c.names {
		report.Results[i] = Result{Name: name, Err: errs[i]}
	}
	return report
}

// ErrEmptyName is returned by AddChecked when the supplied name is empty.
var ErrEmptyName = errors.New("health_utils: check name must not be empty")

// AddChecked is Add with validation. It rejects empty names so a misconfigured
// check cannot silently disappear into a report with a blank label.
func (c *Checker) AddChecked(name string, check Check) error {
	if name == "" {
		return ErrEmptyName
	}
	c.Add(name, check)
	return nil
}
