package fsx

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/pschultz/vatsim"
	"github.com/pschultz/vatsim/EuroScope"
	"github.com/pschultz/vatsim/vPilot"
)

type Installer struct {
	ui            cli.Ui
	fsxRoot       string
	modelMatching *vPilot.RuleSets

	// set of created dirs, to skip superfluous os.MkdirAll calls.
	createdDirs vatsim.Set

	// set of existing airpline directories, lowercased.
	existing vatsim.Set

	buf *bytes.Buffer // re-used by loadConfig
}

func NewInstaller(fsxRoot string, modelMatching *vPilot.RuleSets) *Installer {
	return &Installer{
		fsxRoot:       fsxRoot,
		modelMatching: modelMatching,

		createdDirs: make(vatsim.Set),
		existing:    make(vatsim.Set),
		buf:         &bytes.Buffer{},
	}
}

func (i *Installer) Install(ui cli.Ui, p Package) error {
	i.ui = ui

	files, err := ioutil.ReadDir(filepath.Join(i.fsxRoot, "SimObjects/Airplanes"))
	if err != nil {
		return err
	}

	for _, acDir := range files {
		files, err := ioutil.ReadDir(acDir.Name())
		if err != nil {
			continue
		}
		for _, f := range files {
			if strings.ToLower(filepath.Base(f.Name())) == "aircraft.cfg" {
				id := strings.ToLower(filepath.Base(acDir.Name()))
				i.existing.Add(id)
				break
			}
		}
	}

	for dest, r := range p.Files() {
		idest := strings.ToLower(dest)

		if strings.HasSuffix(idest, "aircraft.cfg") {
			if err := i.installConfig(dest, r); err != nil {
				return err
			}
			continue
		}

		err := i.extract(dest, r)
		r.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func (i *Installer) installConfig(dest string, r io.ReadCloser) error {
	acDir := strings.ToLower(filepath.Base(filepath.Dir(dest)))
	if acDir == "" {
		return errors.New("malformed config filename: " + dest)
	}

	exists := i.existing.Has(acDir)

	flags := fixAirlineFlag
	if exists {
		flags |= fixModelFlag
	}

	i.buf.Reset()
	metas, err := i.loadConfig(r, flags, i.buf)
	if err != nil {
		return err
	}

	// TODO: update model matching
	if exists {
		titles := vatsim.NewSet()
		for _, m := range metas {
			titles.Add(m.Title)
		}

		err = i.patchConfig(dest, i.buf, titles)
	} else {
		err = i.extract(dest, i.buf)
	}
	if err != nil {
		return err
	}

	if i.modelMatching != nil {
		for _, meta := range metas {
			i.modelMatching.Add(vPilot.Rule{
				Title:        meta.Title,
				AirlineCode:  euroscope.AirlineCode(meta.ATCAirline),
				AircraftCode: meta.ATCModel,
			})
		}
		if err := vPilot.WriteRuleSets(i.modelMatching); err != nil {
			return err
		}
	}

	for _, meta := range metas {
		mode := "added"
		if exists {
			mode = "patched"
		}
		i.ui.Output(fmt.Sprintf("%s %s (%s): %s",
			meta.ATCModel, meta.ATCAirline, mode, meta.Title))
	}

	return nil
}

const (
	fixAirlineFlag uint8 = 1 << iota
	fixModelFlag
)

// Used to parse aircraft configs
var atcAirlinePattern = regexp.MustCompile(`^(?i)^\s*atc_airline\s*=\s*(.*?)\s*$`)
var atcModelPattern = regexp.MustCompile(`^(?i)^\s*atc_model\s*=\s*(.*?)\s*$`)
var titlePattern = regexp.MustCompile(`^(?i)^\s*title\s*=\s*(.*?)\s*$`)
var fltsimPattern = regexp.MustCompile(`^(?i)^\s*\[fltsim\.(.*)\]`)

// meta reports some things about aircraft.cfg files
type Meta struct {
	SectionIndex int
	ATCModel     string
	ATCAirline   string
	Title        string
}

func (i *Installer) loadConfig(r io.Reader, fixFlags uint8, buf *bytes.Buffer) ([]Meta, error) {
	var metas []Meta
	var meta Meta

	// These config files are a mess; they are clearly produced by humans, not
	// machines. Don't even try to parse them into structs or maps or whatever.
	// Just read them line-wise and copy lines as-is, unless it looks like one
	// that defines atc_airline or atc_model. In that case we emit our own,
	// possibly fixed line.

	var atcModel string

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if x := fltsimPattern.FindStringSubmatch(line); x != nil {
			n, _ := strconv.Atoi(x[1])
			if meta.Title != "" {
				metas = append(metas, meta)
			}
			meta = Meta{
				SectionIndex: n,
			}
		} else if x := titlePattern.FindStringSubmatch(line); x != nil {
			meta.Title = x[1]
		} else if x := atcAirlinePattern.FindStringSubmatch(line); x != nil {
			meta.ATCAirline = x[1]

			if buf != nil && fixFlags&fixAirlineFlag != 0 {
				meta.ATCAirline = euroscope.FixAirline(i.ui, meta.ATCAirline)
				buf.WriteString("atc_airline=")
				buf.WriteString(meta.ATCAirline)
				buf.WriteByte('\n')
				continue
			}
		} else if x := atcModelPattern.FindStringSubmatch(line); x != nil {
			atcModel = x[1]

			if buf != nil && fixFlags&fixModelFlag != 0 {
				atcModel = euroscope.FixAircraft(i.ui, atcModel)
				buf.WriteString("atc_model=")
				buf.WriteString(atcModel)
				buf.WriteByte('\n')
				continue
			}
		}

		if buf != nil {
			fmt.Fprintln(buf, line)
		}
	}

	if meta.Title != "" {
		metas = append(metas, meta)
	}

	for i := range metas {
		metas[i].ATCModel = atcModel
	}

	return metas, scanner.Err()
}

func (i *Installer) patchConfig(dest string, r io.Reader, want vatsim.Set) error {
	f, err := os.OpenFile(filepath.Join(i.fsxRoot, dest), os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	metas, err := i.loadConfig(f, 0, nil)
	if err != nil {
		return err
	}

	have := vatsim.NewSet()
	for _, m := range metas {
		have.Add(m.Title)
	}
	for w := range want {
		if have.Has(w) {
			i.ui.Warn("Duplicate titles; not patching config: " + dest)
			return nil
		}
	}
	// TODO: check duplicate titles globally

	// f is at the end right now, so we can start appending fltsim sections
	// from buf.
	maxSectionIndex := 0
	for _, m := range metas {
		if m.SectionIndex > maxSectionIndex {
			maxSectionIndex = m.SectionIndex
		}
	}
	n := maxSectionIndex + 1
	write := false

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if x := fltsimPattern.FindStringSubmatch(line); x != nil {
			if _, err := fmt.Fprintln(f, ""); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(f, "[fltsim.%d]\n", n); err != nil {
				return err
			}
			n++
			write = true
			continue
		} else if strings.HasPrefix(strings.TrimSpace(strings.ToLower(line)), "[") {
			write = false
		}

		if write {
			if _, err := fmt.Fprintln(f, line); err != nil {
				return err
			}
		}
	}

	return nil
}

func (i *Installer) extract(dest string, r io.Reader) error {
	fname := filepath.Join(i.fsxRoot, dest)
	dir := filepath.Dir(fname)

	if !i.createdDirs.Has(dir) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		i.createdDirs.Add(dir)
	}

	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}
