# health_utils

Runs a set of named health checks concurrently and reports aggregate pass or fail status.

```go
package main

import (
	"context"
	"errors"
	"fmt"

	health_utils "github.com/example/health_utils"
)

func main() {
	c := health_utils.NewChecker()
	c.Add("database", func(ctx context.Context) error {
		return nil // pretend we pinged the database
	})
	c.Add("cache", func(ctx context.Context) error {
		return errors.New("cache is unreachable")
	})

	report := c.Run(context.Background())
	fmt.Println(report.Healthy()) // false
	fmt.Println(report.Failed())  // [cache]
}
```

## Why this exists

A health endpoint usually needs to answer one question: is everything okay, and if not, what broke? Doing that well means running independent checks without letting one slow dependency serialise the rest, while still producing a stable report. The trade-off here is that `Run` waits for every check even after the context is cancelled, so the report is always complete rather than a partial snapshot; callers that need a deadline should make their checks observe the context themselves.

## The awkward edge

Names are not required to be unique. If you register two checks under `"db"` and one fails, `Failed` will contain `"db"` once, and `Results` will contain two entries with that name. The report is keyed by registration, not by name, so you can always tell how many checks ran. Use `AddChecked` if you want empty names rejected rather than silently registered.
