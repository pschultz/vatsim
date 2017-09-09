package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/mitchellh/cli"
	"golang.org/x/net/html"

	"github.com/pschultz/vatsim/EuroScope"
	"github.com/pschultz/vatsim/fsx"
	"github.com/pschultz/vatsim/vPilot"
	"github.com/pschultz/vatsim/woai"
)

var FSXRoot = "/mnt/c/Program Files (x86)/Steam/steamapps/common/FSX"

func prefix(ui cli.Ui, format string, args ...interface{}) *cli.PrefixedUi {
	prefix := fmt.Sprintf(format, args...)
	return &cli.PrefixedUi{
		AskPrefix:       prefix,
		AskSecretPrefix: prefix,
		OutputPrefix:    prefix,
		InfoPrefix:      prefix,
		ErrorPrefix:     prefix + "Error: ",
		WarnPrefix:      prefix + "Warning: ",
		Ui:              ui,
	}
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s PATTERN\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	if d := os.Getenv("FSX_ROOT"); d != "" {
		FSXRoot = d
	}

	if err := euroscope.LoadDefaults(); err != nil {
		log.Fatal(err)
	}
	if err := euroscope.LoadDefaults(); err != nil {
		log.Fatal(err)
	}

	mm, err := vPilot.LoadDefaults()
	if err != nil {
		log.Fatal(err)
	}

	installer := fsx.NewInstaller(FSXRoot, mm)

	rootUI := &cli.BasicUi{
		Reader:      os.Stdin,
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}

	term := args[0]
	nodes, doc, err := search(term)
	if err != nil {
		log.Fatal(err)
	}

	titles := make([]string, 0, len(nodes))
	for title := range nodes {
		titles = append(titles, title)
	}
	sort.Strings(titles)

	for _, title := range titles {
		ui := prefix(rootUI, "%s: ", title)
		answer, err := ui.Ask("Install? (Y/n)")
		if err != nil {
			log.Fatal(err)
		}
		switch strings.ToUpper(answer) {
		case "", "Y":
		default:
			continue
		}

		ui.Output("Downloading")
		for _, n := range nodes[title] {
			b, dlID, err := downloadPackage(href(doc, n))
			if err != nil {
				ui.Error(err.Error())
				continue
			}

			ui := prefix(rootUI, "%s (%s): ", title, dlID)

			p, err := woai.NewPackage(bytes.NewReader(b))
			if err != nil {
				ui.Error(err.Error())
				continue
			}

			ui.Output("Installing")
			if err := installer.Install(ui, p); err != nil {
				ui.Error(err.Error())
				continue
			}
		}
	}
}

func search(pattern string) (map[string][]*html.Node, *goquery.Document, error) {
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest("GET", "http://www.world-of-ai.com/allpackages.php", nil)
	if err != nil {
		return nil, nil, err
	}

	doc, err := parse(req)
	if err != nil {
		return nil, nil, err
	}

	nodes := make(map[string][]*html.Node)
	dlPageAnchors(nodes, re, doc.Find("#airlines ~ table").First())
	dlPageAnchors(nodes, re, doc.Find("#cargo ~ table").First())

	return nodes, doc, nil
}

func dlPageAnchors(nodes map[string][]*html.Node, re *regexp.Regexp, table *goquery.Selection) {
	table.Find("tr").Each(func(_ int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Find("td").Eq(1).Text())
		if !re.MatchString(title) {
			return
		}

		s.Find("td").Eq(5).Each(func(_ int, s *goquery.Selection) {
			x := s.Find("a[href*='library.avsim.net/search.php']").Last().Get(0)
			nodes[title] = append(nodes[title], x)
		})
	})
}

func href(doc *goquery.Document, n *html.Node) string {
	for _, a := range n.Attr {
		if a.Key == "href" {
			u, err := url.Parse(a.Val)
			if err != nil {
				return ""
			}

			return doc.Url.ResolveReference(u).String()
		}
	}
	return ""
}

func parse(req *http.Request) (*goquery.Document, error) {
	r, err := fetch(req)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(r)
	io.Copy(ioutil.Discard, r)
	r.Close()

	if err != nil {
		return nil, err
	}

	doc.Url = req.URL
	return doc, nil
}

func fetch(req *http.Request) (io.ReadCloser, error) {
	fname := filepath.Join(".cache", req.Method, req.URL.String())
	fname = strings.Map(func(r rune) rune {
		switch r {
		case ':', '?':
			return '_'
		default:
			return r
		}
	}, fname)
	f, err := os.Open(fname)
	if err == nil {
		return f, nil
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	return TeeFileReader(res.Body, fname)
}

// downloadPackage returns the bytes for the outermost zip archive for a
// package. dlPageURL is the URL that is linked on
// http://www.world-of-ai.com/packages.php.
func downloadPackage(dlPageURL string) (b []byte, dlID string, err error) {
	req, err := http.NewRequest("GET", dlPageURL, nil)
	if err != nil {
		log.Fatal(err)
	}

	doc, err := parse(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch download page: %v", err)
	}

	nodes := doc.Find("a[href^='download.php?DLID=']").First().Nodes
	if len(nodes) < 1 {
		return nil, "", fmt.Errorf("interstitial link not found: %v", dlPageURL)
	}

	interstitialURL, err := url.Parse(href(doc, nodes[0]))
	if err != nil {
		return nil, "", fmt.Errorf("cannot parse interstitial URL: %v", err)
	}

	dlID = interstitialURL.Query().Get("DLID")
	if dlID == "" {
		return nil, "", fmt.Errorf("empty DLID in interstitial URL: %v", interstitialURL)
	}

	dlQuery := make(url.Values)
	dlQuery.Set("Location", "AVSIM")
	dlQuery.Set("Proto", "ftp") // wat?
	dlQuery.Set("DLID", dlID)

	req, err = http.NewRequest("GET", "https://library.avsim.net/sendfile.php?"+dlQuery.Encode(), nil)
	if err != nil {
		log.Fatal(err)
	}

	cookie := make(url.Values)
	loginToken := os.Getenv("AVSIM_LOGIN")
	if loginToken == "" {
		log.Fatal("AVSIM_LOGIN not set")
	}
	cookie.Set("LibraryLogin", loginToken)
	req.Header.Set("Cookie", cookie.Encode())

	r, err := fetch(req)
	if err != nil {
		return nil, "", fmt.Errorf("cannot download %s: %v", req.URL.String(), err)
	}
	defer r.Close()

	b, err = ioutil.ReadAll(r)
	if err != nil {
		return nil, "", fmt.Errorf("read response: %v", err)
	}

	return b, dlID, nil
}
