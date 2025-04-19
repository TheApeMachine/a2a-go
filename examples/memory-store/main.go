// memory-store is a **self‑contained** demonstration of the lightweight
// in‑memory long‑term memory façade that ships with the a2a‑go SDK.  It does
// not rely on external services – everything happens in process.
//
//   go run ./examples/memory-store

package main

import (
	"encoding/json"
	"fmt"

	"github.com/theapemachine/a2a-go/memory"
)

func main() {
	store := memory.New()

	// ------------------------------------------------------------------
	// 1. Store two snippets of text (one in each backend)
	// ------------------------------------------------------------------
	id1 := store.Put("vector", "Hello, world!", nil)
	fmt.Printf("Stored doc1 in vector backend → %s\n", id1)

	id2 := store.Put("graph", "Go makes concurrency easy.", map[string]any{"author": "gopher"})
	fmt.Printf("Stored doc2 in graph  backend → %s\n", id2)

	// ------------------------------------------------------------------
	// 2. Retrieve by ID
	// ------------------------------------------------------------------
	if doc, ok := store.Get(id2); ok {
		b, _ := json.Marshal(doc)
		fmt.Printf("Fetched doc2 → %s\n", string(b))
	}

	// ------------------------------------------------------------------
	// 3. Simple search
	// ------------------------------------------------------------------
	hits := store.Search("hello", "vector", 0)
	fmt.Printf("Search for 'hello' in vector backend returned: %v\n", hits)
}
