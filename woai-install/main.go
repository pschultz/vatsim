package main

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/hex"
	"errors"
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

	"golang.org/x/net/html"

	"github.com/PuerkitoBio/goquery"
)

var FSXRoot = "/mnt/c/Program Files (x86)/Steam/steamapps/common/FSX"

func main() {
	if d := os.Getenv("FSX_ROOT"); d != "" {
		FSXRoot = d
	}

	flag.Parse()
	term := ".*"
	if args := flag.Args(); len(args) > 0 {
		term = args[0]
	}

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
		fmt.Println(title)
		for _, n := range nodes[title] {
			b, dlID, err := downloadPackage(href(doc, n))
			if err != nil {
				fmt.Println("\t", err)
				continue
			}

			if err := installPackage(b, dlID); err != nil {
				fmt.Println("\t", err)
				continue
			}
			break
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
		title := s.Find("td").Eq(1).Text()
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
	log.Println("cache miss ", req.URL.String())

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
		return nil, "", fmt.Errorf("Fetch download page: %v", err)
	}

	nodes := doc.Find("a[href^='download.php?DLID=']").First().Nodes
	if len(nodes) < 1 {
		return nil, "", fmt.Errorf("Interstitial link not found: %v", dlPageURL)
	}

	interstitialURL, err := url.Parse(href(doc, nodes[0]))
	if err != nil {
		return nil, "", fmt.Errorf("Cannot parse interstitial URL: %v", err)
	}

	dlID = interstitialURL.Query().Get("DLID")
	if dlID == "" {
		return nil, "", fmt.Errorf("Empty DLID in interstitial URL: %v", interstitialURL)
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
		return nil, "", fmt.Errorf("Cannot download %s: %v\n", dlQuery.Encode(), err)
	}
	defer r.Close()

	b, err = ioutil.ReadAll(r)
	if err != nil {
		return nil, "", fmt.Errorf("Cannot read %s: %v\n", dlID, err)
	}

	return b, dlID, nil
}

// installPackage installs a single World of AI Package. raw are the bytes of
// the outermost zip archive, as returned by downloadPackage.
func installPackage(raw []byte, dlID string) error {
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fmt.Errorf("Cannot read zip %s: %v\n", dlID, err)
	}

	raw, err = unzip(z, ".woai.zip")
	if err != nil {
		return err
	}

	z, err = zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fmt.Errorf("Cannot read %s.woai.zip: %v\n", dlID, err)
	}

	raw, err = unzip(z, ".woai.enc")
	if err != nil {
		return err
	}

	if err := decrypt(raw); err != nil {
		log.Fatal(err) // should never happen
	}

	z, err = zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fmt.Errorf("Cannot read %s.woai.enc: %v\n", dlID, err)
	}

	hash := sha1.New()
	hash.Write(raw)
	digest := hex.EncodeToString(hash.Sum(nil))
	created := make(map[string]bool) // set of already created directories, so we can skip MkdirAll.

	for _, zf := range z.File {
		name := filepath.Clean(strings.Replace(zf.Name, `\`, "/", -1))
		iname := strings.ToLower(name)
		dest := name

		switch {
		case strings.HasPrefix(iname, "aircraft/"):
			// Change dest such that we can write to FSXRoot/dest.
			//
			// dest is something like "aircraft/WoA_AIA_B733v2_Winglet/Aircraft.cfg".
			// Replace the first segment with "SimObjects/Airplanes" and prepend
			// part of the ZIP hash to the second segment (they are not unique
			// across packages).
			dest = strings.TrimPrefix(dest, "aircraft/")
			dest = digest[:8] + "-" + dest
			dest = filepath.Join("SimObjects/Airplanes", dest)

		case strings.HasPrefix(iname, "texture/"),
			strings.HasPrefix(iname, "scenery/"),
			strings.HasPrefix(iname, "effects/"):
			// Not sure if there are collisions here, but it probably doesn't
			// matter much if there are.

		case strings.HasSuffix(iname, ".txt"),
			strings.HasPrefix(iname, "addon scenery/"),
			iname == "avsim.diz",
			iname == "woai.cfg",
			iname == "version.ini":
			// ignore
			continue
		default:
			log.Println("skipping ", name)
			continue
		}

		if strings.HasSuffix(dest, "Aircraft.cfg") {
			dest = strings.TrimSuffix(dest, "Aircraft.cfg") + "aircraft.cfg"
		}
		fname := filepath.Join(FSXRoot, dest)

		r, err := zf.Open()
		if err != nil {
			return fmt.Errorf("Cannot open %s.woai.enc/%s: %v\n", dlID, name, err)
		}

		dir := filepath.Dir(fname)
		if !created[dir] {
			if err := os.MkdirAll(filepath.Dir(fname), 0755); err != nil {
				return err
			}
			created[dir] = true
		}

		f, err := os.Create(fname)
		if err != nil {
			r.Close()
			return fmt.Errorf("Cannot create %s: %v\n", fname, err)
		}
		_, err = io.Copy(f, r)
		r.Close()
		f.Close()
		if err != nil {
			return fmt.Errorf("Cannot extract %s: %v\n", name, err)
		}
		fmt.Println("\t", fname)
	}

	return nil
}

func unzip(z *zip.Reader, fnameSuffix string) ([]byte, error) {
	for _, f := range z.File {
		if !strings.HasSuffix(f.Name, fnameSuffix) {
			continue
		}

		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer r.Close()

		return ioutil.ReadAll(r)
	}

	return nil, errors.New("zip: not found: " + fnameSuffix)
}

func decrypt(b []byte) error {
	key := []byte{
		0xf8, 0x93, 0xab, 0xdd, 0xf3, 0x9b, 0x02, 0x6d,
		0x5d, 0x13, 0x5a, 0x61, 0xfe, 0xcb, 0x91, 0x0f,
		0xe0, 0x69, 0x0a, 0x47, 0xe7, 0xd5, 0x91, 0x83,
		0x9d, 0xdf, 0xc9, 0x70, 0x03, 0x05, 0x3c, 0x5c,
	}
	iv := []byte{
		0x27, 0xc9, 0x1e, 0x10, 0x6f, 0x27, 0x9e, 0xd0,
		0x4d, 0x64, 0xd4, 0x19, 0xcb, 0x74, 0x19, 0xb0,
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	dec := cipher.NewCBCDecrypter(block, iv)
	dec.CryptBlocks(b, b)

	return nil
}
