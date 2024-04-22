package run

type Options struct {
	Graph   bool
	Help    bool
	NoCache bool
	NoColor bool
	Remote  bool
	Target  string
	Version bool
}

func RunCommand(cmd string, tasks []string, opts Options) error {
	return nil
}
