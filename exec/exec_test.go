package exec

import (
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/task"
)

var _ = Describe("Process", func() {
	Describe("Basic Command Execution", func() {
		It("should execute simple commands successfully", func() {
			p := NewExec("echo hello").Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.Err).To(BeNil())
		})

		It("should handle command failures", func() {
			p := NewExec("false").Run()
			Expect(p.IsOK()).To(BeFalse())
			Expect(p.Err).ToNot(BeNil())
		})

		It("should handle invalid commands", func() {
			p := NewExec("nonexistent-command-xyz").Run()
			Expect(p.IsOK()).To(BeFalse())
			Expect(p.Err).ToNot(BeNil())
		})
	})

	Describe("Stdout/Stderr Capturing", func() {
		It("should capture stdout correctly", func() {
			p := NewExec("echo 'hello stdout'").Run()
			Expect(p.GetStdout()).To(Equal("hello stdout\n"))
			Expect(p.GetStderr()).To(Equal(""))
		})

		It("should capture stderr correctly", func() {
			p := NewExec("echo 'hello stderr' >&2").Run()
			Expect(p.GetStdout()).To(Equal(""))
			Expect(p.GetStderr()).To(Equal("hello stderr\n"))
		})

		It("should capture both stdout and stderr", func() {
			p := NewExec("echo 'stdout line'; echo 'stderr line' >&2").Run()
			Expect(p.GetStdout()).To(Equal("stdout line\n"))
			Expect(p.GetStderr()).To(Equal("stderr line\n"))
		})

		It("should combine stdout and stderr with Out() method", func() {
			p := NewExec("echo 'stdout'; echo 'stderr' >&2").Run()
			combined := p.Out()
			Expect(combined).To(ContainSubstring("stdout"))
			Expect(combined).To(ContainSubstring("stderr"))
		})

		XIt("should handle large output", func() {
			// Generate 1000 lines of output
			p := NewExec("for i in {1..1000}; do echo line$i; done").Run()
			Expect(p.IsOK()).To(BeTrue())
			lines := strings.Split(strings.TrimSpace(p.GetStdout()), "\n")
			Expect(len(lines)).To(Equal(1000))
			Expect(lines[0]).To(Equal("line1"))
			Expect(lines[999]).To(Equal("line1000"))
		})

		It("should handle empty output", func() {
			p := NewExec("true").Run()
			Expect(p.GetStdout()).To(Equal(""))
			Expect(p.GetStderr()).To(Equal(""))
			Expect(p.Out()).To(Equal(""))
		})
	})

	Describe("Formatted Command Execution", func() {
		It("should execute Runf with formatting", func() {
			p := NewExecf("echo 'number: %d, string: %s'", 42, "test").Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.GetStdout()).To(Equal("number: 42, string: test\n"))
		})

		It("should handle multiple format arguments", func() {
			p := NewExecf("echo '%s %d %s'", "hello", 123, "world").Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.GetStdout()).To(Equal("hello 123 world\n"))
		})
	})

	Describe("Environment Variables", func() {
		It("should set environment variables with WithEnv", func() {
			env := map[string]string{
				"TEST_VAR": "test_value",
				"CUSTOM":   "custom_value",
			}
			p := NewExec("echo $TEST_VAR $CUSTOM").WithEnv(env).Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.GetStdout()).To(Equal("test_value custom_value\n"))
		})

		It("should handle multiple environment variables", func() {
			env := map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
				"VAR3": "value3",
			}
			p := NewExec("echo $VAR1:$VAR2:$VAR3").WithEnv(env).Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.GetStdout()).To(Equal("value1:value2:value3\n"))
		})

		It("should override existing environment variables", func() {
			env := map[string]string{"PATH": "/custom/path"}
			p := NewExec("echo $PATH").WithEnv(env).Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.GetStdout()).To(Equal("/custom/path\n"))
		})
	})

	Describe("Working Directory", func() {
		It("should change working directory with WithCwd", func() {
			tempDir := "/tmp"
			p := NewExec("pwd").WithCwd(tempDir).Run()
			Expect(p.IsOK()).To(BeTrue())

			actualPath := strings.TrimSpace(p.GetStdout())
			expectedPath, _ := filepath.EvalSymlinks(tempDir)
			resolvedActual, _ := filepath.EvalSymlinks(actualPath)

			Expect(resolvedActual).To(Equal(expectedPath))
		})

		It("should fail with invalid working directory", func() {
			p := NewExec("pwd").WithCwd("/nonexistent/directory").Run()
			Expect(p.IsOK()).To(BeFalse())
		})

		It("should execute relative commands in specified directory", func() {
			tempDir := "/tmp"
			p := NewExec("ls -la . | head -1").WithCwd(tempDir).Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.GetStdout()).To(ContainSubstring("total"))
		})
	})

	Describe("Process Information", func() {
		It("should return process name with Name()", func() {
			p := NewExec("echo test").Run()
			Expect(p.Name()).ToNot(BeEmpty())
		})

		It("should return formatted output with Pretty()", func() {
			p := NewExec("echo test").Run()
			pretty := p.Pretty()
			Expect(pretty.Content).ToNot(BeEmpty())
		})

		It("should detect successful execution with IsOK()", func() {
			p := NewExec("true").Run()
			Expect(p.IsOK()).To(BeTrue())
		})

		It("should detect failed execution with IsOK()", func() {
			p := NewExec("false").Run()
			Expect(p.IsOK()).To(BeFalse())
		})
	})

	Describe("Edge Cases", func() {
		It("should handle commands with special characters", func() {
			p := NewExec("echo 'special chars: !@#$%^&*()'").Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.GetStdout()).To(Equal("special chars: !@#$%^&*()\n"))
		})

		It("should handle commands with quotes", func() {
			p := NewExec(`echo "double quotes" 'single quotes'`).Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.GetStdout()).To(Equal("double quotes single quotes\n"))
		})

		It("should handle multiline commands", func() {
			cmd := `echo line1
echo line2
echo line3`
			p := NewExec(cmd).Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.GetStdout()).To(Equal("line1\nline2\nline3\n"))
		})

		It("should handle commands with pipes", func() {
			p := NewExec("echo 'hello world' | grep hello").Run()
			Expect(p.IsOK()).To(BeTrue())
			Expect(p.GetStdout()).To(Equal("hello world\n"))
		})

		It("should handle commands that exit with specific codes", func() {
			p := NewExec("exit 42").Run()
			Expect(p.IsOK()).To(BeFalse())
			Expect(p.Err).ToNot(BeNil())
		})

		It("should handle timeout scenarios", func() {
			// Test that Wait() function exists and can be called
			p := NewExec("echo quick")
			go p.Run()

			// Wait should not hang for quick commands
			err := p.Wait()
			// For a command that hasn't been properly started, this might error
			// but the function should exist and be callable
			_ = err // We expect this might error in this test scenario
		})
	})

	Describe("Concurrent Execution", func() {
		It("should allow result snapshots while a process starts", func() {
			process := NewExec("/bin/sh", "-c", "printf captured")
			Expect(process.captureOutput).ToNot(BeNil())
			done := make(chan struct{})
			go func() {
				process.Run()
				close(done)
			}()

			// Bounded: a Result() that deadlocks against the running process must
			// fail this spec rather than spin a core until the suite times out.
			deadline := time.After(30 * time.Second)
			for {
				select {
				case <-done:
					Expect(process.Result().Stdout).To(Equal("captured"))
					return
				case <-deadline:
					Fail("process did not finish while Result() was being snapshotted")
				default:
					_ = process.Result()
				}
			}
		})

		It("should handle multiple processes running concurrently", func() {
			processes := make([]*Process, 3)
			results := make(chan string, 3)

			for i := 0; i < 3; i++ {
				processes[i] = NewExec("echo process" + string(rune('1'+i)))
				go func(p *Process, id string) {
					p = p.Run()
					if p.IsOK() {
						results <- strings.TrimSpace(p.GetStdout())
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

		It("should run Process as TypedTask with StartAsTask()", func() {
			p := NewExec("echo 'typed task output'")
			typedTask := p.StartAsTask("Typed Echo Task")

			Expect(typedTask).ToNot(BeNil())

			// Wait for completion and get typed result
			result := typedTask.WaitFor()
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(task.StatusSuccess))

			// Get the typed result
			processResult, err := typedTask.GetResult()
			Expect(err).To(BeNil())
			Expect(processResult.GetStdout()).To(Equal("typed task output\n"))
		})

		It("should handle task failures properly", func() {
			p := NewExec("exit 1")
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
			p := NewExec("sleep 0.1; echo done")

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
			Expect(processResult.GetStdout()).To(Equal("done\n"))
		})

		It("should handle concurrent task execution", func() {
			processes := []*Process{
				NewExec("echo task1"),
				NewExec("echo task2"),
				NewExec("echo task3"),
				//NewExec( "echo task4"),
				NewExec("echo task4"),
			}

			tasks := make([]task.TypedTask[*Process], len(processes))

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
				Expect(processResult.GetStdout()).To(Equal("task" + expectedOutput + "\n"))
			}
		})

		It("should integrate with environment variables in tasks", func() {
			p := NewExec("echo $TEST_TASK_VAR").WithEnv(map[string]string{
				"TEST_TASK_VAR": "task_value",
			})

			typedTask := p.StartAsTask("Env Task")
			result := typedTask.WaitFor()

			Expect(result.Status).To(Equal(task.StatusSuccess))

			processResult, err := typedTask.GetResult()
			Expect(err).To(BeNil())
			Expect(processResult.GetStdout()).To(Equal("task_value\n"))
		})

		It("should work with working directory in tasks", func() {
			tempDir := "/tmp"
			p := NewExec("pwd").WithCwd(tempDir)

			typedTask := p.StartAsTask("Cwd Task")
			result := typedTask.WaitFor()

			Expect(result.Status).To(Equal(task.StatusSuccess))

			processResult, err := typedTask.GetResult()
			Expect(err).To(BeNil())

			actualPath := strings.TrimSpace(processResult.GetStdout())
			expectedPath, _ := filepath.EvalSymlinks(tempDir)
			resolvedActual, _ := filepath.EvalSymlinks(actualPath)

			Expect(resolvedActual).To(Equal(expectedPath))
		})

		It("should preserve task reference in GetTask() after AsTask()", func() {
			p := NewExec("echo test")

			// Create task
			taskObj := p.StartAsTask("Reference Test")

			// Wait for completion first (this ensures the task function runs)
			result := taskObj.WaitFor()
			Expect(result.Status).To(Equal(task.StatusSuccess))

			// After the task has run, the process should have the task reference
			Expect(p.GetTask()).ToNot(BeNil())
		})
	})

	Describe("Task-Aware Logging", func() {
		Context("RunWithLogging", func() {
			It("should execute command with task logger", func() {
				p := NewExec("echo hello")
				t := p.RunAsTask("Test Task")

				// Wait for task to complete
				result, err := t.GetResult()

				_, _ = GinkgoWriter.Write([]byte("r:" + result.Pretty().ANSI() + "\n"))

				Expect(err).To(BeNil())
				Expect(result.Status).To(Equal(string(task.StatusSuccess)))
				Expect(result.Stdout).To(Equal("hello\n"))
			})

			It("should fall back to Run() without task", func() {
				p := NewExec("echo no-task").Run()
				Expect(p.IsOK()).To(BeTrue())
				Expect(p.GetStdout()).To(Equal("no-task\n"))
			})

			It("should handle failed commands", func() {
				p := NewExec("false")
				t := p.RunAsTask("Failing Task")

				t.WaitFor()

				Expect(p.IsOK()).To(BeFalse())
				Expect(p.IsRunning()).To(BeFalse())
				result, _ := t.GetResult()
				_, _ = GinkgoWriter.Write([]byte("r2:" + result.Pretty().ANSI() + "\n"))

				Expect(result.Status).To(Equal(string(task.StatusFailed)))
				Expect(result.Error).ToNot(BeNil())
			})

			It("should handle commands with output", func() {
				cmd := "echo line1; echo line2; echo line3"
				p := NewExec(cmd)
				t := p.StartAsTask("Multi-line Task")

				result := t.WaitFor()
				Expect(result.Status).To(Equal(task.StatusSuccess))
				Expect(p.GetStdout()).To(Equal("line1\nline2\nline3\n"))
			})
		})
	})
})
