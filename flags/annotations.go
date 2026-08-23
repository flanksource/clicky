package flags

import "github.com/spf13/pflag"

const allowedValuesAnnotation = "clicky/allowed-values"

// SetAllowedValues attaches the values accepted by a flag for schema consumers.
func SetAllowedValues(flag *pflag.Flag, values ...string) {
	if flag.Annotations == nil {
		flag.Annotations = make(map[string][]string)
	}
	flag.Annotations[allowedValuesAnnotation] = append([]string(nil), values...)
}

// AllowedValues returns the schema values attached by SetAllowedValues.
func AllowedValues(flag *pflag.Flag) []string {
	return append([]string(nil), flag.Annotations[allowedValuesAnnotation]...)
}
