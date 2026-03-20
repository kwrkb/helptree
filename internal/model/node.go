package model

// Node represents a command or subcommand in the help tree.
type Node struct {
	Name        string
	Description string
	Usage       string
	Options     []Option
	Children    []*Node
	Loaded      bool // whether children have been loaded
	Expanded    bool
}

// Option represents a command-line flag or option.
type Option struct {
	Short       string // e.g. "-v"
	Long        string // e.g. "--verbose"
	Arg         string // e.g. "string", "int" (empty if boolean flag)
	Description string
}

// FullFlag returns the display string for an option (e.g. "-v, --verbose string").
func (o Option) FullFlag() string {
	var s string
	switch {
	case o.Short != "" && o.Long != "":
		s = o.Short + ", " + o.Long
	case o.Long != "":
		s = "    " + o.Long
	case o.Short != "":
		s = o.Short
	}
	if o.Arg != "" {
		s += " " + o.Arg
	}
	return s
}
