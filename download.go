package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	alpm "github.com/Jguer/go-alpm"
	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"

	"github.com/Jguer/yay/v10/pkg/dep"
	"github.com/Jguer/yay/v10/pkg/multierror"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/text"
)

const gitDiffRefName = "AUR_SEEN"

// Update the YAY_DIFF_REVIEW ref to HEAD. We use this ref to determine which diff were
// reviewed by the user
func gitUpdateSeenRef(path, name string) error {
	_, stderr, err := capture(passToGit(filepath.Join(path, name), "update-ref", gitDiffRefName, "HEAD"))
	if err != nil {
		return fmt.Errorf("%s %s", stderr, err)
	}
	return nil
}

// Return wether or not we have reviewed a diff yet. It checks for the existence of
// YAY_DIFF_REVIEW in the git ref-list
func gitHasLastSeenRef(path, name string) bool {
	_, _, err := capture(passToGit(filepath.Join(path, name), "rev-parse", "--quiet", "--verify", gitDiffRefName))
	return err == nil
}

// Returns the last reviewed hash. If YAY_DIFF_REVIEW exists it will return this hash.
// If it does not it will return empty tree as no diff have been reviewed yet.
func getLastSeenHash(path, name string) (string, error) {
	if gitHasLastSeenRef(path, name) {
		stdout, stderr, err := capture(passToGit(filepath.Join(path, name), "rev-parse", gitDiffRefName))
		if err != nil {
			return "", fmt.Errorf("%s %s", stderr, err)
		}

		lines := strings.Split(stdout, "\n")
		return lines[0], nil
	}
	return gitEmptyTree, nil
}

// Check whether or not a diff exists between the last reviewed diff and
// HEAD@{upstream}
func gitHasDiff(path, name string) (bool, error) {
	if gitHasLastSeenRef(path, name) {
		stdout, stderr, err := capture(passToGit(filepath.Join(path, name), "rev-parse", gitDiffRefName, "HEAD@{upstream}"))
		if err != nil {
			return false, fmt.Errorf("%s%s", stderr, err)
		}

		lines := strings.Split(stdout, "\n")
		lastseen := lines[0]
		upstream := lines[1]
		return lastseen != upstream, nil
	}
	// If YAY_DIFF_REVIEW does not exists, we have never reviewed a diff for this package
	// and should display it.
	return true, nil
}

// TODO: yay-next passes args through the header, use that to unify ABS and AUR
func gitDownloadABS(info pkgInfo, path, name string) (bool, error) {
	if err := os.MkdirAll(path, 0700); err != nil {
		return false, err
	}

	if _, errExist := os.Stat(filepath.Join(path, name)); os.IsNotExist(errExist) {
		a := []string{"clone", "--no-progress", "--single-branch"}
		if info.customBranch {
			a = append(a, []string{"-b", "packages/" + name}...)
		}
		a = append(a, []string{info.url, name}...)

		cmd := passToGit(path, a...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		_, stderr, err := capture(cmd)
		if err != nil {
			return false, fmt.Errorf(gotext.Get("error cloning %s: %s", name, stderr))
		}

		return true, nil
	} else if errExist != nil {
		return false, fmt.Errorf(gotext.Get("error reading %s", filepath.Join(path, name, ".git")))
	}

	cmd := passToGit(filepath.Join(path, name), "pull", "--ff-only")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	_, stderr, err := capture(cmd)
	if err != nil {
		return false, fmt.Errorf(gotext.Get("error fetching %s: %s", name, stderr))
	}

	return true, nil
}

func gitDownload(url, path, name string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, name, ".git"))
	if os.IsNotExist(err) {
		cmd := passToGit(path, "clone", "--no-progress", url, name)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		_, stderr, errCapture := capture(cmd)
		if errCapture != nil {
			return false, fmt.Errorf(gotext.Get("error cloning %s: %s", name, stderr))
		}

		return true, nil
	} else if err != nil {
		return false, fmt.Errorf(gotext.Get("error reading %s", filepath.Join(path, name, ".git")))
	}

	cmd := passToGit(filepath.Join(path, name), "fetch")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	_, stderr, err := capture(cmd)
	if err != nil {
		return false, fmt.Errorf(gotext.Get("error fetching %s: %s", name, stderr))
	}

	return false, nil
}

func gitMerge(path, name string) error {
	_, stderr, err := capture(passToGit(filepath.Join(path, name), "reset", "--hard", "HEAD"))
	if err != nil {
		return fmt.Errorf(gotext.Get("error resetting %s: %s", name, stderr))
	}

	_, stderr, err = capture(passToGit(filepath.Join(path, name), "merge", "--no-edit", "--ff"))
	if err != nil {
		return fmt.Errorf(gotext.Get("error merging %s: %s", name, stderr))
	}

	return nil
}

func getPkgbuilds(pkgs []string, alpmHandle *alpm.Handle, force bool) error {
	missing := false
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	pkgs = query.RemoveInvalidTargets(pkgs, config.Runtime.Mode)
	aur, repo, err := packageSlices(pkgs, alpmHandle)
	if err != nil {
		return err
	}

	for n := range aur {
		_, pkg := text.SplitDBFromName(aur[n])
		aur[n] = pkg
	}

	info, err := query.AURInfoPrint(aur, config.RequestSplitN)
	if err != nil {
		return err
	}

	if len(repo) > 0 {
		missing, err = getPkgbuildsfromABS(repo, wd, alpmHandle, force)
		if err != nil {
			return err
		}
	}

	if len(aur) > 0 {
		allBases := dep.GetBases(info)
		bases := make([]dep.Base, 0)

		for _, base := range allBases {
			name := base.Pkgbase()
			pkgDest := filepath.Join(wd, name)
			_, err = os.Stat(pkgDest)
			if os.IsNotExist(err) {
				bases = append(bases, base)
			} else if err != nil {
				text.Errorln(err)
				continue
			} else {
				if force {
					if err = os.RemoveAll(pkgDest); err != nil {
						text.Errorln(err)
						continue
					}
					bases = append(bases, base)
				} else {
					text.Warnln(gotext.Get("%s already exists. Use -f/--force to overwrite", pkgDest))
					continue
				}
			}
		}

		if _, err = downloadPkgbuilds(bases, nil, wd); err != nil {
			return err
		}

		missing = missing || len(aur) != len(info)
	}

	if missing {
		err = fmt.Errorf("")
	}

	return err
}

type pkgInfo struct {
	url          string
	customBranch bool
}

// GetPkgbuild downloads pkgbuild from the ABS or Artix's gitea.
func getPkgbuildsfromABS(pkgs []string, path string, alpmHandle *alpm.Handle, force bool) (bool, error) {
	var wg sync.WaitGroup
	var mux sync.Mutex
	var errs multierror.MultiError
	names := make(map[string]pkgInfo)
	missing := make([]string, 0)
	downloaded := 0

	dbList, err := alpmHandle.SyncDBs()
	if err != nil {
		return false, err
	}

	for _, pkgN := range pkgs {
		var pkg *alpm.Package
		var err error
		var url string
		pkgDB, name := text.SplitDBFromName(pkgN)

		if pkgDB != "" {
			if db, errSync := alpmHandle.SyncDBByName(pkgDB); errSync == nil {
				pkg = db.Pkg(name)
			}
		} else {
			_ = dbList.ForEach(func(db alpm.DB) error {
				if pkg = db.Pkg(name); pkg != nil {
					return fmt.Errorf("")
				}
				return nil
			})
		}

		if pkg == nil {
			missing = append(missing, name)
			continue
		}

		name = pkg.Base()
		if name == "" {
			name = pkg.Name()
		}

		customBranch := true

		// TODO: Check existence with ls-remote
		// https://git.archlinux.org/svntogit/packages.git
		switch pkg.DB().Name() {
		case "core", "extra", "testing":
			url = "https://git.archlinux.org/svntogit/packages.git"
		case "community", "multilib", "community-testing", "multilib-testing":
			url = "https://git.archlinux.org/svntogit/community.git"
		case "system", "world", "galaxy":
			customBranch = false
			url = fmt.Sprintf("https://gitea.artixlinux.org/packages%s/%s.git", strings.ToUpper(string(name[0])), name)
		default:
			missing = append(missing, name)
			continue
		}

		_, err = os.Stat(filepath.Join(path, name))
		switch {
		case err != nil && !os.IsNotExist(err):
			text.Errorln(err)
			continue
		case os.IsNotExist(err), force:
			if err = os.RemoveAll(filepath.Join(path, name)); err != nil {
				text.Errorln(err)
				continue
			}
		default:
			text.Warn(gotext.Get("%s already downloaded -- use -f to overwrite", cyan(name)))
			continue
		}

		names[name] = pkgInfo{
			customBranch: customBranch,
			url:          url,
		}
	}

	if len(missing) != 0 {
		text.Warnln(gotext.Get("Missing ABS packages:"),
			cyan(strings.Join(missing, ", ")))
	}

	download := func(pkg string, info pkgInfo) {
		defer wg.Done()
		if _, err := gitDownloadABS(info, config.ABSDir, pkg); err != nil {
			errs.Add(errors.New(gotext.Get("failed to get pkgbuild: %s: %s", cyan(pkg), err.Error())))
			return
		}

		_, stderr, err := capture(exec.Command("cp", "-r", filepath.Clean(filepath.Join(config.ABSDir, pkg, "trunk")), filepath.Clean(filepath.Join(path, pkg))))
		mux.Lock()
		downloaded++
		if err != nil {
			errs.Add(errors.New(gotext.Get("failed to link %s: %s", cyan(pkg), stderr)))
		} else {
			fmt.Fprintln(os.Stdout, gotext.Get("(%d/%d) Downloaded PKGBUILD from ABS: %s", downloaded, len(names), cyan(pkg)))
		}
		mux.Unlock()
	}

	count := 0
	for name, info := range names {
		wg.Add(1)
		go download(name, info)
		count++
		if count%25 == 0 {
			wg.Wait()
		}
	}

	wg.Wait()

	return len(missing) != 0, errs.Return()
}
