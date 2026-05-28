// Example: embed dex as a library. Constructs an engine, lists the bundled
// marketplace, fetches the manifest for a built-in plugin, and prints its
// declared operations.
package main

import (
	"context"
	"fmt"
	"log"

	dex "github.com/fluxplane/fluxplane-dex"
)

func main() {
	engine, err := dex.New(dex.Config{})
	if err != nil {
		log.Fatalf("dex.New: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	fmt.Println("Marketplace plugins:")
	for _, p := range engine.Plugins().All(ctx) {
		fmt.Printf("  - %s: %s\n", p.Name, p.Description)
	}

	manifest, err := engine.Manifest(ctx, "websearch")
	if err != nil {
		log.Fatalf("manifest: %v", err)
	}
	fmt.Printf("\nOperations declared by %s (%s):\n", manifest.Name, manifest.Description)
	for _, op := range manifest.Operations {
		fmt.Printf("  - %s: %s\n", op.Name, op.Description)
	}
}
