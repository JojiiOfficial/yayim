package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/google/go-github/v32/github"
)

// Return user and repository by repo url
func ownerInfoFromRepo(repoURL string) (string, string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", "", err
	}

	p := u.Path
	if strings.HasPrefix(p, "/") {
		p = p[1:]
	}
	if strings.HasSuffix(p, ".git") {
		p = p[:len(p)-4]
	}

	s := strings.Split(p, "/")

	return s[0], s[1], nil
}

func getLanguagesFromRepo(repo string) (map[string]int, error) {
	// Create github client
	client := github.NewClient(nil)

	// Parse repo path
	user, repo, err := ownerInfoFromRepo(repo)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// Use 4s as timeout
	ctx, cncl := context.WithTimeout(context.Background(), 4*time.Second)
	defer cncl()

	// Do request
	languages, _, err := client.Repositories.ListLanguages(ctx, user, repo)
	return languages, err
}

// Filter important languages
func parseLanguages(languages map[string]int) map[string]int {
	var allBytes uint
	var percent, usedPercent int
	langs := make(map[string]int)

	// Calculate all bytes
	for _, bs := range languages {
		allBytes += uint(bs)
	}

	// Calculate percent of each language
	// with a use of more than 10%
	for lang, bs := range languages {
		if uint(bs) > uint(0.1*float64(allBytes)) {
			percent = int(uint(bs) * 100 / allBytes)
			usedPercent += percent
			langs[lang] = percent
		}
	}

	// Fill to 100
	if usedPercent < 100 {
		langs["Other"] = 100 - usedPercent
	}

	return langs
}

const langParseFormat = "%d%s %s;"

type langErr struct {
	languages map[string]int
	err       error
}

func getLangsFromSourceinfos(srcInfos map[string]*gosrc.Srcinfo) (string, int, error) {
	var sLangs string
	langs := make(map[string]int)

	c := make(chan langErr, 1)

	go func() {
		wg := sync.WaitGroup{}

		// Loop all packages
		for _, si := range srcInfos {
			// Loop sources
			for _, sourceURL := range si.Source {
				u := toURL(filterSrc(sourceURL.Value))

				if u == nil {
					continue
				}

				// Filter github
				if u.Hostname() != "github.com" {
					continue
				}

				wg.Add(1)

				// Do requests concurrent
				go func() {
					// Request github API
					lang, err := getLanguagesFromRepo(u.String())
					c <- langErr{
						languages: lang,
						err:       err,
					}
					wg.Done()
				}()
			}
		}

		// Wait for all requests
		wg.Wait()

		close(c)
	}()

	// Loop results and add
	// them to 'lang'
	for le := range c {
		if le.err != nil {
			return "", 0, le.err
		}

		ls := parseLanguages(le.languages)

		// Add languages to langs list
		for lang := range ls {
			val, ok := langs[lang]
			if ok {
				langs[lang] = val + 1
			} else {
				langs[lang] = 1
			}
		}
	}

	sLangs = langsToOneliner(langs, "x")
	return sLangs, len(langs), nil
}

func langsToOneliner(langs map[string]int, unit string) string {
	var sLangs string

	// Format to one liner
	for lang, count := range langs {
		// We want to add 'other' at the end
		if strings.ToLower(lang) == "other" {
			continue
		}

		if len(sLangs) != 0 {
			sLangs += " "
		}

		if count == 1 {
			sLangs += lang + ";"
		} else {
			sLangs += fmt.Sprintf(langParseFormat, count, unit, lang)
		}
	}

	// Append 'other' or remove last ';'
	if val, ok := langs["Other"]; ok {
		if !strings.HasSuffix(sLangs, " ") {
			sLangs += " "
		}

		if val == 1 {
			sLangs += "Other"
		} else {
			sLangs += fmt.Sprintf(langParseFormat, val, unit, "Other")
		}
	} else {
		if strings.HasSuffix(sLangs, ";") {
			sLangs = sLangs[:len(sLangs)-1]
		}
	}

	return sLangs
}

// Start after given substring
func startAtSubstring(s string, substring string) string {
	if !strings.Contains(s, substring) {
		return s
	}

	return s[strings.LastIndex(s, substring)+len(substring):]
}

// Remove some prefixes used by PKGBUILDS
func filterSrc(srcURL string) string {
	src := startAtSubstring(srcURL, "::")
	src = startAtSubstring(src, "+")
	return src
}

func toURL(src string) *url.URL {
	// Parse src url
	u, err := url.Parse(src)
	if err != nil {
		return &url.URL{
			Host: src,
		}
	}

	return u
}
