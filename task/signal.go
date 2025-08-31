package task

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// DisableSignalHandling disables automatic signal handling
func (tm *Manager) DisableSignalHandling() {
	tm.signalMu.Lock()
	defer tm.signalMu.Unlock()

	if tm.signalRegistered && tm.signalChan != nil {
		signal.Stop(tm.signalChan)
		close(tm.signalChan)
		tm.signalRegistered = false
	}
}

// registerSignalHandling sets up signal handling for graceful shutdown
func (tm *Manager) registerSignalHandling() {
	tm.signalMu.Lock()
	defer tm.signalMu.Unlock()

	if tm.signalRegistered {
		return // Already registered
	}

	tm.signalChan = make(chan os.Signal, 2) // Buffer for 2 signals (graceful + hard)
	signal.Notify(tm.signalChan, os.Interrupt, syscall.SIGTERM)
	tm.signalRegistered = true

	// Start signal handler goroutine
	go tm.handleSignals()
}

// handleSignals processes incoming signals for graceful and hard shutdown
func (tm *Manager) handleSignals() {
	signalCount := 0
	var gracefulShutdownDone chan bool

	for sig := range tm.signalChan {
		signalCount++

		switch signalCount {
		case 1:
			// First signal: initiate graceful shutdown
			gracefulShutdownDone = make(chan bool, 1)
			go tm.gracefulShutdown(sig, gracefulShutdownDone)

			// Set up a timer for the second signal or timeout (hard exit)
			go func() {
				select {
				case <-gracefulShutdownDone:
					// Graceful shutdown completed successfully
					return
				case <-time.After(tm.gracefulTimeout):
					// Timeout reached, proceed with hard exit
					tm.hardExit("timeout")
				case nextSig := <-tm.signalChan:
					// Second signal received
					signalCount++
					if signalCount == 2 {
						fmt.Fprintf(os.Stderr, "\n🛑 Received second signal %v - forcing exit with goroutine dump\n", nextSig)
						tm.forceExitWithStack()
					} else {
						// Third or more signals - panic immediately
						fmt.Fprintf(os.Stderr, "\n☠️ Received signal #%d - PANIC EXIT\n", signalCount)
						tm.panicExit()
					}
				}
			}()

		case 2:
			// Second signal: force exit with stack trace
			fmt.Fprintf(os.Stderr, "\n🛑 Received second signal %v - forcing exit with goroutine dump\n", sig)
			tm.forceExitWithStack()

		default:
			// Third or more signals: panic immediately
			fmt.Fprintf(os.Stderr, "\n☠️ Received signal #%d (%v) - PANIC EXIT\n", signalCount, sig)
			tm.panicExit()
		}
	}
}

// gracefulShutdown initiates graceful shutdown process
func (tm *Manager) gracefulShutdown(sig os.Signal, gracefulDone chan bool) {
	tm.shutdownOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "\n🛑 Received %v - initiating graceful shutdown...\n", sig)
		fmt.Fprintf(os.Stderr, "   Press Ctrl+C again to force immediate exit\n\n")
		fmt.Fprint(os.Stderr, Debug())

		// Call user-defined interrupt handler if provided
		if tm.onInterrupt != nil {
			tm.onInterrupt()
		}

		// Cancel all running tasks
		CancelAll()

		// Wait for tasks to complete with a shorter internal timeout
		done := make(chan bool, 1)
		go func() {
			tm.wg.Wait()
			done <- true
		}()

		select {
		case <-done:
			// All tasks completed gracefully
			fmt.Fprintf(os.Stderr, "✅ All tasks completed gracefully\n")
			gracefulDone <- true
			os.Exit(0)

		case <-time.After(tm.gracefulTimeout):
			// Timeout reached
			fmt.Fprintf(os.Stderr, "⏰ Graceful shutdown timeout reached\n")
			fmt.Fprint(os.Stderr, tm.Pretty().String())
			gracefulDone <- true
			os.Exit(1)
		}
	})
}

// hardExit performs immediate forced exit
func (tm *Manager) hardExit(reason string) {
	fmt.Fprintf(os.Stderr, "\n💥 Force exit (%s) - terminating immediately\n", reason)

	fmt.Fprint(os.Stderr, tm.Pretty().String())
	// Cancel all tasks immediately (best effort)
	CancelAll()

	os.Exit(130) // Standard exit code for interrupted process
}

// forceExitWithStack performs forced exit with goroutine stack dump
func (tm *Manager) forceExitWithStack() {
	fmt.Fprintf(os.Stderr, "\n💥 Force exit - dumping goroutine stacks...\n")
	fmt.Fprintf(os.Stderr, "=====================================\n")

	// Cancel all tasks immediately
	CancelAll()

	// Get goroutine count first
	numGoroutines := runtime.NumGoroutine()
	fmt.Fprintf(os.Stderr, "Number of goroutines: %d\n", numGoroutines)
	fmt.Fprintf(os.Stderr, "-------------------------------------\n")

	// Print all goroutine stacks
	buf := make([]byte, 1<<20)           // 1MB buffer for stack traces
	stackLen := runtime.Stack(buf, true) // true = all goroutines
	fmt.Fprintf(os.Stderr, "%s\n", buf[:stackLen])

	fmt.Fprintf(os.Stderr, "=====================================\n")
	fmt.Fprintf(os.Stderr, "Waiting 1 second before exit...\n")

	// Wait 1 second for stack dump to be visible
	time.Sleep(1 * time.Second)

	fmt.Fprintf(os.Stderr, "Forcing exit now...\n")
	os.Exit(130)
}

// panicExit performs immediate panic to force exit
func (tm *Manager) panicExit() {
	fmt.Fprintf(os.Stderr, "\n☠️ PANIC EXIT - Multiple interrupts received!\n")
	fmt.Fprintf(os.Stderr, "Forcing immediate panic with full stack trace...\n")

	// This will generate a panic with full stack traces for all goroutines
	panic("FORCE EXIT: Process interrupted multiple times - emergency termination")
}
