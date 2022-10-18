package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
	mvSaveGlobal  = "global.rpgsave" // rpg maker mv save index file
	mvSaveFileFmt = "file%d.rpgsave" // individual rpg maker mv save file

	extRpgArchive = ".rpgarch" // default extension of the archive file

	idSeparator = '#' // separator between filename and id

	idNotOpenEnded = math.MaxInt // the id that represents not-a-open-range
)

var (
	idMatch       = regexp.MustCompile(`^(.*?)([#@]([\d,-]+))?$`) // FILENAME.EXT#id,id-id,...
	saveFileMatch = regexp.MustCompile(`^file(\d+)\.rpgsave$`)    // file[ID].rpgsave
)

func rpgMvIndexFilename(dirpath string) string {
	return filepath.Join(dirpath, mvSaveGlobal)
}
func rpgMvSaveFilename(dirpath string, id int) string {
	return filepath.Join(dirpath, fmt.Sprintf(mvSaveFileFmt, id))
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

// A generic save entry
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
func (ss *saveFileSelector) readRpgMvSaveIndex() (save []*saveEntry, err error) {
	// read global.rpgsave
	lzs, err := readLzstringFile(rpgMvIndexFilename(ss.Path))
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
	ss.ResetId()
	currentId, ok := ss.NextId()
	for i, d := range sIndex {
		if d == nil {
			continue
		}
		for currentId < i && ok {
			currentId, ok = ss.NextId()
		}
		if !ok {
			break
		}
		if i < currentId {
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
func (ss *saveFileSelector) readRpgMvSaveAll() (save []*saveEntry, err error) {
	// read global.save
	s, err := ss.readRpgMvSaveIndex()
	if err != nil {
		return
	}

	// read each savefile
	for _, f := range s {
		savename := rpgMvSaveFilename(ss.Path, f.Id)
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
		for e.Id > len(sIndex) {
			sIndex = append(sIndex, []byte("null"))
		}
		sIndex = append(sIndex, data)
	}
	js, err := json.Marshal(sIndex)
	if err != nil {
		return
	}
	indexFile := rpgMvIndexFilename(dirpath)
	enc := lzstring.CompressToBase64(string(js))
	return os.WriteFile(indexFile, []byte(enc), 0644)
}

// write individual savedata files to rpg maker mv save directory
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

// write index and savedata files to rpg maker mv save directory
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
			//log.Printf("not changed")
		case nil:
			// do nothing
		default:
			return e
		}
		//f.filename = fname // store last save filename
	}

	return
}

// remove individual savefiles that are NOT contained in the save entries
func removeUnusedRpgMvSave(dirpath string, save []*saveEntry) (err error) {
	// list indexes
	idmap := make(map[int]bool)
	for _, s := range save {
		idmap[s.Id] = true
	}

	// list files
	fl, err := os.ReadDir(dirpath)
	if err != nil {
		return
	}
	for _, f := range fl {
		if f.IsDir() {
			continue
		}
		m := saveFileMatch.FindStringSubmatch(f.Name())
		if m == nil {
			continue
		}
		id, _ := strconv.Atoi(m[1])
		if !idmap[id] {
			err = os.Remove(f.Name())
			if err != nil {
				return
			}
		}
	}

	return nil
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
func (ss *saveFileSelector) readRpgArch() (save []*saveEntry, err error) {
	data, err := os.ReadFile(ss.NormalizedPath)
	if err != nil {
		return
	}
	var arch []*archEntry
	err = json.Unmarshal(data, &arch)
	if err != nil {
		return
	}

	// create savefile
	sv := make([]*saveEntry, 0)

	// normalize index
	ss.ResetId()
	currentId, ok := ss.NextId()
	for _, en := range arch {
		for currentId < en.Id && ok {
			currentId, ok = ss.NextId()
		}
		if !ok {
			break
		}
		if en.Id < currentId {
			continue
		}

		sve := &saveEntry{
			Id:      en.Id,
			Comment: en.Comment,
		}
		if en.IndexJson != nil {
			// index in raw json
			sve.IndexJson = en.IndexJson
		} else if en.Index != "" {
			// index in compressed lzstring
			jstr, e := lzstring.DecompressBase64(en.Index)
			if e == nil && jstr != "" {
				sve.IndexJson = []byte(jstr)
			}
		}
		if en.SaveData != "" {
			// savedata in compressed lzstring
			sve.SaveData = en.SaveData
		} else if en.SaveJson != nil {
			// compress raw json to lzstring
			sve.SaveData = lzstring.CompressToBase64(string(en.SaveJson))
		}
		sv = append(sv, sve)
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

// a struct to hold save filename and index
type saveFileSelector struct {
	Path           string // input path
	NormalizedPath string // normalized path created when opening the file
	IsRpgMvSave    bool   // true if the NormalizedPath points toa rpg maker MV save directory

	// ID list generator. usually the parsed result of #ID,ID,ID-... string
	IdList    []int // list of individual IDs
	OpenStart int   // the first id of open-ended id list. if idNotOpen, then there is no id list

	currentIdList []int // internal vars for NextId()
	currentOpen   int
}

// init the savefile selector with filepath and id string
func NewSaveFileSelector(pathAndId string) (*saveFileSelector, error) {
	path, id, openStart, err := parsePathIndex(pathAndId)
	if err != nil {
		return nil, err
	}

	return &saveFileSelector{
		Path:           path,
		NormalizedPath: path,

		IdList:    id,
		OpenStart: openStart,

		currentIdList: id,
		currentOpen:   openStart,
	}, nil
}

// make filepath with an ID to be displayed
func (ss *saveFileSelector) displayPath(id int) string {
	if ss.IsRpgMvSave {
		return rpgMvSaveFilename(ss.NormalizedPath, id)
	}
	return fmt.Sprintf("%s%c%d", ss.NormalizedPath, idSeparator, id)
}

// generate the next id.
func (ss *saveFileSelector) NextId() (id int, ok bool) {
	ok = true
	if len(ss.currentIdList) > 0 {
		// the list is not empty
		id = ss.currentIdList[0]
		ss.currentIdList = ss.currentIdList[1:]
	} else {
		if ss.currentOpen == idNotOpenEnded {
			// the list is not open ended
			id = idNotOpenEnded
			ok = false
			return
		}
		id = ss.currentOpen
		ss.currentOpen++
	}
	return
}

// reset the id generation.
func (ss *saveFileSelector) ResetId() {
	ss.currentIdList, ss.currentOpen = ss.IdList, ss.OpenStart
}

// parse filename with ID numbers separated with a idSeparator mark.
// ID is comma-separated, hyphen-connected increasing numbers.
// openStartId contains the last id entry when it ends with a hyphen. idNotOpen if the list is not open-ended.
// ex) "FILENAME#1,2,7,8-10,13,25-" -> id=[1,2,7,8,9,10,13], openStart=25
func parsePathIndex(namepath string) (path string, id []int, openStartId int, err error) {

	idStr := ""
	m := idMatch.FindStringSubmatch(namepath)
	if m != nil {
		namepath, idStr = m[1], m[3]
	}

	if namepath == "" {
		// the current directory
		namepath = "." + string(os.PathSeparator)
	}

	if idStr == "" || idStr == "*" {
		// special case: treat "file%d.rpgsave" as rpgMvSave file
		fdir, fname := filepath.Split(namepath)
		m := saveFileMatch.FindStringSubmatch(fname)
		if m != nil {
			fid, _ := strconv.Atoi(m[1])
			if fdir == "" {
				fdir = "."
			}
			fdir += string(os.PathSeparator)
			return fdir, []int{fid}, idNotOpenEnded, nil
		}

		// list with open end
		return namepath, nil, 1, nil
	}

	// split IDstr with commas
	idList := make([]int, 0)
	a := strings.Split(idStr, ",")
	last := -1
	_openStartId := idNotOpenEnded
	_endIsOpen := false
	for _, s := range a {
		if _endIsOpen {
			// new entry appears after an open-ended entry
			err = ErrInvalidId
			return
		}
		m := mIdRange.FindStringSubmatch(s) // "START-END"
		if m != nil {                       // id range
			a, b := 0, 0
			if m[1] != "" {
				a, err = strconv.Atoi(m[1])
				if err != nil {
					return
				}
				if a <= last {
					err = ErrInvalidId
					return
				}
			}
			if m[2] == "" {
				// open end
				_openStartId, _endIsOpen = a, true
				continue
			}
			b, err = strconv.Atoi(m[2])
			if err != nil {
				return
			}
			for i := a; i <= b; i++ {
				idList = append(idList, i)
			}
			last = b
		} else { // single id
			var n int
			n, err = strconv.Atoi(s)
			if err != nil {
				return
			}
			if n <= last {
				err = ErrInvalidId
				return
			}
			idList = append(idList, n)
			last = n
		}
	}

	return namepath, idList, _openStartId, nil
}

// determine the type of save at the path
func detectSaveType(inPath string) (path string, isRpgMvSave bool, err error) {

	mkDirPath := func(s string) string {
		// append / or \ at the end of the path
		return filepath.Join(s, "") + string(os.PathSeparator)
	}

	_, f := filepath.Split(inPath)
	if f == "" {
		// the path ends with a slash
		// treat it as an RpgMvSave directory
		return mkDirPath(inPath), true, nil
	}

	st, err := os.Lstat(inPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return
		}
		ext := filepath.Ext(inPath)
		if ext == "" && cfg.useDefaultExt {
			inPath = inPath + extRpgArchive
			st, err = os.Lstat(inPath)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return
			}
		}
	}
	if st == nil {
		// file not found: treat it as a rpgarch archive
		return inPath, false, nil
	}

	if st.IsDir() {
		// a directory found
		return mkDirPath(inPath), true, nil
	}
	_p, _n := filepath.Split(inPath)
	if _n == mvSaveGlobal {
		// the file is global.rpgsave in the save directory
		inPath = _p
		isRpgMvSave = true
		return _p, true, nil
	}
	// the path is a normal file
	return inPath, false, nil
}

// open the save at the path, autodetecting the save type,
// if indexOnly is true, then actualy save body is not loaded for Rpgmaker Mv savefiles.
// if allEntry is true, then ss.IdList and ss.OpenStart is ignored and all entries are loaded
func (ss *saveFileSelector) readSaveAtPath(indexOnly bool, allEntry bool) (save []*saveEntry, err error) {

	if allEntry && (len(ss.IdList) > 0 || ss.OpenStart != idNotOpenEnded) { // read all entry
		// make a temporary selector with "all" selector
		tmpss := *ss
		tmpss.IdList, tmpss.OpenStart = nil, 0
		tmpss.ResetId()
		// open the file with the temporary selector
		save, err = tmpss.readSaveAtPath(indexOnly, false)
		// store parsed path info
		ss.NormalizedPath, ss.IsRpgMvSave = tmpss.NormalizedPath, tmpss.IsRpgMvSave
		return
	}

	path, rpgMvSave, err := detectSaveType(ss.Path)
	if err != nil {
		return
	}
	ss.NormalizedPath, ss.IsRpgMvSave = path, rpgMvSave
	if rpgMvSave {
		if indexOnly {
			save, err = ss.readRpgMvSaveIndex()
			return
		} else {
			save, err = ss.readRpgMvSaveAll()
			return
		}
	}
	save, err = ss.readRpgArch()
	return
}

// write the save to the path, autodetecting the save type
func (ss *saveFileSelector) writeSaveToPath(save []*saveEntry, rawJson, pretty bool) (err error) {
	path, rpgMvSave := ss.NormalizedPath, ss.IsRpgMvSave
	if path == "" {
		path, rpgMvSave, err = detectSaveType(ss.Path)
		if err != nil {
			return
		}
	}
	if rpgMvSave {
		// write the savefiles as Rpg maker MV save directory
		err = os.MkdirAll(path, 0644)
		if err != nil {
			return
		}
		return writeRpgMvSaveAll(path, save)
	}
	// write the savefiles as a JSON archive
	return writeRpgArch(path, save, rawJson, pretty)
}
