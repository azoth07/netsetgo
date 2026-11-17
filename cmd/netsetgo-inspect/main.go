package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"netsetgo/internal/network"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	adapters, err := network.ListAdapters(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(adapters); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
