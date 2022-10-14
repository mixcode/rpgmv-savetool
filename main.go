package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	lzstring "github.com/mixcode/golib-lzstring"
)

type config struct {
	force      bool // force overwrite
	rawJson    bool // save raw json (if possible)
	prettyJson bool // save pretty formatted json
	verbose    bool // verbose mode
	setComment bool // set comments to modifying entries
	comment    string
}

var (
	cfg = config{
		force:      false,
		rawJson:    false,
		prettyJson: false,
		verbose:    false,
		setComment: false,
	}

	args []string
)

func getArg(n int) string {
	if n < len(args) {
		return args[n]
	}
	return ""
}

func run() (err error) {
	if len(args) == 0 {
		err = errors.New("give a command. use -h for help")
		return
	}
	cmd := args[0]

	switch cmd {
	case "ls": // list the contents of the archive
		path := getArg(1)
		if path == "" {
			path = "."
		}
		err = cmdLs(path)

	case "cp": // copy savefile between archives
		src, dest := getArg(1), getArg(2)
		if src == "" {
			err = fmt.Errorf("please give a source filename and/or an #id")
			return
		}
		if dest == "" {
			err = fmt.Errorf("please give a dest filename and/or an #id")
			return
		}
		err = cmdCp(src, dest)

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

func main() {
	var err error

	/*
		force      bool // force overwrite
		rawJson    bool // save raw json (if possible)
		prettyJson bool // save pretty formatted json
		verbose    bool // verbose mode

		setComment bool // set comments to modifying entries
		comment    string
	*/

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

	fs.BoolVar(&cfg.force, "f", cfg.force, "Force overwrite")
	fs.BoolVar(&cfg.verbose, "v", cfg.verbose, "verbose mode")
	fs.StringVar(&cfg.comment, "c", "", "set comment to modifying savefiles")

	if help {
		fs.Usage()
		os.Exit(0)
	}

	// add hidden flags
	fs.BoolVar(&cfg.rawJson, "j", cfg.rawJson, "")       // print raw json
	fs.BoolVar(&cfg.prettyJson, "p", cfg.prettyJson, "") // pretty-printed json
	fs.Parse(flagArgs)                                   // parse again to enable hidden flags
	fs.Visit(func(f *flag.Flag) {                        // check whether each flag is set
		if f.Name == "c" {
			// comment has entered
			cfg.setComment = true // turn comment-modify flag ON
		}
	})

	// execute the main function
	err = run()

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
