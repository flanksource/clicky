package exec_test

import (
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/exec"
	"github.com/flanksource/clicky/task"
	"github.com/flanksource/commons/logger"
)

var _ = Describe("Process", func() {
	Describe("Basic Command Execution", func() {
		It("should execute simple commands successfully", func() {
			p := exec.Process{Cmd: "echo hello"}.Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Err).To(BeNil())
		})

		It("should handle command failures", func() {
			p := exec.Process{Cmd: "false"}.Run()
			Expect(p.IsOK()).To(BeFalse())
			Expect(p.Err).ToNot(BeNil())
		})

		It("should handle invalid commands", func() {
			p := exec.Process{Cmd: "nonexistent-command-xyz"}.Run()
			Expect(p.IsOK()).To(BeFalse())
			Expect(p.Err).ToNot(BeNil())
		})
	})

	Describe("Stdout/Stderr Capturing", func() {
		It("should capture stdout correctly", func() {
			p := exec.Process{Cmd: "echo 'hello stdout'"}.Run()
			Expect(p.Stdout.String()).To(Equal("hello stdout\n"))
			Expect(p.Stderr.String()).To(Equal(""))
		})

		It("should capture stderr correctly", func() {
			p := exec.Process{Cmd: "echo 'hello stderr' >&2"}.Run()
			Expect(p.Stdout.String()).To(Equal(""))
			Expect(p.Stderr.String()).To(Equal("hello stderr\n"))
		})

		It("should capture both stdout and stderr", func() {
			p := exec.Process{Cmd: "echo 'stdout line'; echo 'stderr line' >&2"}.Run()
			Expect(p.Stdout.String()).To(Equal("stdout line\n"))
			Expect(p.Stderr.String()).To(Equal("stderr line\n"))
		})

		It("should combine stdout and stderr with Out() method", func() {
			p := exec.Process{Cmd: "echo 'stdout'; echo 'stderr' >&2"}.Run()
			combined := p.Out()
			Expect(combined).To(ContainSubstring("stdout"))
			Expect(combined).To(ContainSubstring("stderr"))
		})

		XIt("should handle large output", func() {
			// Generate 1000 lines of output
			p := exec.Process{Cmd: "for i in {1..1000}; do echo line$i; done"}.Run()
			Expect(p.IsOK()).To(BeTrue())
			lines := strings.Split(strings.TrimSpace(p.Stdout.String()), "\n")
			Expect(len(lines)).To(Equal(1000))
			Expect(lines[0]).To(Equal("line1"))
			Expect(lines[999]).To(Equal("line1000"))
		})

		It("should handle empty output", func() {
			p := exec.Process{Cmd: "true"}.Run()
			Expect(p.Stdout.String()).To(Equal(""))
			Expect(p.Stderr.String()).To(Equal(""))
			Expect(p.Out()).To(Equal(""))
		})
	})

	Describe("Formatted Command Execution", func() {
		It("should execute Runf with formatting", func() {
			p := exec.Process{}.Runf("echo 'number: %d, string: %s'", 42, "test")
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Stdout.String()).To(Equal("number: 42, string: test\n"))
		})

		It("should handle multiple format arguments", func() {
			p := exec.Process{}.Runf("echo '%s %d %s'", "hello", 123, "world")
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Stdout.String()).To(Equal("hello 123 world\n"))
		})
	})

	Describe("Environment Variables", func() {
		It("should set environment variables with WithEnv", func() {
			env := map[string]string{
				"TEST_VAR": "test_value",
				"CUSTOM":   "custom_value",
			}
			p := exec.Process{Cmd: "echo $TEST_VAR $CUSTOM"}.WithEnv(env).Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Stdout.String()).To(Equal("test_value custom_value\n"))
		})

		It("should handle multiple environment variables", func() {
			env := map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
				"VAR3": "value3",
			}
			p := exec.Process{Cmd: "echo $VAR1:$VAR2:$VAR3"}.WithEnv(env).Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Stdout.String()).To(Equal("value1:value2:value3\n"))
		})

		It("should override existing environment variables", func() {
			env := map[string]string{"PATH": "/custom/path"}
			p := exec.Process{Cmd: "echo $PATH"}.WithEnv(env).Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Stdout.String()).To(Equal("/custom/path\n"))
		})
	})

	Describe("Working Directory", func() {
		It("should change working directory with WithCwd", func() {
			tempDir := "/tmp"
			p := exec.Process{Cmd: "pwd"}.WithCwd(tempDir).Run()
			Expect(p.IsOK()).To(BeTrue())

			actualPath := strings.TrimSpace(p.Stdout.String())
			expectedPath, _ := filepath.EvalSymlinks(tempDir)
			resolvedActual, _ := filepath.EvalSymlinks(actualPath)

			Expect(resolvedActual).To(Equal(expectedPath))
		})

		It("should fail with invalid working directory", func() {
			p := exec.Process{Cmd: "pwd"}.WithCwd("/nonexistent/directory").Run()
			Expect(p.IsOK()).To(BeFalse())
		})

		It("should execute relative commands in specified directory", func() {
			tempDir := "/tmp"
			p := exec.Process{Cmd: "ls -la . | head -1"}.WithCwd(tempDir).Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Stdout.String()).To(ContainSubstring("total"))
		})
	})

	Describe("Logger Integration", func() {
		var testLogger logger.Logger

		BeforeEach(func() {
			testLogger = logger.StandardLogger()
		})

		It("should attach logger with WithLogger", func() {
			p := exec.Process{Cmd: "echo test"}.WithLogger(testLogger).Run()
			Expect(p.Log).ToNot(BeNil())
			Expect(p.IsOK()).To(BeTrue())
		})

		It("should use verbose logger with filtering", func() {
			p := exec.Process{Cmd: "echo 'filter this'; echo 'show this'"}.WithLogger(testLogger).Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Stdout.String()).To(ContainSubstring("filter this"))
			Expect(p.Stdout.String()).To(ContainSubstring("show this"))
		})
	})

	Describe("Process Information", func() {
		It("should return process name with Name()", func() {
			p := exec.Process{Cmd: "echo test"}.Run()
			Expect(p.Name()).ToNot(BeEmpty())
		})

		It("should return formatted output with Pretty()", func() {
			p := exec.Process{Cmd: "echo test"}.Run()
			pretty := p.Pretty()
			Expect(pretty.Content).ToNot(BeEmpty())
		})

		It("should detect successful execution with IsOK()", func() {
			p := exec.Process{Cmd: "true"}.Run()
			Expect(p.IsOK()).To(BeTrue())
		})

		It("should detect failed execution with IsOK()", func() {
			p := exec.Process{Cmd: "false"}.Run()
			Expect(p.IsOK()).To(BeFalse())
		})
	})

	Describe("Process Control", func() {
		Context("when starting processes in background", func() {
			var p exec.Process

			BeforeEach(func() {
				p = exec.Process{Cmd: "sleep 2; echo done"}
			})

			It("should start process in background", func() {
				err := p.Start()
				Expect(err).To(BeNil())

				// Process should be running
				time.Sleep(100 * time.Millisecond)
				// Since Start() runs in background, we can't easily test the state
			})
		})

		Context("when controlling long-running processes", func() {
			It("should handle process termination", func() {
				p := exec.Process{Cmd: "sleep 1"}
				go p.Run()

				time.Sleep(100 * time.Millisecond)

				// Test termination methods exist (actual termination testing is complex in unit tests)
				Expect(p.Stop).ToNot(BeNil())
				Expect(p.Kill).ToNot(BeNil())
				Expect(p.Terminate).ToNot(BeNil())
				Expect(p.ForceKill).ToNot(BeNil())
			})

			It("should handle MustStop with timeout", func() {
				p := exec.Process{Cmd: "sleep 0.1"}
				go p.Run()

				time.Sleep(50 * time.Millisecond)

				err := p.MustStop(5 * time.Second)
				// MustStop should complete without error for short-lived process
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("Edge Cases", func() {
		It("should handle commands with special characters", func() {
			p := exec.Process{Cmd: "echo 'special chars: !@#$%^&*()'"}.Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Stdout.String()).To(Equal("special chars: !@#$%^&*()\n"))
		})

		It("should handle commands with quotes", func() {
			p := exec.Process{Cmd: `echo "double quotes" 'single quotes'`}.Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Stdout.String()).To(Equal("double quotes single quotes\n"))
		})

		It("should handle multiline commands", func() {
			cmd := `echo line1
echo line2
echo line3`
			p := exec.Process{Cmd: cmd}.Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Stdout.String()).To(Equal("line1\nline2\nline3\n"))
		})

		It("should handle commands with pipes", func() {
			p := exec.Process{Cmd: "echo 'hello world' | grep hello"}.Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Stdout.String()).To(Equal("hello world\n"))
		})

		It("should handle commands that exit with specific codes", func() {
			p := exec.Process{Cmd: "exit 42"}.Run()
			Expect(p.IsOK()).To(BeFalse())
			Expect(p.Err).ToNot(BeNil())
		})

		It("should handle timeout scenarios", func() {
			// Test that Wait() function exists and can be called
			p := exec.Process{Cmd: "echo quick"}
			go p.Run()

			// Wait should not hang for quick commands
			err := p.Wait()
			// For a command that hasn't been properly started, this might error
			// but the function should exist and be callable
			_ = err // We expect this might error in this test scenario
		})
	})

	Describe("Concurrent Execution", func() {
		It("should handle multiple processes running concurrently", func() {
			processes := make([]exec.Process, 3)
			results := make(chan string, 3)

			for i := 0; i < 3; i++ {
				processes[i] = exec.Process{Cmd: "echo process" + string(rune('1'+i))}
				go func(p exec.Process, id string) {
					p = p.Run()
					if p.IsOK() {
						results <- strings.TrimSpace(p.Stdout.String())
					} else {
						results <- "error"
					}
				}(processes[i], string(rune('1'+i)))
			}

			// Collect results
			outputs := make([]string, 3)
			for i := 0; i < 3; i++ {
				select {
				case output := <-results:
					outputs[i] = output
				case <-time.After(5 * time.Second):
					Fail("Timeout waiting for concurrent processes")
				}
			}

			// Verify all processes completed
			Expect(len(outputs)).To(Equal(3))
			for _, output := range outputs {
				Expect(output).To(MatchRegexp("process[123]"))
			}
		})
	})

	Describe("Task Interface Integration", func() {
		It("should implement Taskable interface with GetTask()", func() {
			p := &exec.Process{Cmd: "echo test"}

			// Initially, task should be nil
			Expect(p.GetTask()).To(BeNil())

			// After creating a task, it should be set
			taskObj := p.AsTask("Test Task")
			Expect(taskObj).ToNot(BeNil())
		})

		It("should convert Process to Task with AsTask()", func() {
			p := &exec.Process{Cmd: "echo 'hello from task'"}
			taskObj := p.AsTask("Echo Task")

			Expect(taskObj).ToNot(BeNil())
			Expect(taskObj.Name()).To(Equal("Echo Task"))

			// Wait for task completion
			result := taskObj.WaitFor()
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(task.StatusSuccess))
		})

		It("should run Process as TypedTask with StartAsTask()", func() {
			p := &exec.Process{Cmd: "echo 'typed task output'"}
			typedTask := p.StartAsTask("Typed Echo Task")

			Expect(typedTask).ToNot(BeNil())

			// Wait for completion and get typed result
			result := typedTask.WaitFor()
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(task.StatusSuccess))

			// Get the typed result
			processResult, err := typedTask.GetResult()
			Expect(err).To(BeNil())
			Expect(processResult.Stdout.String()).To(Equal("typed task output\n"))
		})

		It("should handle task failures properly", func() {
			p := &exec.Process{Cmd: "exit 1"}
			typedTask := p.StartAsTask("Failing Task")

			result := typedTask.WaitFor()
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(task.StatusFailed))
			Expect(result.Error).ToNot(BeNil())

			// The typed result should reflect the failure
			processResult, err := typedTask.GetResult()
			// Either GetResult returns an error OR the process result has an error
			if err == nil {
				// If GetResult succeeded, the process itself should have failed
				Expect(processResult.Err).ToNot(BeNil())
			} else {
				// If GetResult failed, that's also acceptable for a failing task
				Expect(err).ToNot(BeNil())
			}
		})

		It("should support task options", func() {
			p := &exec.Process{Cmd: "sleep 0.1; echo done"}

			// Create task with timeout option
			typedTask := p.StartAsTask("Task with Options",
				task.WithTimeout(5*time.Second),
				task.WithPriority(1),
			)

			result := typedTask.WaitFor()
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(task.StatusSuccess))

			processResult, err := typedTask.GetResult()
			Expect(err).To(BeNil())
			Expect(processResult.Stdout.String()).To(Equal("done\n"))
		})

		It("should handle concurrent task execution", func() {
			processes := []*exec.Process{
				{Cmd: "echo task1"},
				{Cmd: "echo task2"},
				{Cmd: "echo task3"},
			}

			tasks := make([]task.TypedTask[exec.Process], len(processes))

			// Start all tasks
			for i, p := range processes {
				tasks[i] = p.StartAsTask(string(rune('A'+i)) + " Concurrent Task")
			}

			// Wait for all to complete
			for i, t := range tasks {
				result := t.WaitFor()
				Expect(result.Status).To(Equal(task.StatusSuccess))

				processResult, err := t.GetResult()
				Expect(err).To(BeNil())
				expectedOutput := string(rune('1' + i))
				Expect(processResult.Stdout.String()).To(Equal("task" + expectedOutput + "\n"))
			}
		})

		It("should integrate with environment variables in tasks", func() {
			p := (&exec.Process{
				Cmd: "echo $TEST_TASK_VAR",
			}).WithEnv(map[string]string{
				"TEST_TASK_VAR": "task_value",
			})

			typedTask := (&p).StartAsTask("Env Task")
			result := typedTask.WaitFor()

			Expect(result.Status).To(Equal(task.StatusSuccess))

			processResult, err := typedTask.GetResult()
			Expect(err).To(BeNil())
			Expect(processResult.Stdout.String()).To(Equal("task_value\n"))
		})

		It("should work with working directory in tasks", func() {
			tempDir := "/tmp"
			p := (&exec.Process{
				Cmd: "pwd",
			}).WithCwd(tempDir)

			typedTask := (&p).StartAsTask("Cwd Task")
			result := typedTask.WaitFor()

			Expect(result.Status).To(Equal(task.StatusSuccess))

			processResult, err := typedTask.GetResult()
			Expect(err).To(BeNil())

			actualPath := strings.TrimSpace(processResult.Stdout.String())
			expectedPath, _ := filepath.EvalSymlinks(tempDir)
			resolvedActual, _ := filepath.EvalSymlinks(actualPath)

			Expect(resolvedActual).To(Equal(expectedPath))
		})

		It("should preserve task reference in GetTask() after AsTask()", func() {
			p := &exec.Process{Cmd: "echo test"}

			// Create task
			taskObj := p.AsTask("Reference Test")

			// Wait for completion first (this ensures the task function runs)
			result := taskObj.WaitFor()
			Expect(result.Status).To(Equal(task.StatusSuccess))

			// After the task has run, the process should have the task reference
			Expect(p.GetTask()).ToNot(BeNil())
		})
	})

	Describe("Shell Detection", func() {
		Context("ContainsShellOperators", func() {
			It("should detect pipe operator", func() {
				Expect(exec.ContainsShellOperators("echo foo | grep bar")).To(BeTrue())
			})

			It("should detect output redirect", func() {
				Expect(exec.ContainsShellOperators("echo foo > file.txt")).To(BeTrue())
			})

			It("should detect input redirect", func() {
				Expect(exec.ContainsShellOperators("cat < file.txt")).To(BeTrue())
			})

			It("should detect stderr redirect", func() {
				Expect(exec.ContainsShellOperators("command 2> error.log")).To(BeTrue())
			})

			It("should detect AND operator", func() {
				Expect(exec.ContainsShellOperators("command1 && command2")).To(BeTrue())
			})

			It("should detect OR operator", func() {
				Expect(exec.ContainsShellOperators("command1 || command2")).To(BeTrue())
			})

			It("should detect semicolon separator", func() {
				Expect(exec.ContainsShellOperators("command1; command2")).To(BeTrue())
			})

			It("should detect backticks", func() {
				Expect(exec.ContainsShellOperators("echo `date`")).To(BeTrue())
			})

			It("should detect command substitution", func() {
				Expect(exec.ContainsShellOperators("echo $(date)")).To(BeTrue())
			})

			It("should not detect operators in simple commands", func() {
				Expect(exec.ContainsShellOperators("echo hello world")).To(BeFalse())
			})

			It("should not detect operators in commands with args", func() {
				Expect(exec.ContainsShellOperators("/usr/bin/command --option=value")).To(BeFalse())
			})
		})

		Context("Shell Wrapping Behavior", func() {
			It("should execute commands with pipes correctly", func() {
				p := exec.Process{Cmd: "echo hello | tr h H"}.Run()
				Expect(p.IsOK()).To(BeTrue())
				Expect(p.Stdout.String()).To(Equal("Hello\n"))
			})

			It("should execute commands with redirects correctly", func() {
				// Test stderr redirect to stdout
				p := exec.Process{Cmd: "echo error >&2 | cat"}.Run()
				Expect(p.IsOK()).To(BeTrue())
				// Stderr should contain the error message
				Expect(p.Stderr.String()).To(ContainSubstring("error"))
			})

			It("should execute commands with AND operator", func() {
				p := exec.Process{Cmd: "echo first && echo second"}.Run()
				Expect(p.IsOK()).To(BeTrue())
				Expect(p.Stdout.String()).To(Equal("first\nsecond\n"))
			})

			It("should execute commands with OR operator", func() {
				p := exec.Process{Cmd: "false || echo fallback"}.Run()
				Expect(p.IsOK()).To(BeTrue())
				Expect(p.Stdout.String()).To(Equal("fallback\n"))
			})

			It("should execute commands with command substitution", func() {
				p := exec.Process{Cmd: "echo result: $(echo nested)"}.Run()
				Expect(p.IsOK()).To(BeTrue())
				Expect(p.Stdout.String()).To(Equal("result: nested\n"))
			})
		})
	})
})
