package task

import "sync"

type ansiTerminalOwner string

const (
	ansiTerminalOwnerNone         ansiTerminalOwner = ""
	ansiTerminalOwnerTaskRenderer ansiTerminalOwner = "task-renderer"
	ansiTerminalOwnerPrompt       ansiTerminalOwner = "prompt"
)

type ansiTerminalController struct {
	mu            sync.Mutex
	cond          *sync.Cond
	owner         ansiTerminalOwner
	manager       *Manager
	promptWaiters int
}

func newANSITerminalController() *ansiTerminalController {
	controller := &ansiTerminalController{}
	controller.cond = sync.NewCond(&controller.mu)
	return controller
}

var globalANSITerminal = newANSITerminalController()

func (c *ansiTerminalController) tryAcquireTaskRenderer(manager *Manager) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.owner != ansiTerminalOwnerNone || c.promptWaiters > 0 {
		return false
	}

	c.owner = ansiTerminalOwnerTaskRenderer
	c.manager = manager
	return true
}

func (c *ansiTerminalController) beginPromptAcquire() *Manager {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.promptWaiters++
	if c.owner == ansiTerminalOwnerTaskRenderer {
		return c.manager
	}

	return nil
}

func (c *ansiTerminalController) cancelPromptAcquire() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.promptWaiters == 0 {
		return
	}

	c.promptWaiters--
	if c.promptWaiters == 0 {
		c.cond.Broadcast()
	}
}

func (c *ansiTerminalController) acquirePrompt() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for c.owner != ansiTerminalOwnerNone {
		c.cond.Wait()
	}

	c.owner = ansiTerminalOwnerPrompt
	c.manager = nil
	if c.promptWaiters > 0 {
		c.promptWaiters--
	}
}

func (c *ansiTerminalController) releaseTaskRenderer(manager *Manager) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.owner != ansiTerminalOwnerTaskRenderer || c.manager != manager {
		return
	}

	c.owner = ansiTerminalOwnerNone
	c.manager = nil
	c.cond.Broadcast()
}

func (c *ansiTerminalController) releasePrompt() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.owner != ansiTerminalOwnerPrompt {
		return
	}

	c.owner = ansiTerminalOwnerNone
	c.manager = nil
	c.cond.Broadcast()
}

// AcquirePromptTerminal waits for exclusive ANSI terminal ownership for prompt
// rendering, stopping the active task renderer first when necessary.
// The second return value reports whether the prompt displaced an active task renderer.
func AcquirePromptTerminal() (func(), bool) {
	manager := globalANSITerminal.beginPromptAcquire()
	acquired := false
	defer func() {
		if !acquired {
			globalANSITerminal.cancelPromptAcquire()
		}
	}()

	if manager != nil {
		manager.stopRender()
	}

	globalANSITerminal.acquirePrompt()
	acquired = true

	return func() {
		globalANSITerminal.releasePrompt()
	}, manager != nil
}
