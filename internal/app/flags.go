package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"slices"
)

// ErrUsage is wrapped by ParseFlags for any mistake on the command line.
// ExitCode turns it into ExitUsage.
var ErrUsage = errors.New("app: bad command line")

// Options are the choices given on the command line. Each one that's set
// answers its menu question, so that screen is skipped; an empty one is
// asked as usual.
type Options struct {
	// Mode is "match", "single", or "" to ask.
	Mode string
	// Opponent is "computer", "human", or "" to ask.
	Opponent string
	// Role is "breaker", "maker" or "random" (a coin flip) for the first
	// player in a Single game, or "" to ask.
	Role string
	// Seed fixes every random choice, so a game can be replayed.
	Seed uint64
	// NoColor turns colors off.
	NoColor bool
	// ASCII draws with ASCII characters only.
	ASCII bool
}

// The values each flag accepts. "" (the flag not given) is allowed too.
var (
	modes     = []string{"match", "single"}
	opponents = []string{"computer", "human"}
	roles     = []string{"breaker", "maker", "random"}
)

// ParseFlags reads the command-line arguments (without the program name)
// into Options. Mistakes are reported on stderr, followed by the usage
// text, and returned wrapped in ErrUsage. Asking for help with -h or -help
// prints the usage and returns flag.ErrHelp.
//
// Without -seed, a random seed is chosen, so every game is different.
//
// Go note: flag.NewFlagSet makes a private set of flags instead of using
// the package-level flag.String and friends, which all share one global
// set. With ContinueOnError, Parse returns an error instead of exiting
// the program, so this function can be tested like any other.
func ParseFlags(args []string, stderr io.Writer) (Options, error) {
	var o Options
	fs := flag.NewFlagSet("codebreaker", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.Mode, "mode", "", "the game: match or single (default: ask)")
	fs.StringVar(&o.Opponent, "opponent", "", "who to play: computer or human (default: ask)")
	fs.StringVar(&o.Role, "role", "", "your role in a single game: breaker, maker or random (default: ask)")
	fs.Uint64Var(&o.Seed, "seed", 0, "seed for all random choices, to replay a game (default: random)")
	fs.BoolVar(&o.NoColor, "no-color", false, "turn off colors")
	fs.BoolVar(&o.ASCII, "ascii", false, "draw with ASCII characters only")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), "Usage: codebreaker [flags]\n\n"+
			"Play Code Breaker in the terminal. Each flag answers a menu question,\n"+
			"which is then skipped.\n\nFlags:\n")
		fs.PrintDefaults()
	}

	// usage reports a problem found after parsing, the same way the flag
	// package reports its own: the message, then the usage text.
	usage := func(format string, a ...any) (Options, error) {
		msg := fmt.Sprintf(format, a...)
		fmt.Fprintln(fs.Output(), msg)
		fs.Usage()
		return Options{}, fmt.Errorf("%w: %s", ErrUsage, msg)
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return Options{}, err
		}
		// The flag package has already printed the message and usage.
		return Options{}, fmt.Errorf("%w: %v", ErrUsage, err)
	}
	if fs.NArg() > 0 {
		return usage("unexpected argument %q", fs.Arg(0))
	}

	checks := []struct {
		name, value string
		allowed     []string
	}{
		{"mode", o.Mode, modes},
		{"opponent", o.Opponent, opponents},
		{"role", o.Role, roles},
	}
	for _, c := range checks {
		// Go note: slices.Contains reports whether a value is in a slice.
		if c.value != "" && !slices.Contains(c.allowed, c.value) {
			return usage("invalid value %q for -%s: use one of %v", c.value, c.name, c.allowed)
		}
	}
	if o.Role != "" {
		if o.Mode == "match" {
			return usage("-role only applies to a single game; in a match, a coin flip picks the first Maker")
		}
		o.Mode = "single" // choosing a role implies a Single game
	}

	// Go note: fs.Visit calls the function for each flag that was actually
	// set on the command line. That's the way to tell "-seed 0" apart from
	// no -seed at all, since both leave o.Seed at 0.
	seedSet := false
	fs.Visit(func(f *flag.Flag) { seedSet = seedSet || f.Name == "seed" })
	if !seedSet {
		o.Seed = rand.Uint64()
	}
	return o, nil
}
