// Package allowed demonstrates the //clicky:allow-stdout file-level opt-out.
// Nothing here should be flagged even though it contains fmt.Println.
//
//clicky:allow-stdout — this migration script intentionally prints
package allowed

import "fmt"

func Hack() {
	fmt.Println("one-off debug output")
	fmt.Printf("count=%d\n", 42)
}
