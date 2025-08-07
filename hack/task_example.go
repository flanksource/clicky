package main

import (
	"fmt"
	"os"
	"time"
	
	"github.com/flanksource/clicky"
)

func main() {
	tm := clicky.NewTaskManager()

	task1 := tm.Start("Downloading files")
	go func() {
		for i := 0; i <= 100; i += 5 {
			task1.SetProgress(i, 100)
			time.Sleep(50 * time.Millisecond)
		}
		task1.Success()
	}()

	task2 := tm.Start("Processing data")
	go func() {
		time.Sleep(200 * time.Millisecond)
		for i := 0; i <= 75; i += 15 {
			task2.SetProgress(i, 75)
			time.Sleep(100 * time.Millisecond)
		}
		task2.Failed()
	}()

	task3 := tm.Start("Analyzing results")
	go func() {
		time.Sleep(300 * time.Millisecond)
		for i := 0; i <= 50; i += 10 {
			task3.SetProgress(i, 50)
			time.Sleep(80 * time.Millisecond)
		}
		task3.Warning()
	}()

	task4 := tm.Start("Optimizing")
	go func() {
		time.Sleep(400 * time.Millisecond)
		for i := 0; i <= 30; i += 5 {
			task4.SetProgress(i, 30)
			time.Sleep(60 * time.Millisecond)
		}
		task4.Success()
	}()

	task5 := tm.Start("Generating report")
	go func() {
		time.Sleep(600 * time.Millisecond)
		task5.SetStatus("Compiling results...")
		time.Sleep(500 * time.Millisecond)
		task5.SetStatus("Writing output...")
		time.Sleep(300 * time.Millisecond)
		task5.Success()
	}()

	if len(os.Args) > 1 && os.Args[1] == "fatal" {
		task6 := tm.Start("Critical operation")
		go func() {
			time.Sleep(1 * time.Second)
			task6.Fatal(fmt.Errorf("critical system failure"))
		}()
	}

	exitCode := tm.Wait()
	os.Exit(exitCode)
}