// Package main is always allowed to print — CLIs are expected to.
package main

import "fmt"

func main() {
	fmt.Println("hello")
	fmt.Printf("world\n")
}
