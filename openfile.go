package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	lzstring "github.com/mixcode/golib-lzstring"
)

const (
	saveGlobal  = "global.rpgsave" // save index file
	saveFileFmt = "file%d.rpgsave" // individual save file

	// the id that represents not-a-open-range
	idNotOpen = math.MaxInt
)

func rpgMvIndexFilename(dirpath string) string {
	return filepath.Join(dirpath, saveGlobal)
}
func rpgMvSaveFilename(dirpath string, id int) string {
	return filepath.Join(dirpath, fmt.Sprintf(saveFileFmt, id))
}

var (
	ErrNoData     = errors.New("no contents")
	ErrNotChanged = errors.New("not changed")
	ErrInvalidId  = errors.New("invalid id")
)

func readLzstringFile(filename string) (data string, err error) {
	lzs, err := os.ReadFile(filename)
	if err != nil {
		return
	}
	return lzstring.DecompressBase64(string(lzs))
}

// A single entry of global.rpgsave
// Note that entries of global.rpgsave is should be treated as read-only; Actual data MUST BE handled as json.RawMessage and MUST NOT be changed.
type rpgMvSaveIndexEntry struct {
	GlobalId string `json:"globalId"` // "RPGMV"
	Title    string `json:"title"`    // The title name of the game

	// character info
	Characters []json.RawMessage `json:"characters"`
	Faces      []json.RawMessage `json:"faces"`

	// global info
	Playtime  string `json:"playtime"`
	Timestamp int64  `json:"timestamp"`
	MapName   string `json:"mapname"`
	Gold      int    `json:"gold"`
	SaveCount int    `json:"savecount"`

	// unknown fields
	//Value1        int `json:"value1"`
	//Value2        int `json:"value2"`
	//Value3        int `json:"value3"`
	//Value4        int `json:"value4"`
	//GradeVariable int `json:"gradeVariable"`
}

func (ie *rpgMvSaveIndexEntry) timestamp() time.Time {
	return time.UnixMilli(ie.Timestamp)
}

// A save entry
type saveEntry struct {
	Id int // ID number

	IndexJson json.RawMessage // decoded raw index, extracted from "global.rpgsave"
	SaveData  string          // contents of "file%d.rpgsave"

	Comment string // comment
}

func (se *saveEntry) indexEntry() (indexEntry *rpgMvSaveIndexEntry, err error) {
	if se.IndexJson == nil {
		err = ErrNoData
		return
	}
	var ie rpgMvSaveIndexEntry
	err = json.Unmarshal(se.IndexJson, &ie)
	if err != nil {
		return
	}
	return &ie, nil
}

// read rpg maker mv save directory, index only
func readRpgMvSaveIndex(dirpath string) (save []*saveEntry, err error) {
	// read global.rpgsave
	lzs, err := readLzstringFile(rpgMvIndexFilename(dirpath))
	if err != nil {
		return
	}
	var sIndex []json.RawMessage
	err = json.Unmarshal([]byte(lzs), &(sIndex))
	if len(sIndex) == 0 {
		return
	}

	// build save index without body
	sdata := make([]*saveEntry, 0)
	if err != nil {
		return
	}
	for i, d := range sIndex {
		if d == nil {
			continue
		}
		var se *rpgMvSaveIndexEntry
		e := json.Unmarshal(d, &se)
		if e != nil || se == nil {
			continue
		}
		// note: actual save data is NOT read
		newSE := &saveEntry{
			Id:        i,
			IndexJson: d,
		}
		sdata = append(sdata, newSE)
	}

	return sdata, nil
}

// read rpg maker mv save files
func readRpgMvSaveAll(dirpath string) (save []*saveEntry, err error) {
	// read global.save
	s, err := readRpgMvSaveIndex(dirpath)
	if err != nil {
		return
	}

	// read each savefile
	for _, f := range s {
		savename := rpgMvSaveFilename(dirpath, f.Id)
		data, e := os.ReadFile(savename)
		if e != nil {
			continue
		}
		f.SaveData = string(data)
	}

	return s, nil
}

// write savefile index to global.rpgsave
func writeRpgMvSaveIndex(save []*saveEntry, dirpath string) (err error) {
	sIndex := make([]json.RawMessage, 0)
	for _, e := range save {
		data := e.IndexJson
		if data == nil {
			continue
		}
		for e.Id < len(sIndex) {
			sIndex = append(sIndex, []byte("null"))
		}
		sIndex = append(sIndex, data)
	}
	js, err := json.Marshal(sIndex)
	if err != nil {
		return
	}
	indexFile := rpgMvIndexFilename(dirpath)
	return os.WriteFile(indexFile, js, 0644)
}

// write savedata to rpg maker mv save directory
func writeRpgMvSave(dirpath string, save *saveEntry) (filename string, err error) {
	filename = rpgMvSaveFilename(dirpath, save.Id)
	if save.SaveData == "" {
		err = ErrNoData
		return
	}
	// compare file contents
	content, e := os.ReadFile(filename)
	if e == nil {
		if string(content) != save.SaveData {
			// no need to copy
			err = ErrNotChanged
			return
		}
	}
	err = os.WriteFile(filename, []byte(save.SaveData), 0644)
	return
}

// write savedata to rpg maker mv save directory
func writeRpgMvSaveAll(dirpath string, save []*saveEntry) (err error) {
	// write index
	err = writeRpgMvSaveIndex(save, dirpath)
	if err != nil {
		return
	}

	// write each file
	for _, f := range save {
		_, e := writeRpgMvSave(dirpath, f)
		switch e {
		case ErrNoData:
		case ErrNotChanged:
			log.Printf("not changed")
		default:
			return e
		}
		//f.filename = fname // store last save filename
	}

	return
}

// .rpgarch archive file entry
type archEntry struct {
	Id int `json:"id"` // ID number

	Index     string          `json:"index,omitempty"`     // lzstring-compresse index json
	IndexJson json.RawMessage `json:"indexJson,omitempty"` // decoded raw index, extracted from "global.rpgsave"

	SaveData string          `json:"saveData,omitempty"` // contents of "file%d.rpgsave"
	SaveJson json.RawMessage `json:"saveJson,omitempty"` // decoded save data

	Comment string `json:"comment,omitempty"` // comment
}

// read rpgarch file
func readRpgArch(filename string) (save []*saveEntry, err error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return
	}
	var arch []*archEntry
	err = json.Unmarshal(data, &arch)
	if err != nil {
		return
	}

	// create savefile
	sv := make([]*saveEntry, len(arch))

	// normalize index
	for i, en := range arch {
		sve := &saveEntry{
			Id:      en.Id,
			Comment: en.Comment,
		}
		if en.IndexJson != nil {
			sve.IndexJson = en.IndexJson
		} else if en.Index != "" {
			// index is json-first
			jstr, e := lzstring.DecompressBase64(en.Index)
			if e == nil && jstr != "" {
				sve.IndexJson = []byte(jstr)
			}
		}
		if en.SaveData != "" {
			sve.SaveData = en.SaveData
		} else if en.SaveJson != nil {
			sve.SaveData = lzstring.CompressToBase64(string(en.SaveJson))
		}
		sv[i] = sve
	}
	return sv, nil
}

func writeRpgArch(filename string, save []*saveEntry, rawJson, pretty bool) (err error) {
	arch := make([]*archEntry, len(save))
	for i, se := range save {
		ae := &archEntry{
			Id:      se.Id,
			Comment: se.Comment,
		}
		if rawJson {
			ae.IndexJson = se.IndexJson
			jstr, e := lzstring.DecompressBase64(se.SaveData)
			if e == nil {
				ae.SaveJson = json.RawMessage(jstr)
			}
		} else {
			ae.Index = lzstring.CompressToBase64(string(se.IndexJson))
			ae.SaveData = se.SaveData
		}
		arch[i] = ae
	}
	var data []byte
	if pretty {
		data, err = json.MarshalIndent(arch, "", "\t")
	} else {
		data, err = json.Marshal(arch)
	}
	if err != nil {
		return
	}

	return os.WriteFile(filename, data, 0644)
}

var (
	mIdRange = regexp.MustCompile(`^(\d*)-(\d*)$`)
)

// parse filename with ID numbers separated with a # mark.
// ID is comma-separated, hyphen-connected increasing numbers.
// openEnd is true if last ID is end with a hyphen
// ex) FILENAME#1,2,7,8-9,13,25-
// func splitIndex(namepath string) (path string, id []int, openEnd bool, err error) {
func splitIndex(namepath string) (path string, id []int, openStart int, err error) {
	a := strings.Split(namepath, "#")

	idStr := ""
	if len(a) > 1 {
		idStr = a[len(a)-1]
		namepath = strings.Join(a[:len(a)-1], "#")
	}

	if idStr == "" || idStr == "*" {
		// list with open end
		return namepath, nil, 1, nil
	}

	// split IDstr with commas
	idList := make([]int, 0)
	a = strings.Split(idStr, ",")
	last := -1
	_openStart := idNotOpen
	_openEnd := false
	for _, s := range a {
		if _openEnd {
			// must NOT loop
			err = ErrInvalidId
			return
		}
		m := mIdRange.FindStringSubmatch(s) // "START-END"
		if m != nil {
			a, e := strconv.Atoi(m[1])
			if e != nil {
				err = e
				return
			}
			if a <= last {
				err = ErrInvalidId
				return
			}
			if m[2] == "" {
				// open end
				_openStart, _openEnd = a, true
				continue
			}
			b, e := strconv.Atoi(m[2])
			if e != nil {
				err = e
				return
			}
			for i := a; i <= b; i++ {
				idList = append(idList, i)
			}
			last = b
			continue
		}
		n, e := strconv.Atoi(s)
		if e != nil {
			err = e
			return
		}
		if n <= last {
			err = ErrInvalidId
			return
		}
		idList = append(idList, n)
		last = n
	}

	return namepath, idList, _openStart, nil
}

// determine the type of save at the path
func detectSaveType(inPath string) (path string, isRpgMvSave bool, err error) {
	st, err := os.Lstat(inPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// file not exist: must be a new archive type
			return inPath, false, nil
		}
		return
	}
	rpgMvSave := false
	if st.IsDir() {
		rpgMvSave = true
	} else {
		_p, _n := filepath.Split(inPath)
		if _n == saveGlobal {
			inPath = _p
			rpgMvSave = true
		}
	}
	return inPath, rpgMvSave, nil
}

// open the save at the path
func readSaveAtPath(path string, indexOnly bool) (save []*saveEntry, err error) {
	path, rpgMvSave, err := detectSaveType(path)
	if err != nil {
		return
	}
	if rpgMvSave {
		if indexOnly {
			return readRpgMvSaveIndex(path)
		} else {
			return readRpgMvSaveAll(path)
		}
	}
	return readRpgArch(path)
}

func writeSaveToPath(path string, save []*saveEntry, rawJson, pretty bool) (err error) {
	path, _, _, _ = splitIndex(path)
	path, rpgMvSave, err := detectSaveType(path)
	if err != nil {
		return
	}
	if rpgMvSave {
		return writeRpgMvSaveAll(path, save)
	}
	return writeRpgArch(path, save, rawJson, pretty)
}
