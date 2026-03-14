package cli

import (
	"fmt"
	"strings"
)

// ParsedArgs holds parsed command-line arguments
type ParsedArgs struct {
	Command    string
	Subcommand string
	Flags      map[string]string
	Args       []string
}

// ParseArgs parses command-line arguments into a structured format.
// It extracts the command, subcommand (if any), flags, and positional arguments.
//
// Format expected: [command] [subcommand] [flags...] [args...]
// Flags can be: --key=value, --key value, -k=value, -k value, --bool-flag, -b
func ParseArgs(args []string) (*ParsedArgs, error) {
	result := &ParsedArgs{
		Flags: make(map[string]string),
		Args:  make([]string, 0),
	}

	if len(args) == 0 {
		return result, nil
	}

	i := 0

	// First non-flag argument is the command
	for i < len(args) {
		arg := args[i]

		// Stop at first non-flag
		if !strings.HasPrefix(arg, "-") {
			result.Command = arg
			i++
			break
		}

		// Skip global flags at this level
		if strings.HasPrefix(arg, "--") {
			if strings.Contains(arg, "=") {
				i++
			} else {
				// Check if next arg is a value
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i += 2
				} else {
					i++
				}
			}
		} else if strings.HasPrefix(arg, "-") {
			// Short flag
			if strings.Contains(arg, "=") {
				i++
			} else {
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i += 2
				} else {
					i++
				}
			}
		}
	}

	// Second non-flag argument could be a subcommand
	if i < len(args) && !strings.HasPrefix(args[i], "-") {
		result.Subcommand = args[i]
		i++
	}

	// Parse remaining flags and args
	for i < len(args) {
		arg := args[i]

		if strings.HasPrefix(arg, "--") {
			// Long flag
			if idx := strings.Index(arg, "="); idx != -1 {
				// --key=value format
				key := arg[2:idx]
				value := arg[idx+1:]
				result.Flags[key] = value
				i++
			} else {
				// --key value format or --bool-flag
				key := arg[2:]
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					result.Flags[key] = args[i+1]
					i += 2
				} else {
					// Boolean flag
					result.Flags[key] = "true"
					i++
				}
			}
		} else if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			// Short flag
			if idx := strings.Index(arg, "="); idx != -1 {
				// -k=value format
				key := arg[1:idx]
				value := arg[idx+1:]
				result.Flags[key] = value
				i++
			} else {
				// -k value format or -bool
				key := arg[1:]
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					result.Flags[key] = args[i+1]
					i += 2
				} else {
					// Boolean flag
					result.Flags[key] = "true"
					i++
				}
			}
		} else {
			// Positional argument
			result.Args = append(result.Args, arg)
			i++
		}
	}

	return result, nil
}

// GlobalFlags are available for all commands
type GlobalFlags struct {
	Help    bool
	Version bool
	Format  string // json or table
}

// ParseGlobalFlags extracts global flags from the argument list.
// It returns the parsed global flags, the remaining arguments (after removing
// global flags), and any error encountered.
func ParseGlobalFlags(args []string) (*GlobalFlags, []string, error) {
	globals := &GlobalFlags{
		Format: "table", // default
	}

	var remaining []string
	i := 0

	for i < len(args) {
		arg := args[i]

		// Check for help flags
		if arg == "-h" || arg == "--help" {
			globals.Help = true
			i++
			continue
		}

		// Check for version flags
		if arg == "-v" || arg == "--version" {
			globals.Version = true
			i++
			continue
		}

		// Check for format flag
		if strings.HasPrefix(arg, "--format=") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				globals.Format = parts[1]
				if globals.Format != "json" && globals.Format != "table" {
					return nil, nil, fmt.Errorf("invalid format: %s (must be 'json' or 'table')", globals.Format)
				}
			}
			i++
			continue
		}

		if arg == "--format" {
			if i+1 < len(args) {
				globals.Format = args[i+1]
				if globals.Format != "json" && globals.Format != "table" {
					return nil, nil, fmt.Errorf("invalid format: %s (must be 'json' or 'table')", globals.Format)
				}
				i += 2
			} else {
				return nil, nil, fmt.Errorf("--format requires a value")
			}
			continue
		}

		// Check for format short flag
		if strings.HasPrefix(arg, "-f=") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				globals.Format = parts[1]
				if globals.Format != "json" && globals.Format != "table" {
					return nil, nil, fmt.Errorf("invalid format: %s (must be 'json' or 'table')", globals.Format)
				}
			}
			i++
			continue
		}

		if arg == "-f" {
			if i+1 < len(args) {
				globals.Format = args[i+1]
				if globals.Format != "json" && globals.Format != "table" {
					return nil, nil, fmt.Errorf("invalid format: %s (must be 'json' or 'table')", globals.Format)
				}
				i += 2
			} else {
				return nil, nil, fmt.Errorf("-f requires a value")
			}
			continue
		}

		// Not a global flag, add to remaining
		remaining = append(remaining, arg)
		i++
	}

	return globals, remaining, nil
}

// HasFlag checks if a flag exists in the parsed arguments
func (p *ParsedArgs) HasFlag(name string) bool {
	_, ok := p.Flags[name]
	return ok
}

// GetFlag returns a flag value and a boolean indicating if it exists
func (p *ParsedArgs) GetFlag(name string) (string, bool) {
	val, ok := p.Flags[name]
	return val, ok
}

// GetFlagDefault returns a flag value or a default if not present
func (p *ParsedArgs) GetFlagDefault(name, defaultValue string) string {
	if val, ok := p.Flags[name]; ok {
		return val
	}
	return defaultValue
}
