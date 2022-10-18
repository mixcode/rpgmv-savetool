// A tool to manipulate rpg maker mv savefiles
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	lzstring "github.com/mixcode/golib-lzstring"
)

//
// TODO: add localization
//

type config struct {
	force bool // force overwrite

	keepGap bool // keep gap between savefile IDs

	rawJson    bool // save raw json (if possible)
	prettyJson bool // save pretty formatted json

	useDefaultExt bool

	verbose bool // verbose mode

	setComment bool // set comments to modifying entries
	comment    string
}

var (

	// gloval config
	cfg = config{
		force:         false,
		keepGap:       false,
		rawJson:       false,
		prettyJson:    false,
		useDefaultExt: true,
		verbose:       true,
		setComment:    false,
	}

	// non-flag arguments
	args []string
)

func getArg(n int) string {
	if n < len(args) {
		return args[n]
	}
	return ""
}

// actual main
func run() (err error) {
	cmd := getArg(0)
	if cmd == "" {
		err = fmt.Errorf("no command given. valid commands are 'ls', 'cp', 'mv', 'rm'. use -h for help")
		return
	}

	switch cmd {

	case "ls": // list the contents of the archive
		a := args[1:]
		if len(a) == 0 {
			a = append(a, ".")
		}
		for i, p := range a {
			var ss *saveFileSelector
			ss, err = NewSaveFileSelector(p)
			if err != nil {
				return
			}
			if i > 0 {
				fmt.Println()
			}
			err = cmdLs(ss)
			if err != nil {
				return
			}
		}

	case "cp", "mv": // copy or move savefile between archives

		a := args[1:]
		if len(a) < 2 {
			err = fmt.Errorf("please set source filenames and a destination filename and/or %cid", idSeparator)
			return
		}

		// the last arg is the destination file
		destFile, a := a[len(a)-1], a[:len(a)-1]
		var destSS *saveFileSelector
		destSS, err = NewSaveFileSelector(destFile)
		if err != nil {
			return
		}

		// all other args are the source files
		srcSS := make([]*saveFileSelector, len(a))
		for i, s := range a {
			srcSS[i], err = NewSaveFileSelector(s)
			if err != nil {
				return
			}
		}
		if cmd == "cp" {
			err = cmdCp(srcSS, destSS)
		} else if cmd == "mv" {
			if len(srcSS) == 1 && destSS.Path == "" {
				// if the filename of dest path is omitted, then use the same filename with the source.
				destSS.Path = srcSS[0].Path
			}
			err = cmdMv(srcSS, destSS)
		}

	case "rm": // remove savefile
		a := args[1:]
		if len(a) == 0 {
			err = fmt.Errorf("please provide a filename and/or %cid", idSeparator)
			return
		}
		for _, p := range a {
			var ss *saveFileSelector
			ss, err = NewSaveFileSelector(p)
			if err != nil {
				return
			}
			err = cmdRm(ss)
			if err != nil {
				return
			}
		}

	case "d", "e": // "d" and "e" is hidden commands for decoding and encoding lzstring file
		src, dest := getArg(1), getArg(2)
		if src == "" {
			err = fmt.Errorf("must have source and dest filename")
			return
		}
		var data string
		if cmd == "d" { // decode
			data, err = readLzstringFile(src)
			if err != nil {
				return
			}
		} else if cmd == "e" { // encode
			var tmp []byte
			tmp, err = os.ReadFile(src)
			if err != nil {
				return
			}
			data = lzstring.CompressToBase64(string(tmp))
		}
		if dest == "" || dest == "-" {
			fmt.Printf("%s\n", data)
		} else {
			err = os.WriteFile(dest, []byte(data), 0644)
		}
		return
	default:
		err = fmt.Errorf("unknown command %s", cmd)
	}

	return
}

var (
	ErrShowHelp = errors.New("help shown")
)

func parseFlags() (err error) {

	// separate args and flags
	flagArgs := make([]string, 0)
	args = make([]string, 0)
	help := false
	for _, a := range os.Args[1:] {
		if (len(a) >= 2 && a[:2] == "-h") || (len(a) >= 3 && a[:3] == "--h") {
			help = true
			break
		}
		var c byte
		if len(a) > 0 {
			c = a[0]
		}
		if c == '-' {
			flagArgs = append(flagArgs, a)
		} else {
			args = append(args, a)
		}
	}

	// set normal flags
	fs := flag.NewFlagSet("cmd", flag.ContinueOnError)

	quiet := false

	fs.BoolVar(&cfg.force, "f", cfg.force, "Force overwrite")
	fs.BoolVar(&cfg.keepGap, "k", cfg.keepGap, "Keep gaps between savefile IDs")
	//fs.BoolVar(&cfg.verbose, "v", cfg.verbose, "verbose mode")
	fs.BoolVar(&quiet, "q", !cfg.verbose, "quiet. suppress non-error messages")
	fs.BoolVar(&cfg.useDefaultExt, "x", cfg.useDefaultExt, fmt.Sprintf("add extension (%s) to file if no extension found", extRpgArchive))
	fs.StringVar(&cfg.comment, "c", "", "set comment to modifying savefiles")

	// alternative flags
	fs.Bool("no-default-ext", false, "same as '-x=false'")
	//fs.Bool("no-gap", false, "same as '-k=false'")

	// show helps
	if help {
		fs.Usage()
		return ErrShowHelp
	}

	// add hidden flags
	fs.BoolVar(&cfg.rawJson, "j", cfg.rawJson, "")       // write savefiles as raw json, rather than encrypted lzstring
	fs.BoolVar(&cfg.prettyJson, "p", cfg.prettyJson, "") // write pretty-printed json when writing json

	fs.SetOutput(io.Discard) // disable output to prevent error and usage printing, which may reveals hidden commands.

	// parse flags
	err = fs.Parse(flagArgs)
	if err != nil {
		return
	}

	cfg.verbose = !quiet // verbose is inverse of the quiet flag

	fs.Visit(func(f *flag.Flag) { // check whether each flag is set
		switch f.Name {
		case "c":
			// comment has set
			cfg.setComment = true // turn comment-modify flag ON

		case "no-default-ext":
			if f.Value.String() == "true" {
				cfg.useDefaultExt = false
			}
		//case "no-gap":
		//	if f.Value.String() == "true" {
		//		cfg.keepGap = false
		//	}
		case "verbose":
			//fs.Lookup("v").Value.Set(f.Value.String())
			cfg.verbose = (f.Value.String() == "true")
		}
	})
	return nil
}

func main() {

	err := parseFlags()

	if err == nil {
		// execute the main function
		err = run()
	}

	if err != nil && err != ErrShowHelp {
		//fmt.Fprintln(os.Stderr, "Error: "+err.Error())
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
