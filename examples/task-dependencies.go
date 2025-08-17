package main

import (
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/spf13/pflag"
)

func main() {

	flags := clicky.BindAllFlags(pflag.CommandLine)
	showDependencyGraph := pflag.Bool("show-graph", true, "Show dependency graph")
	pflag.Parse()
	flags.UseFlags()

	// Create task manager
	tm := task.NewManagerWithOptions(&flags.TaskManagerOptions)

	fmt.Println("=== Task Dependencies & Groups Example ===\n")

	// Phase 1: Setup tasks (no dependencies)
	setupDatabase := tm.Start("Setup Database", task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
		t.Infof("Creating database schema")
		time.Sleep(500 * time.Millisecond)
		t.SetProgress(1, 2)
		time.Sleep(500 * time.Millisecond)
		t.SetProgress(2, 2)
		t.Success()
		return nil
	}))

	setupCache := tm.Start("Setup Cache", task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
		t.Infof("Initializing cache layer")
		time.Sleep(400 * time.Millisecond)
		t.Success()
		return nil
	}))

	loadConfig := tm.Start("Load Configuration", task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
		t.Infof("Reading configuration files")
		time.Sleep(300 * time.Millisecond)
		t.Success()
		return nil
	}))

	// Phase 2: Tasks that depend on setup
	migrateData := tm.Start("Migrate Data",
		task.WithDependencies(setupDatabase),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			t.Infof("Running database migrations")
			for i := 1; i <= 5; i++ {
				t.SetProgress(i, 5)
				t.Infof("Migration %d of 5", i)
				time.Sleep(200 * time.Millisecond)
			}
			t.Success()
			return nil
		}))

	initServices := tm.Start("Initialize Services",
		task.WithDependencies(loadConfig, setupCache),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			t.Infof("Starting application services")
			services := []string{"Auth", "API", "Worker", "Scheduler"}
			for i, service := range services {
				t.SetProgress(i+1, len(services))
				t.Infof("Starting %s service", service)
				time.Sleep(300 * time.Millisecond)
			}
			t.Success()
			return nil
		}))

	// Phase 3: Tasks that depend on phase 2
	seedData := tm.Start("Seed Test Data",
		task.WithDependencies(migrateData),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			t.Infof("Inserting test data")
			time.Sleep(600 * time.Millisecond)
			t.Success()
			return nil
		}))

	startAPI := tm.Start("Start API Server",
		task.WithDependencies(initServices, migrateData),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			t.Infof("Starting API server on port 8080")
			time.Sleep(500 * time.Millisecond)
			t.Success()
			t.Infof("API server ready")
			return nil
		}))

	// Phase 4: Final tasks
	healthCheck := tm.Start("Health Check",
		task.WithDependencies(startAPI, seedData),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			t.Infof("Running health checks")
			endpoints := []string{"/health", "/ready", "/metrics"}
			for i, endpoint := range endpoints {
				t.SetProgress(i+1, len(endpoints))
				t.Infof("Checking %s", endpoint)
				time.Sleep(200 * time.Millisecond)
			}
			t.Success()
			return nil
		}))

	// Create a task group for parallel operations
	fmt.Println("\n--- Creating Task Group for parallel data processing ---")

	dataGroup := task.StartGroup[interface{}]("Data Processing Group")

	// Add tasks to the group
	for i := 1; i <= 4; i++ {
		region := fmt.Sprintf("Region-%d", i)
		dataGroup.Add(fmt.Sprintf("Process %s", region),
			func(ctx flanksourceContext.Context, t *task.Task) (interface{}, error) {
				t.Infof("Processing data for %s", region)

				// Simulate data processing
				records := 100
				for j := 0; j < records; j += 20 {
					t.SetProgress(j, records)
					time.Sleep(100 * time.Millisecond)
				}
				t.SetProgress(records, records)

				t.Success()
				t.Infof("Processed %d records", records)
				return nil, nil
			})
	}

	// Create a task that depends on the entire group
	// Get tasks from the group as slice
	groupTasks := make([]*task.Task, 0)
	for _, item := range dataGroup.GetTasks() {
		if t, ok := item.(*task.Task); ok {
			groupTasks = append(groupTasks, t)
		}
	}
	aggregateResults := tm.Start("Aggregate Results",
		task.WithDependencies(groupTasks...),
		task.WithFunc(func(ctx flanksourceContext.Context, t *task.Task) error {
			t.Infof("Aggregating results from all regions")
			time.Sleep(500 * time.Millisecond)
			t.Success()
			t.Infof("Aggregation complete")
			return nil
		}))

	// Display dependency graph if requested
	if *showDependencyGraph {
		fmt.Println("\n=== Dependency Graph ===")
		fmt.Println("Legend: → means 'depends on'")
		fmt.Println()

		dependencies := []struct {
			Task string
			Deps []string
		}{
			{"Migrate Data", []string{"Setup Database"}},
			{"Initialize Services", []string{"Load Configuration", "Setup Cache"}},
			{"Seed Test Data", []string{"Migrate Data"}},
			{"Start API Server", []string{"Initialize Services", "Migrate Data"}},
			{"Health Check", []string{"Start API Server", "Seed Test Data"}},
			{"Aggregate Results", []string{"Data Processing Group (4 tasks)"}},
		}

		for _, dep := range dependencies {
			if len(dep.Deps) > 0 {
				fmt.Printf("%s → %v\n", dep.Task, dep.Deps)
			}
		}
	}

	// Wait for all tasks
	fmt.Println("\n--- Executing tasks respecting dependencies ---\n")
	exitCode := tm.Wait()

	// Create execution timeline
	timeline := struct {
		Title string
		Tasks []struct {
			Name      string
			Status    string
			Duration  string
			DependsOn []string
		}
	}{
		Title: "Execution Timeline",
		Tasks: []struct {
			Name      string
			Status    string
			Duration  string
			DependsOn []string
		}{},
	}

	// Collect task information
	allTasks := []*task.Task{
		setupDatabase, setupCache, loadConfig,
		migrateData, initServices,
		seedData, startAPI,
		healthCheck, aggregateResults,
	}

	for _, t := range allTasks {
		// Dependencies field is not exported, use empty slice
		var deps []string
		// We could track dependencies manually based on our setup above
		// but for now just leave empty

		taskInfo := struct {
			Name      string
			Status    string
			Duration  string
			DependsOn []string
		}{
			Name:      t.Name(),
			Status:    string(t.Status()),
			Duration:  t.Duration().String(),
			DependsOn: deps,
		}
		timeline.Tasks = append(timeline.Tasks, taskInfo)
	}

	// Add group tasks
	for _, gt := range dataGroup.GetTasks() {
		t := gt.GetTask()
		timeline.Tasks = append(timeline.Tasks, struct {
			Name      string
			Status    string
			Duration  string
			DependsOn []string
		}{
			Name:     t.Name(),
			Status:   string(t.Status()),
			Duration: t.Duration().String(),
		})
	}

	// Display timeline
	fmt.Println("\n=== Execution Results ===")
	output, err := clicky.Format(timeline)
	if err != nil {
		// Fallback display
		for _, t := range timeline.Tasks {
			status := "✓"
			if t.Status == string(task.StatusFailed) {
				status = "✗"
			} else if t.Status == string(task.StatusWarning) {
				status = "⚠"
			}
			fmt.Printf("%s %s (%s)", status, t.Name, t.Duration)
			if len(t.DependsOn) > 0 {
				fmt.Printf(" - waited for: %v", t.DependsOn)
			}
			fmt.Println()
		}
	} else {
		fmt.Println(output)
	}

	// Group results
	fmt.Printf("\n=== Group Status ===\n")
	fmt.Printf("Data Processing Group: %s\n", dataGroup.Status())
	fmt.Printf("Group Duration: %s\n", dataGroup.Duration())
	fmt.Printf("Tasks in group: %d\n", len(dataGroup.GetTasks()))

	os.Exit(exitCode)
}
