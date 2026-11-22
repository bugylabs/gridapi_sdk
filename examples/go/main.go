// Runnable GridAPI Go SDK example. Uses environment variables only:
//
//	export GRIDAPI_API_KEY=gp_live_...       # account key, sent as Bearer
//	export GRIDAPI_PROJECT_KEY=gapi_live_... # local scope guard, never transmitted
//	export GRIDAPI_BASE_URL=https://api.gridapi.in   # optional
//	export GRIDAPI_TIMEOUT_MS=30000                  # optional
//	go run ./examples/go
//
// Both keys are required. Data methods call Verify automatically: the account
// is checked once and the project slug is checked against the account's
// authorized projects. Keep both keys server-side.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	gridapi "github.com/bugylabs/gridapi_sdk/packages/go"
)

func main() {
	client, err := gridapi.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	me, err := client.GetMe(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("authenticated: %v (scope: %s)\n", me.Authenticated, me.Account.Scope)

	projects, err := client.ListProjects(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, p := range projects {
		fmt.Printf("project %s (%s)\n", p.Slug, p.Role)
	}

	if len(os.Args) < 2 {
		fmt.Println("usage: main.go <project> <table>")
		return
	}
	project := os.Args[1]
	table := "users"
	if len(os.Args) > 2 {
		table = os.Args[2]
	}

	// Data methods verify automatically; call Verify for an early scope check.
	if err := client.Verify(ctx, project); err != nil {
		log.Fatal(err)
	}

	users, err := client.Get(ctx, project, table, &gridapi.QueryOptions{
		Filter: map[string]string{"status": "active"},
		Limit:  10,
		Page:   1,
	})
	if err != nil {
		var quota *gridapi.QuotaExceededError
		var auth *gridapi.AuthenticationError
		switch {
		case errors.As(err, &quota):
			log.Fatalf("quota exhausted: %v", err)
		case errors.As(err, &auth):
			log.Fatalf("check GRIDAPI_API_KEY / GRIDAPI_PROJECT_KEY: %v", err)
		default:
			log.Fatal(err)
		}
	}
	fmt.Printf("fetched %d records (total %d)\n", len(users.Data), users.Meta.Total)
	for _, row := range users.Data {
		fmt.Printf("  %v\t%v\n", row["id"], row["name"])
	}

	total, err := client.Count(ctx, project, table, "status:active")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("active count: %d\n", total)
}
