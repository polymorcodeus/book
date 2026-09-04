package cmd

import "fmt"

// requireFlag returns an error if value is empty, naming the missing flag.
func requireFlag(name, value string) error {
	if value == "" {
		return fmt.Errorf("missing required flag: --%s", name)
	}
	return nil
}

// requireFlags returns an error listing any empty flag values. Pairs must be
// provided as alternating flag name and value strings.
func requireFlags(pairs ...string) error {
	if len(pairs)%2 != 0 {
		panic("requireFlags expects alternating name/value pairs")
	}

	var missing []string
	for i := 0; i < len(pairs); i += 2 {
		if pairs[i+1] == "" {
			missing = append(missing, "--"+pairs[i])
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required flags: %s", missing)
	}
	return nil
}
