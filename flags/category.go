package flags

import "github.com/spf13/pflag"

// CategoryAnnotation stores the display category used by grouped CLI help.
const CategoryAnnotation = "clicky_category"

const outputFormatShortcutAnnotation = "clicky_output_format_shortcut"

const (
	CategoryModelSelection   = "Model selection"
	CategoryModelPermissions = "Model permissions"
	CategoryModelRuntime     = "Model runtime"
	CategoryFormatting       = "Formatting"
	CategoryLogging          = "Logging"
	CategoryTasks            = "Tasks"
)

// SetCategory assigns a display category to a registered flag.
func SetCategory(flag *pflag.Flag, category string) {
	if flag == nil {
		panic("cannot categorize an unregistered flag")
	}
	if category == "" {
		panic("flag category must not be empty")
	}
	if flag.Annotations == nil {
		flag.Annotations = make(map[string][]string)
	}
	flag.Annotations[CategoryAnnotation] = []string{category}
}

// Category returns a flag's category, or empty for an uncategorized flag.
func Category(flag *pflag.Flag) string {
	if categories := flag.Annotations[CategoryAnnotation]; len(categories) > 0 {
		return categories[0]
	}
	return ""
}

// MarkOutputFormatShortcut puts a flag in the Formatting section's shortcuts line.
func MarkOutputFormatShortcut(flag *pflag.Flag) {
	SetCategory(flag, CategoryFormatting)
	flag.Annotations[outputFormatShortcutAnnotation] = []string{"true"}
}

// IsOutputFormatShortcut reports whether grouped help should list the flag as a shortcut.
func IsOutputFormatShortcut(flag *pflag.Flag) bool {
	_, marked := flag.Annotations[outputFormatShortcutAnnotation]
	return marked
}
