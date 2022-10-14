package main

import (
	"fmt"
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
func cmdLs(path string) (err error) {

	path, idList, openStart, err := splitIndex(path)
	if err != nil {
		return
	}

	saveEntry, err := readSaveAtPath(path, true)
	if err != nil {
		return
	}

	// TODO: show comments if cfg.verbose is set
	// TODO: terminal-aligned texts

	lines := make([]string, 0)
	lines = append(lines, // label
		//"id\000savetime\000playtime\000char\000title\000map",
		"id\000savetime\000playtime\000char\000gold\000map",
	)
	for _, en := range saveEntry {

		if en.Id < openStart {
			if len(idList) == 0 {
				// list is empty
				continue
			}
			for len(idList) > 0 && idList[0] < en.Id {
				// the first entry inexists
				idList = idList[1:]
			}
			if len(idList) == 0 || idList[0] != en.Id {
				// the first entry is larger than this
				continue
			}
			idList = idList[1:]
		}
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

type copyMap struct {
	From, To int
	Save     *saveEntry
}

// sequence emitter
type idEmit struct {
	LList, RList           []int
	LOpenStart, ROpenStart int
	N                      int
}

func (em *idEmit) Next() (L, R int, ok bool) {
	ok = true
	if em.N < len(em.LList) {
		L = em.LList[em.N]
	} else {
		if em.LOpenStart == idNotOpen {
			ok = false
			return
		}
		L = em.LOpenStart
		em.LOpenStart++
	}
	if em.N < len(em.RList) {
		R = em.RList[em.N]
	} else {
		if em.ROpenStart == idNotOpen {
			ok = false
			return
		}
		R = em.ROpenStart
		em.ROpenStart++
	}
	em.N++
	return
}

// open two save file and get mapping between two list
func selectEntryMap(src, dest string) (srcList, destList []*saveEntry, sel []copyMap, err error) {

	src, srcId, srcOpenStart, err := splitIndex(src)
	if err != nil {
		return
	}
	dest, destId, destOpenStart, err := splitIndex(dest)
	if err != nil {
		return
	}

	// check index count
	if destOpenStart != idNotOpen {
		if srcOpenStart == idNotOpen || len(srcId) > len(destId) {
			err = ErrInvalidId
			return
		}
	}

	srcEntry, err := readSaveAtPath(src, false)
	if err != nil {
		return
	}
	srcMap := make(map[int]*saveEntry)
	for _, e := range srcEntry {
		srcMap[e.Id] = e
	}

	destEntry, e := readSaveAtPath(dest, false)
	if e != nil {
		destEntry = make([]*saveEntry, 0)
	}

	// select files from src
	emitter := &idEmit{
		LList:      srcId,
		LOpenStart: srcOpenStart,
		RList:      destId,
		ROpenStart: destOpenStart,
	}
	selection := make([]copyMap, 0)
	for {
		l, r, ok := emitter.Next()
		if !ok {
			break
		}
		for len(srcEntry) > 0 && srcEntry[0].Id < l {
			srcEntry = srcEntry[1:]
		}
		if len(srcEntry) == 0 {
			break
		}
		if srcEntry[0].Id == l {
			selection = append(selection, copyMap{From: l, To: r, Save: srcEntry[0]})
			srcEntry = srcEntry[1:]
		}
	}

	return srcEntry, destEntry, selection, nil
}

// copy a file
func cmdCp(src, dest string) (err error) {

	_, destEntry, cMap, err := selectEntryMap(src, dest)
	if err != nil {
		return
	}

	// change ID of entries to be copied
	copyEntry := make([]*saveEntry, len(cMap))
	for i, e := range cMap {
		copyEntry[i] = e.Save
		copyEntry[i].Id = e.To
	}

	// merge dest with selection
	l := mergeSort(destEntry, copyEntry, func(i, j int) bool { return destEntry[i].Id <= copyEntry[j].Id })

	// check duplicates
	l2 := make([]*saveEntry, 0)
	for i := 0; i < len(l)-1; i++ {
		if cfg.setComment {
			l[i].Comment = cfg.comment
		}
		l2 = append(l2, l[i])
		if l[i].Id == l[i+1].Id {
			overwrite := false
			if cfg.force {
				overwrite = true
			} else {
				// show an overwrite prompt
				overwrite = promptYN(fmt.Sprintf("Overwrite #%d? (y/N) ", l[i].Id), false)
			}
			if overwrite {
				// remove the last added entry
				l2 = l2[:len(l2)-1]
			} else {
				// skip the next entry
				i++
			}
		}
	}

	// save to file
	err = writeSaveToPath(dest, l2, cfg.rawJson, cfg.prettyJson)

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
	if err == nil {
		s := strings.ToLower(string(r))
		if s == "y" {
			return true
		} else if s == "n" {
			return false
		}
	}
	fmt.Print("\n")
	return defaultYes
}

func mergeSort[T any](l, r []T, leftFirst func(i, j int) bool) []T {
	newList := make([]T, len(l)+len(r))
	i, j, k := 0, 0, 0
	for i < len(l) && j < len(r) {
		if leftFirst(i, j) {
			newList[k] = l[i]
			i++
		} else {
			newList[k] = r[j]
			j++
		}
		k++
	}
	for i < len(l) {
		newList[k] = l[i]
		i++
		k++
	}
	for j < len(r) {
		newList[k] = r[j]
		j++
		k++
	}
	return newList
}
