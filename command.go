package main

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	tty "github.com/mattn/go-tty"
)

/*
//

	func printWidth(s string) int {
		ambiguous := 2 // width for EastAsianAmbiguous
		w := 0
		b := []byte(s)
		for len(b) > 0 {
			p, n := width.Lookup(b)
			b = b[n:]
			switch p.Kind() {
			case width.EastAsianAmbiguous:
				w += ambiguous
			case width.Neutral, width.EastAsianNarrow, width.EastAsianHalfwidth:
				w += 1
			case width.EastAsianWide, width.EastAsianFullwidth:
				w += 2
			}
		}
		return w
	}
*/
func runeCount(s string) int {
	return len([]rune(s))
}

func getMaxColumnSize(lines [][]string) []int {
	// get max length of each column
	columnSize := make([]int, 0)
	for _, l := range lines {
		for i, c := range l {
			for i >= len(columnSize) {
				columnSize = append(columnSize, 0)
			}
			//w := printWidth(c)
			w := runeCount(c)
			if w > columnSize[i] {
				columnSize[i] = w
			}
		}
	}
	return columnSize
}

func printAlignedStrings(lines [][]string) {
	cs := getMaxColumnSize(lines)
	fstr := ""
	for i := range cs {
		if cs[i] == 0 {
			continue
		}
		if fstr != "" {
			fstr += "  "
		}
		fstr += fmt.Sprintf("%%-%ds", cs[i]) // left aligned string
	}
	fstr += "\n"
	//log.Printf("%v", fstr)
	for _, l := range lines {
		a := make([]any, len(l))
		for i := 0; i < len(l); i++ {
			a[i] = l[i]
		}
		fmt.Printf(fstr, a...)
	}
}

func printAlignedLines(lines []string, sep string) {
	ll := make([][]string, len(lines))
	for i, l := range lines {
		ll[i] = strings.Split(l, sep)
	}
	printAlignedStrings(ll)
}

// List savefiles
// func cmdLs(path string) (err error) {
func cmdLs(ss *saveFileSelector) (err error) {

	saveEntry, err := ss.readSaveAtPath(true, false)
	if err != nil {
		return
	}

	// TODO: show comments if cfg.verbose is set
	// TODO: terminal-aligned texts

	title := ""
	if len(saveEntry) > 0 {
		var ie *rpgMvSaveIndexEntry
		ie, err = saveEntry[0].indexEntry()
		if err != nil {
			return
		}
		title = ie.Title
	}

	fmt.Printf("%s", ss.NormalizedPath)
	if title != "" {
		fmt.Printf(" %s", title)
	}
	fmt.Println()

	lines := make([]string, 0)
	lines = append(lines, // label
		//"id\000savetime\000playtime\000char\000title\000map",
		"id\000savetime\000playtime\000char\000gold\000map",
	)
	for _, en := range saveEntry {
		ie, e := en.indexEntry()
		if e != nil {
			continue
		}

		//ts := ie.timestamp().Format("2006-01-02 15:04:05")
		ts := ie.timestamp().Format("2006-01-02 15:04")
		charcount := len(ie.Characters)
		playtime := ie.Playtime
		if len(playtime) == 8 {
			// truncate ":second"
			playtime = playtime[:5]
		}
		lines = append(lines, fmt.Sprintf(
			//				"#%d\000%s\000[%s]\000%d\000%s\000%s",
			//				en.Id, ts, playtime, charcount, ie.Title, ie.MapName,
			"#%d\000%s\000[%s]\000%d\000%d\000%s",
			en.Id, ts, playtime, charcount, ie.Gold, ie.MapName,
		))
	}
	printAlignedLines(lines, "\000")

	return
}

// remove savefiles
func cmdRm(ss *saveFileSelector) (err error) {
	// read all savedata at dest savefile
	entries, err := ss.readSaveAtPath(false, true)
	if err != nil {
		return
	}

	// select non-deleting entires
	ss.ResetId()
	newSave := make([]*saveEntry, 0)
	nextId, removing := ss.NextId()
	for _, e := range entries {
		if e.Id == nextId {
			// remove ID matched; skip without append to the new entry
			if cfg.verbose {
				fmt.Printf("removing %s#%d\n", ss.NormalizedPath, e.Id)
			}
			nextId, removing = ss.NextId()
			continue
		}
		if !removing || e.Id < nextId {
			// keep the savedata
			newSave = append(newSave, e)
			continue
		}
		for nextId < e.Id && removing { // get the next remove ID
			nextId, removing = ss.NextId()
		}
	}

	// save to file
	err = ss.writeSaveToPath(newSave, cfg.rawJson, cfg.prettyJson)
	return
}

// copy savedata.
func cmdCp(src []*saveFileSelector, dest *saveFileSelector) (err error) {

	// read all savedata at dest savefile
	destEntry, _ := dest.readSaveAtPath(false, true)

	// merge src savefiles into the dest savefile
	dest.ResetId()
	newSave := make([]*saveEntry, 0)
	for _, ss := range src {
		var srcEntry []*saveEntry
		srcEntry, err = ss.readSaveAtPath(false, false)
		if err != nil {
			return
		}
		prevId := 0
		for _, en := range srcEntry {
			var nextId int
			var ok bool
			if cfg.keepGap {
				for prevId < en.Id {
					nextId, ok = dest.NextId()
					prevId++
				}
			} else {
				nextId, ok = dest.NextId()
				prevId = nextId
			}

			if !ok {
				err = errors.New("too many source savefiles")
				return
			}
			for len(destEntry) > 0 && destEntry[0].Id < nextId {
				// keep the entries that not contained in the overwrite id list
				newSave = append(newSave, destEntry[0])
				destEntry = destEntry[1:]
			}
			if len(destEntry) > 0 && destEntry[0].Id == nextId {
				// duplicated ID
				overwrite := false
				if cfg.force {
					overwrite = true
				} else {
					// show an overwrite prompt
					overwrite = promptYN(fmt.Sprintf("Overwrite #%d with %s#%d? (y/N) ", nextId, ss.NormalizedPath, en.Id), false)
				}
				if !overwrite {
					// keep the old entry
					newSave = append(newSave, destEntry[0])
					destEntry = destEntry[1:]
					continue
				}
				// overwriting; skip the existing entry
				destEntry = destEntry[1:]
			}
			// copy a source entry to dest
			if cfg.verbose {
				fmt.Printf("copying %s#%d to %s#%d\n", ss.NormalizedPath, en.Id, dest.NormalizedPath, nextId)
			}
			en.Id = nextId
			newSave = append(newSave, en)
		}
	}

	// save to file
	err = dest.writeSaveToPath(newSave, cfg.rawJson, cfg.prettyJson)
	return
}

// convert save entry list to map of id=>*saveEntry
/*
func mkOrderedMap(save []*saveEntry) *OrderedMap[int, *saveEntry] {
	sm := NewOrderedMap[int, *saveEntry]()
	for _, e := range save {
		sm.Append(e.Id, e)
	}
	return sm
}
*/
func mkMap(save []*saveEntry) map[int]*saveEntry {
	sm := make(map[int]*saveEntry)
	for _, e := range save {
		sm[e.Id] = e
	}
	return sm
}

// move savedata between files.
func cmdMv(src []*saveFileSelector, dest *saveFileSelector) (err error) {

	// all savedata
	saveFiles := make(map[string]map[int]*saveEntry)

	// list of open save file
	srcFiles := make(map[string]*saveFileSelector)

	// read all savedata at dest savefile
	destEntry, _ := dest.readSaveAtPath(false, true)
	destM := mkMap(destEntry)
	saveFiles[dest.NormalizedPath] = destM

	// merge src savefiles into the dest savefile
	dest.ResetId()

	newSave := make([]*saveEntry, 0) // map of id -> saveEntry map to be added to dest

	for _, ss := range src {

		// open the save file
		var srcEntry []*saveEntry
		srcEntry, err = ss.readSaveAtPath(false, true)
		if err != nil {
			return
		}

		// check if the save file is already in the cache list
		srcM, ok := saveFiles[ss.NormalizedPath]
		if !ok {
			srcM = mkMap(srcEntry)
			srcFiles[ss.NormalizedPath] = ss
		}
		maxSrcId := 0
		for _, e := range srcM {
			if e.Id > maxSrcId {
				maxSrcId = e.Id
			}
		}

		sameFile := (ss.NormalizedPath == dest.NormalizedPath)

		// copy individual savefiles
		ss.ResetId()
		prevId := -1
		for {
			stepCounter := 1 // distance between previous dest ID to next dest ID
			// get next srcId from srcId list
			srcId, srcOk := ss.NextId()
			if !srcOk || srcId > maxSrcId {
				break
			}
			if prevId != -1 {
				if cfg.keepGap {
					stepCounter = srcId - prevId
				}
			}
			prevId = srcId

			// get save entry of the id
			se, ok := srcM[srcId]
			if !ok { // no entry found at the id
				prevId -= stepCounter // rewind the distance
				continue
			}

			var destId int
			var destOk bool
			if cfg.keepGap {
				for stepCounter > 0 {
					destId, destOk = dest.NextId()
					stepCounter--
				}
			} else {
				destId, destOk = dest.NextId()
			}
			if !destOk {
				err = fmt.Errorf("too many source savefiles")
				return
			}

			overwrite := true
			if _, ok := destM[destId]; ok {
				// destination file has an entry with the same ID
				if cfg.force {
					overwrite = true
				} else {
					// show an overwrite prompt
					overwrite = promptYN(fmt.Sprintf("Overwrite #%d with %s#%d? (y/N) ", destId, ss.NormalizedPath, srcId), false)
				}
			}

			if overwrite {
				if cfg.verbose {
					fmt.Printf("moving %s#%d to #%d\n", ss.NormalizedPath, srcId, destId)
				}
				se.Id = destId
				newSave = append(newSave, se)
				delete(srcM, srcId)
				if sameFile { // the src and dest is same file
					delete(destM, srcId)
				}
			}
		}
		// save savedata of a source file
		saveFiles[ss.NormalizedPath] = srcM

	}

	// write move destination file
	for _, e := range destM {
		newSave = append(newSave, e)
	}
	sort.Slice(newSave, func(i, j int) bool { return newSave[i].Id < newSave[j].Id })
	err = dest.writeSaveToPath(newSave, cfg.rawJson, cfg.prettyJson)
	if err != nil {
		return
	}

	// write modified source files
	for name, ss := range srcFiles {
		if name == dest.NormalizedPath {
			continue
		}
		sm := saveFiles[name]
		save := make([]*saveEntry, 0, len(sm))
		for _, e := range sm {
			save = append(save, e)
		}
		sort.Slice(save, func(i, j int) bool { return save[i].Id < save[j].Id })
		log.Printf("%v", save)
		err = ss.writeSaveToPath(save, cfg.rawJson, cfg.prettyJson)
		if err != nil {
			return
		}
	}

	return
}

func promptYN(msg string, defaultYes bool) bool {
	tt, err := tty.Open()
	if err != nil {
		return defaultYes
	}
	defer tt.Close()

	fmt.Print(msg)
	r, err := tt.ReadRune()
	fmt.Print("\n")
	if err == nil {
		s := strings.ToLower(string(r))
		if s == "y" {
			return true
		} else if s == "n" {
			return false
		}
	}
	return defaultYes
}
