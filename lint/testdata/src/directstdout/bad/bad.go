// Package bad contains patterns that should trip the direct-stdout rule.
package bad

import (
	"fmt"
	"os"
)

func UsesFmtPrintln() {
	fmt.Println("hello") // want `avoid fmt\.Println`
}

func UsesFmtPrintf() {
	fmt.Printf("n=%d\n", 1) // want `avoid fmt\.Printf`
}

func UsesFmtPrint() {
	fmt.Print("raw") // want `avoid fmt\.Print`
}

func UsesFprintlnStdout() {
	fmt.Fprintln(os.Stdout, "hi") // want `avoid fmt\.Fprintln writing to os\.Stdout`
}

func UsesFprintfStderr() {
	fmt.Fprintf(os.Stderr, "boom: %v\n", "x") // want `avoid fmt\.Fprintf writing to os\.Stderr`
}

func UsesStdoutWrite() {
	_, _ = os.Stdout.Write([]byte("hi")) // want `avoid direct os\.Stdout\.Write`
}

func UsesStderrWriteString() {
	_, _ = os.Stderr.WriteString("err") // want `avoid direct os\.Stderr\.WriteString`
}
