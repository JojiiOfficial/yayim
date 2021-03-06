package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/leonelquinteros/gotext"
	rpc "github.com/mikkeloscar/aur"

	alpm "github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/intrange"
	"github.com/Jguer/yay/v10/pkg/multierror"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
)

const arrow = "==>"
const smallArrow = " ->"

func print(warnings *query.AURWarnings) {
	if len(warnings.Missing) > 0 {
		text.Warn(gotext.Get("Missing AUR Packages:"))
		for _, name := range warnings.Missing {
			fmt.Print("  " + cyan(name))
		}
		fmt.Println()
	}

	if len(warnings.Orphans) > 0 {
		text.Warn(gotext.Get("Orphaned AUR Packages:"))
		for _, name := range warnings.Orphans {
			fmt.Print("  " + cyan(name))
		}
		fmt.Println()
	}

	if len(warnings.OutOfDate) > 0 {
		text.Warn(gotext.Get("Flagged Out Of Date AUR Packages:"))
		for _, name := range warnings.OutOfDate {
			fmt.Print("  " + cyan(name))
		}
		fmt.Println()
	}
}

// PrintSearch handles printing search results in a given format
func (q aurQuery) printSearch(start int, alpmHandle *alpm.Handle) {
	localDB, _ := alpmHandle.LocalDB()

	for i := range q {
		var toprint string
		if config.SearchMode == numberMenu {
			switch config.SortMode {
			case settings.TopDown:
				toprint += magenta(strconv.Itoa(start+i) + " ")
			case settings.BottomUp:
				toprint += magenta(strconv.Itoa(len(q)+start-i-1) + " ")
			default:
				text.Warnln(gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save"))
			}
		} else if config.SearchMode == minimal {
			fmt.Println(q[i].Name)
			continue
		}

		toprint += bold(text.ColorHash("aur")) + "/" + bold(q[i].Name) +
			" " + cyan(q[i].Version) +
			bold(" (+"+strconv.Itoa(q[i].NumVotes)) +
			" " + bold(strconv.FormatFloat(q[i].Popularity, 'f', 2, 64)+") ")

		if q[i].Maintainer == "" {
			toprint += bold(red(gotext.Get("(Orphaned)"))) + " "
		}

		if q[i].OutOfDate != 0 {
			toprint += bold(red(gotext.Get("(Out-of-date: %s)", text.FormatTime(q[i].OutOfDate)))) + " "
		}

		if pkg := localDB.Pkg(q[i].Name); pkg != nil {
			if pkg.Version() != q[i].Version {
				toprint += bold(green(gotext.Get("(Installed: %s)", pkg.Version())))
			} else {
				toprint += bold(green(gotext.Get("(Installed)")))
			}
		}
		toprint += "\n    " + q[i].Description
		fmt.Println(toprint)
	}
}

func formatAur(pkg *rpc.Pkg, i int, localDB *alpm.DB) string {
	toprint := magenta(strconv.Itoa(i+1)+" ") +
		bold(
			text.ColorHash("aur"),
		) + "/" +
		bold(pkg.Name) + " " +
		cyan(pkg.Version) +
		bold(
			" (+"+strconv.Itoa(pkg.NumVotes),
		) + " " +
		bold(
			strconv.FormatFloat(pkg.Popularity, 'f', 2, 64)+"%) ",
		)

	if pkg.Maintainer == "" {
		toprint += bold(red("(Orphaned)")) + " "
	}

	if pkg.OutOfDate != 0 {
		toprint += bold(red("(Out-of-date "+text.FormatTime(pkg.OutOfDate)+")")) + " "
	}

	if p := localDB.Pkg(pkg.Name); p != nil {
		if p.Version() != pkg.Version {
			toprint += bold(green("(Installed: " + p.Version() + ")"))
		} else {
			toprint += bold(green("(Installed)"))
		}
	}
	toprint += "\n    " + pkg.Description
	return toprint
}

// PrintSearch receives a RepoSearch type and outputs pretty text.
func (s repoQuery) printSearch(alpmHandle *alpm.Handle) {
	for i, res := range s {
		var toprint string
		if config.SearchMode == numberMenu {
			switch config.SortMode {
			case settings.TopDown:
				toprint += magenta(strconv.Itoa(i+1) + " ")
			case settings.BottomUp:
				toprint += magenta(strconv.Itoa(len(s)-i) + " ")
			default:
				text.Warnln(gotext.Get("invalid sort mode. Fix with yay -Y --bottomup --save"))
			}
		} else if config.SearchMode == minimal {
			fmt.Println(res.Name())
			continue
		}

		toprint += bold(text.ColorHash(res.DB().Name())) + "/" + bold(res.Name()) +
			" " + cyan(res.Version()) +
			bold(" ("+text.Human(res.Size())+
				" "+text.Human(res.ISize())+") ")

		if len(res.Groups().Slice()) != 0 {
			toprint += fmt.Sprint(res.Groups().Slice(), " ")
		}

		localDB, err := alpmHandle.LocalDB()
		if err == nil {
			if pkg := localDB.Pkg(res.Name()); pkg != nil {
				if pkg.Version() != res.Version() {
					toprint += bold(green(gotext.Get("(Installed: %s)", pkg.Version())))
				} else {
					toprint += bold(green(gotext.Get("(Installed)")))
				}
			}
		}

		toprint += "\n    " + res.Description()
		fmt.Println(toprint)
	}
}

// PrintSearch receives a RepoSearch type and outputs pretty text.
func format(res *alpm.Package, alpmHandle *alpm.Handle, i int) string {
	toprint := magenta(strconv.Itoa(i+1)+" ") +
		bold(
			text.ColorHash(res.DB().Name()),
		) + "/" +
		bold(res.Name()) + " " +
		cyan(res.Version()) +
		bold(
			" ("+
				text.Human(res.Size())+" "+
				text.Human(res.ISize())+") ",
		)

	if len(res.Groups().Slice()) != 0 {
		toprint += fmt.Sprint(res.Groups().Slice(), " ")
	}

	localDB, err := alpmHandle.LocalDB()
	if err == nil {
		if pkg := localDB.Pkg(res.Name()); pkg != nil {
			if pkg.Version() != res.Version() {
				toprint += bold(green("(Installed: " + pkg.Version() + ")"))
			} else {
				toprint += bold(green("(Installed)"))
			}
		}
	}

	toprint += "\n    " + res.Description()
	return toprint
}

// Pretty print a set of packages from the same package base.

func (u *upgrade) StylizedNameWithRepository() string {
	return bold(text.ColorHash(u.Repository)) + "/" + bold(u.Name)
}

// Print prints the details of the packages to upgrade.
func (u upSlice) print() {
	longestName, longestVersion := 0, 0
	for _, pack := range u {
		packNameLen := len(pack.StylizedNameWithRepository())
		packVersion, _ := getVersionDiff(pack.LocalVersion, pack.RemoteVersion)
		packVersionLen := len(packVersion)
		longestName = intrange.Max(packNameLen, longestName)
		longestVersion = intrange.Max(packVersionLen, longestVersion)
	}

	namePadding := fmt.Sprintf("%%-%ds  ", longestName)
	versionPadding := fmt.Sprintf("%%-%ds", longestVersion)
	numberPadding := fmt.Sprintf("%%%dd  ", len(fmt.Sprintf("%v", len(u))))

	for k, i := range u {
		left, right := getVersionDiff(i.LocalVersion, i.RemoteVersion)

		fmt.Print(magenta(fmt.Sprintf(numberPadding, len(u)-k)))

		fmt.Printf(namePadding, i.StylizedNameWithRepository())

		fmt.Printf("%s -> %s\n", fmt.Sprintf(versionPadding, left), right)
	}
}

// Print prints repository packages to be downloaded
// func (do *depOrder) Print() {
// 	repo := ""
// 	repoMake := ""
// 	aur := ""
// 	aurMake := ""

// 	repoLen := 0
// 	repoMakeLen := 0
// 	aurLen := 0
// 	aurMakeLen := 0

// 	for _, pkg := range do.Repo {
// 		if do.Runtime.Get(pkg.Name()) {
// 			repo += "  " + pkg.Name() + "-" + pkg.Version()
// 			repoLen++
// 		} else {
// 			repoMake += "  " + pkg.Name() + "-" + pkg.Version()
// 			repoMakeLen++
// 		}
// 	}

// 	for _, base := range do.Aur {
// 		pkg := base.Pkgbase()
// 		pkgStr := "  " + pkg + "-" + base[0].Version
// 		pkgStrMake := pkgStr

// 		push := false
// 		pushMake := false

// 		switch {
// 		case len(base) > 1, pkg != base[0].Name:
// 			pkgStr += " ("
// 			pkgStrMake += " ("

// 			for _, split := range base {
// 				if do.Runtime.Get(split.Name) {
// 					pkgStr += split.Name + " "
// 					aurLen++
// 					push = true
// 				} else {
// 					pkgStrMake += split.Name + " "
// 					aurMakeLen++
// 					pushMake = true
// 				}
// 			}

// 			pkgStr = pkgStr[:len(pkgStr)-1] + ")"
// 			pkgStrMake = pkgStrMake[:len(pkgStrMake)-1] + ")"
// 		case do.Runtime.Get(base[0].Name):
// 			aurLen++
// 			push = true
// 		default:
// 			aurMakeLen++
// 			pushMake = true
// 		}

// 		if push {
// 			aur += pkgStr
// 		}
// 		if pushMake {
// 			aurMake += pkgStrMake
// 		}
// 	}

// 	printDownloads("Repo", repoLen, 9, repo)
// 	printDownloads("Repo Make", repoMakeLen, 9, repoMake)
// 	printDownloads("Aur", aurLen, 9, aur)
// 	printDownloads("Aur Make", aurMakeLen, 9, aurMake)
// }

func printDownloads(repoName string, length, padd int, packages string) {
	packages = strings.TrimSpace(packages)
	if length < 1 {
		return
	}

	repoInfo := bold(blue("[" + repoName + ": " + strconv.Itoa(length) + "]"))

	var padding string
	// Create padding for aligning
	if padd != 0 {
		paddMount := int(padd / 8)
		padding = strings.Repeat("\t", paddMount)
	} else {
		padding = " "
	}

	fmt.Println(repoInfo + padding + cyan(packages))
}

// PrintInfo prints package info like pacman -Si.
func PrintInfo(a *rpc.Pkg, extendedInfo bool) {
	text.PrintInfoValue(gotext.Get("Repository"), "aur")
	text.PrintInfoValue(gotext.Get("Name"), a.Name)
	if len(a.Keywords) > 0 {
		text.PrintInfoValue(gotext.Get("Keywords"), strings.Join(a.Keywords, "  "))
	}

	text.PrintInfoValue(gotext.Get("Version"), a.Version)
	text.PrintInfoValue(gotext.Get("Description"), a.Description)
	text.PrintInfoValue(gotext.Get("URL"), a.URL)
	text.PrintInfoValue(gotext.Get("AUR URL"), config.AURURL+"/packages/"+a.Name)

	if len(a.Groups) > 0 {
		text.PrintInfoValue(gotext.Get("Groups"), strings.Join(a.Groups, "  "))
	}

	if len(a.URL) > 0 {
		u, err := url.Parse(a.URL)
		if err == nil && u.Host == "github.com" {
			lang, err := getLanguagesFromRepo(a.URL)
			if err == nil {
				add := ""
				if len(lang) > 1 {
					add = "s"
				}
				text.PrintInfoValue(gotext.Get("Language"+add), langsToOneliner(parseLanguages(lang), "%"))
			}
		}
	}

	text.PrintInfoValue(gotext.Get("Licenses"), strings.Join(a.License, "  "))

	if len(a.Provides) > 0 {
		text.PrintInfoValue(gotext.Get("Provides"), strings.Join(a.Provides, "  "))
	}

	if len(a.Depends) > 0 {
		text.PrintInfoValue(gotext.Get("Depends On"), strings.Join(a.Depends, "  "))
	}

	if len(a.MakeDepends) > 0 {
		text.PrintInfoValue(gotext.Get("Make Deps"), strings.Join(a.MakeDepends, "  "))
	}

	if len(a.CheckDepends) > 0 {
		text.PrintInfoValue(gotext.Get("Check Deps"), strings.Join(a.CheckDepends, "  "))
	}

	if len(a.OptDepends) > 0 {
		text.PrintInfoValue(gotext.Get("Optional Deps"), strings.Join(a.OptDepends, "  "))
	}

	if len(a.Conflicts) > 0 {
		text.PrintInfoValue(gotext.Get("Conflicts With"), strings.Join(a.Conflicts, "  "))
	}

	text.PrintInfoValue(gotext.Get("Maintainer"), a.Maintainer)
	text.PrintInfoValue(gotext.Get("Votes"), fmt.Sprintf("%d", a.NumVotes))
	text.PrintInfoValue(gotext.Get("Popularity"), fmt.Sprintf("%.2f", a.Popularity))
	text.PrintInfoValue(gotext.Get("First Submitted"), text.FormatTimeQuery(a.FirstSubmitted))
	text.PrintInfoValue(gotext.Get("Last Modified"), text.FormatTimeQuery(a.LastModified))

	if a.OutOfDate != 0 {
		text.PrintInfoValue(gotext.Get("Out-of-date"), text.FormatTimeQuery(a.OutOfDate))
	} else {
		text.PrintInfoValue(gotext.Get("Out-of-date"), "No")
	}

	if extendedInfo {
		text.PrintInfoValue("ID", fmt.Sprintf("%d", a.ID))
		text.PrintInfoValue(gotext.Get("Package Base ID"), fmt.Sprintf("%d", a.PackageBaseID))
		text.PrintInfoValue(gotext.Get("Package Base"), a.PackageBase)
		text.PrintInfoValue(gotext.Get("Snapshot URL"), config.AURURL+a.URLPath)
	}

	fmt.Println()
}

// BiggestPackages prints the name of the ten biggest packages in the system.
func biggestPackages(alpmHandle *alpm.Handle) {
	localDB, err := alpmHandle.LocalDB()
	if err != nil {
		return
	}

	pkgCache := localDB.PkgCache()
	pkgS := pkgCache.SortBySize().Slice()

	if len(pkgS) < 10 {
		return
	}

	for i := 0; i < 10; i++ {
		fmt.Printf("%s: %s\n", bold(pkgS[i].Name()), cyan(text.Human(pkgS[i].ISize())))
	}
	// Could implement size here as well, but we just want the general idea
}

// localStatistics prints installed packages statistics.
func localStatistics(alpmHandle *alpm.Handle) error {
	info, err := statistics(alpmHandle)
	if err != nil {
		return err
	}

	_, remoteNames, err := query.GetPackageNamesBySource(alpmHandle)
	if err != nil {
		return err
	}

	text.Infoln(gotext.Get("Yay version v%s", yayVersion))
	fmt.Println(bold(cyan("===========================================")))
	text.Infoln(gotext.Get("Total installed packages: %s", cyan(strconv.Itoa(info.Totaln))))
	text.Infoln(gotext.Get("Total foreign installed packages: %s", cyan(strconv.Itoa(len(remoteNames)))))
	text.Infoln(gotext.Get("Explicitly installed packages: %s", cyan(strconv.Itoa(info.Expln))))
	text.Infoln(gotext.Get("Total Size occupied by packages: %s", cyan(text.Human(info.TotalSize))))
	fmt.Println(bold(cyan("===========================================")))
	text.Infoln(gotext.Get("Ten biggest packages:"))
	biggestPackages(alpmHandle)
	fmt.Println(bold(cyan("===========================================")))

	query.AURInfoPrint(remoteNames, config.RequestSplitN)

	return nil
}

// TODO: Make it less hacky
func printNumberOfUpdates(alpmHandle *alpm.Handle, enableDowngrade bool) error {
	warnings := query.NewWarnings()
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	aurUp, repoUp, err := upList(warnings, alpmHandle, enableDowngrade)
	os.Stdout = old // restoring the real stdout
	if err != nil {
		return err
	}
	fmt.Println(len(aurUp) + len(repoUp))

	return nil
}

// TODO: Make it less hacky
func printUpdateList(cmdArgs *settings.Arguments, alpmHandle *alpm.Handle, enableDowngrade bool) error {
	targets := stringset.FromSlice(cmdArgs.Targets)
	warnings := query.NewWarnings()
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	localNames, remoteNames, err := query.GetPackageNamesBySource(alpmHandle)
	if err != nil {
		return err
	}

	aurUp, repoUp, err := upList(warnings, alpmHandle, enableDowngrade)
	os.Stdout = old // restoring the real stdout
	if err != nil {
		return err
	}

	noTargets := len(targets) == 0

	if !cmdArgs.ExistsArg("m", "foreign") {
		for _, pkg := range repoUp {
			if noTargets || targets.Get(pkg.Name) {
				if cmdArgs.ExistsArg("q", "quiet") {
					fmt.Printf("%s\n", pkg.Name)
				} else {
					fmt.Printf("%s %s -> %s\n", bold(pkg.Name), green(pkg.LocalVersion), green(pkg.RemoteVersion))
				}
				delete(targets, pkg.Name)
			}
		}
	}

	if !cmdArgs.ExistsArg("n", "native") {
		for _, pkg := range aurUp {
			if noTargets || targets.Get(pkg.Name) {
				if cmdArgs.ExistsArg("q", "quiet") {
					fmt.Printf("%s\n", pkg.Name)
				} else {
					fmt.Printf("%s %s -> %s\n", bold(pkg.Name), green(pkg.LocalVersion), green(pkg.RemoteVersion))
				}
				delete(targets, pkg.Name)
			}
		}
	}

	missing := false

outer:
	for pkg := range targets {
		for _, name := range localNames {
			if name == pkg {
				continue outer
			}
		}

		for _, name := range remoteNames {
			if name == pkg {
				continue outer
			}
		}

		text.Errorln(gotext.Get("package '%s' was not found", pkg))
		missing = true
	}

	if missing {
		return fmt.Errorf("")
	}

	return nil
}

type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Creator     string `xml:"dc:creator"`
}

type channel struct {
	Title         string `xml:"title"`
	Link          string `xml:"link"`
	Description   string `xml:"description"`
	Language      string `xml:"language"`
	Lastbuilddate string `xml:"lastbuilddate"`
	Items         []item `xml:"item"`
}

type rss struct {
	Channel channel `xml:"channel"`
}

const (
	redCode     = "\x1b[31m"
	greenCode   = "\x1b[32m"
	blueCode    = "\x1b[34m"
	magentaCode = "\x1b[35m"
	cyanCode    = "\x1b[36m"
	boldCode    = "\x1b[1m"

	resetCode = "\x1b[0m"
)

func stylize(startCode, in string) string {
	if text.UseColor {
		return startCode + in + resetCode
	}

	return in
}

func red(in string) string {
	return stylize(redCode, in)
}

func green(in string) string {
	return stylize(greenCode, in)
}

func blue(in string) string {
	return stylize(blueCode, in)
}

func cyan(in string) string {
	return stylize(cyanCode, in)
}

func magenta(in string) string {
	return stylize(magentaCode, in)
}

func bold(in string) string {
	return stylize(boldCode, in)
}

func printPkgbuilds(pkgS []string, alpmHandle *alpm.Handle) error {
	var pkgbuilds []string
	var localPkgbuilds []string
	missing := false
	pkgS = query.RemoveInvalidTargets(pkgS, settings.ModeAny)
	aurS, repoS, err := packageSlices(pkgS, alpmHandle)
	if err != nil {
		return err
	}
	var errs multierror.MultiError

	if len(aurS) != 0 {
		noDB := make([]string, 0, len(aurS))
		for _, pkg := range aurS {
			_, name := text.SplitDBFromName(pkg)
			noDB = append(noDB, name)
		}
		localPkgbuilds, err = aurPkgbuilds(noDB)
		pkgbuilds = append(pkgbuilds, localPkgbuilds...)
		errs.Add(err)
	}

	if len(repoS) != 0 {
		localPkgbuilds, err = repoPkgbuilds(repoS, alpmHandle)
		pkgbuilds = append(pkgbuilds, localPkgbuilds...)
		errs.Add(err)
	}

	if err = errs.Return(); err != nil {
		missing = true
		fmt.Fprintln(os.Stderr, err)
	}

	if len(aurS) != len(pkgbuilds) {
		missing = true
	}

	if len(pkgbuilds) != 0 {
		for _, pkgbuild := range pkgbuilds {
			fmt.Print(pkgbuild)
		}
	}

	if missing {
		err = fmt.Errorf("")
	}

	return err
}
