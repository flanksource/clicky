package cobra

// Command is a minimal stand-in for spf13/cobra.Command, carrying just the
// fields the clickylint entity-registration checks inspect.
type Command struct {
	Use   string
	Short string
	Long  string
	Run   func(cmd *Command, args []string)
	RunE  func(cmd *Command, args []string) error
}

func (c *Command) AddCommand(cmds ...*Command) {}
