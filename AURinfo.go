package main

import (
	"net/url"
	"strings"

	"github.com/JojiiOfficial/gaw"
	gosrc "github.com/Morganamilo/go-srcinfo"
)

// AURinfo info for aur packages
type AURinfo struct {
	SrcInfos map[string]*gosrc.Srcinfo
}

// NewAURInfo create new AURinfo
func NewAURInfo(srcInfos map[string]*gosrc.Srcinfo) *AURinfo {
	return &AURinfo{
		SrcInfos: srcInfos,
	}
}

// GetLanguages used in SrcInfos
func (aurInfo *AURinfo) GetLanguages() (string, int, error) {
	return getLangsFromSourceinfos(aurInfo.SrcInfos)
}

// GetSourceList lists used sources
func (aurInfo *AURinfo) GetSourceList() (map[string][]string, uint32) {
	var count uint32
	sources := make(map[string][]string, 0)

	for _, si := range aurInfo.SrcInfos {
		for _, source := range si.Source {
			if len(source.Value) == 0 {
				continue
			}

			src := filterSrc(source.Value)
			sourceType := getSourceType(src, toURL(src))

			arr, ok := sources[sourceType]
			if ok {
				sources[sourceType] = append(arr, src)
			} else {
				sources[sourceType] = []string{src}
			}

			count++
		}
	}

	return sources, count
}

func getSourceType(source string, u *url.URL) string {
	if len(u.Scheme) > 0 && gaw.IsInStringArray(u.Scheme, []string{"https", "http"}) {
		switch u.Hostname() {
		case "github.com":
			return "Github"
		case "gitlab.com":
			return "Gitlab"
		}
	}

	if strings.Contains(source, ".") && len(source) >= 3 {
		// If source ends with a '.' but
		// has another '.' in its name
		// cut the last '.' off
		if strings.HasSuffix(source, ".") &&
			strings.Contains(source[:len(source)-1], ".") {

			source = source[:len(source)-1]
		}

		ending := source[strings.LastIndex(source, ".")+1:]
		switch ending {
		case "patch":
			return "Patch file"
		case "desktop":
			return "Desktop file"
		case "xz":
			return "xz archive"
		case "gz":
			return "gz archive"
		case "diff":
			return "Diff file"
		case "sig":
			return "Signature"
		}
	}

	return "other"
}
