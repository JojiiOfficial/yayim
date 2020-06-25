package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	alpm "github.com/Jguer/go-alpm"
	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/text"
)

// Verbosity settings for search
const (
	numberMenu = iota
	detailed
	minimal
)

const (
	// Describes Sorting method for numberdisplay
	bottomUp = iota
	topDown
)

const (
	modeAUR targetMode = iota
	modeRepo
	modeAny
)

type targetMode int

// Configuration stores yay's config.
type Configuration struct {
	AURURL             string `json:"aururl"`
	BuildDir           string `json:"buildDir"`
	ABSDir             string `json:"absdir"`
	Editor             string `json:"editor"`
	EditorFlags        string `json:"editorflags"`
	MakepkgBin         string `json:"makepkgbin"`
	MakepkgConf        string `json:"makepkgconf"`
	PacmanBin          string `json:"pacmanbin"`
	PacmanConf         string `json:"pacmanconf"`
	ReDownload         string `json:"redownload"`
	ReBuild            string `json:"rebuild"`
	AnswerClean        string `json:"answerclean"`
	AnswerDiff         string `json:"answerdiff"`
	AnswerEdit         string `json:"answeredit"`
	AnswerUpgrade      string `json:"answerupgrade"`
	GitBin             string `json:"gitbin"`
	GpgBin             string `json:"gpgbin"`
	GpgFlags           string `json:"gpgflags"`
	MFlags             string `json:"mflags"`
	SortBy             string `json:"sortby"`
	SearchBy           string `json:"searchby"`
	GitFlags           string `json:"gitflags"`
	RemoveMake         string `json:"removemake"`
	SudoBin            string `json:"sudobin"`
	SudoFlags          string `json:"sudoflags"`
	RequestSplitN      int    `json:"requestsplitn"`
	SearchMode         int    `json:"-"`
	SortMode           int    `json:"sortmode"`
	CompletionInterval int    `json:"completionrefreshtime"`
	SudoLoop           bool   `json:"sudoloop"`
	TimeUpdate         bool   `json:"timeupdate"`
	NoConfirm          bool   `json:"-"`
	Devel              bool   `json:"devel"`
	CleanAfter         bool   `json:"cleanAfter"`
	Provides           bool   `json:"provides"`
	PGPFetch           bool   `json:"pgpfetch"`
	UpgradeMenu        bool   `json:"upgrademenu"`
	CleanMenu          bool   `json:"cleanmenu"`
	DiffMenu           bool   `json:"diffmenu"`
	EditMenu           bool   `json:"editmenu"`
	CombinedUpgrade    bool   `json:"combinedupgrade"`
	UseAsk             bool   `json:"useask"`
	BatchInstall       bool   `json:"batchinstall"`
	Fuzzy              bool   `json:"fuzzy"`
	LangCheck          bool   `json:"langcheck"`
}

var yayVersion = "10.0.2"

var localePath = "/usr/share/locale"

// configFileName holds the name of the config file.
const configFileName string = "config.json"

// vcsFileName holds the name of the vcs file.
const vcsFileName string = "vcs.json"

// useColor enables/disables colored printing
var useColor bool

// configHome handles config directory home
var configHome string

// cacheHome handles cache home
var cacheHome string

// savedInfo holds the current vcs info
var savedInfo vcsInfo

// configfile holds yay config file path.
var configFile string

// vcsfile holds yay vcs info file path.
var vcsFile string

// shouldSaveConfig holds whether or not the config should be saved
var shouldSaveConfig bool

// YayConf holds the current config values for yay.
var config *Configuration

// AlpmConf holds the current config values for pacman.
var pacmanConf *pacmanconf.Config

// AlpmHandle is the alpm handle used by yay.
var alpmHandle *alpm.Handle

// Mode is used to restrict yay to AUR or repo only modes
var mode = modeAny

var hideMenus = false

// SaveConfig writes yay config to file.
func (config *Configuration) saveConfig() error {
	marshalledinfo, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return err
	}
	in, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer in.Close()
	if _, err = in.Write(marshalledinfo); err != nil {
		return err
	}
	return in.Sync()
}

func defaultSettings() *Configuration {
	newConfig := &Configuration{
		AURURL:             "https://aur.archlinux.org",
		BuildDir:           "$HOME/.cache/yay",
		ABSDir:             "$HOME/.cache/yay/abs",
		CleanAfter:         false,
		Editor:             "",
		EditorFlags:        "",
		Devel:              false,
		MakepkgBin:         "makepkg",
		MakepkgConf:        "",
		NoConfirm:          false,
		PacmanBin:          "pacman",
		PGPFetch:           true,
		PacmanConf:         "/etc/pacman.conf",
		GpgFlags:           "",
		MFlags:             "",
		GitFlags:           "",
		SortMode:           bottomUp,
		CompletionInterval: 7,
		SortBy:             "votes",
		SearchBy:           "name-desc",
		SudoLoop:           false,
		GitBin:             "git",
		GpgBin:             "gpg",
		SudoBin:            "sudo",
		SudoFlags:          "",
		TimeUpdate:         false,
		RequestSplitN:      150,
		ReDownload:         "no",
		ReBuild:            "no",
		BatchInstall:       false,
		AnswerClean:        "",
		AnswerDiff:         "",
		AnswerEdit:         "",
		AnswerUpgrade:      "",
		RemoveMake:         "ask",
		Provides:           true,
		UpgradeMenu:        true,
		CleanMenu:          true,
		DiffMenu:           true,
		EditMenu:           false,
		UseAsk:             false,
		CombinedUpgrade:    false,
		Fuzzy:              false,
	}

	if os.Getenv("XDG_CACHE_HOME") != "" {
		newConfig.BuildDir = "$XDG_CACHE_HOME/yay"
	}

	return newConfig
}

func (config *Configuration) expandEnv() {
	config.AURURL = os.ExpandEnv(config.AURURL)
	config.ABSDir = os.ExpandEnv(config.ABSDir)
	config.BuildDir = os.ExpandEnv(config.BuildDir)
	config.Editor = os.ExpandEnv(config.Editor)
	config.EditorFlags = os.ExpandEnv(config.EditorFlags)
	config.MakepkgBin = os.ExpandEnv(config.MakepkgBin)
	config.MakepkgConf = os.ExpandEnv(config.MakepkgConf)
	config.PacmanBin = os.ExpandEnv(config.PacmanBin)
	config.PacmanConf = os.ExpandEnv(config.PacmanConf)
	config.GpgFlags = os.ExpandEnv(config.GpgFlags)
	config.MFlags = os.ExpandEnv(config.MFlags)
	config.GitFlags = os.ExpandEnv(config.GitFlags)
	config.SortBy = os.ExpandEnv(config.SortBy)
	config.SearchBy = os.ExpandEnv(config.SearchBy)
	config.GitBin = os.ExpandEnv(config.GitBin)
	config.GpgBin = os.ExpandEnv(config.GpgBin)
	config.SudoBin = os.ExpandEnv(config.SudoBin)
	config.SudoFlags = os.ExpandEnv(config.SudoFlags)
	config.ReDownload = os.ExpandEnv(config.ReDownload)
	config.ReBuild = os.ExpandEnv(config.ReBuild)
	config.AnswerClean = os.ExpandEnv(config.AnswerClean)
	config.AnswerDiff = os.ExpandEnv(config.AnswerDiff)
	config.AnswerEdit = os.ExpandEnv(config.AnswerEdit)
	config.AnswerUpgrade = os.ExpandEnv(config.AnswerUpgrade)
	config.RemoveMake = os.ExpandEnv(config.RemoveMake)
}

// Editor returns the preferred system editor.
func editor() (editor string, args []string) {
	switch {
	case config.Editor != "":
		editor, err := exec.LookPath(config.Editor)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			return editor, strings.Fields(config.EditorFlags)
		}
		fallthrough
	case os.Getenv("EDITOR") != "":
		if editorArgs := strings.Fields(os.Getenv("EDITOR")); len(editorArgs) != 0 {
			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				return editor, editorArgs[1:]
			}
		}
		fallthrough
	case os.Getenv("VISUAL") != "":
		if editorArgs := strings.Fields(os.Getenv("VISUAL")); len(editorArgs) != 0 {
			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				return editor, editorArgs[1:]
			}
		}
		fallthrough
	default:
		fmt.Fprintln(os.Stderr)
		text.Errorln(gotext.Get("%s is not set", bold(cyan("$EDITOR"))))
		text.Warnln(gotext.Get("Add %s or %s to your environment variables", bold(cyan("$EDITOR")), bold(cyan("$VISUAL"))))

		for {
			text.Infoln(gotext.Get("Edit PKGBUILD with?"))
			editorInput, err := getInput("")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			editorArgs := strings.Fields(editorInput)
			if len(editorArgs) == 0 {
				continue
			}

			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			return editor, editorArgs[1:]
		}
	}
}

// ContinueTask prompts if user wants to continue task.
// If NoConfirm is set the action will continue without user input.
func continueTask(s string, cont bool) bool {
	if config.NoConfirm {
		return cont
	}

	var response string
	var postFix string
	yes := gotext.Get("yes")
	no := gotext.Get("no")
	y := string([]rune(yes)[0])
	n := string([]rune(no)[0])

	if cont {
		postFix = fmt.Sprintf(" [%s/%s] ", strings.ToUpper(y), n)
	} else {
		postFix = fmt.Sprintf(" [%s/%s] ", y, strings.ToUpper(n))
	}

	text.Info(bold(s), bold(postFix))

	if _, err := fmt.Scanln(&response); err != nil {
		return cont
	}

	response = strings.ToLower(response)
	return response == yes || response == y
}

func getInput(defaultValue string) (string, error) {
	text.Info()
	if defaultValue != "" || config.NoConfirm {
		fmt.Println(defaultValue)
		return defaultValue, nil
	}

	reader := bufio.NewReader(os.Stdin)

	buf, overflow, err := reader.ReadLine()
	if err != nil {
		return "", err
	}

	if overflow {
		return "", fmt.Errorf(gotext.Get("input too long"))
	}

	return string(buf), nil
}

func (config *Configuration) String() string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "\t")
	if err := enc.Encode(config); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return buf.String()
}

func toUsage(usages []string) alpm.Usage {
	if len(usages) == 0 {
		return alpm.UsageAll
	}

	var ret alpm.Usage
	for _, usage := range usages {
		switch usage {
		case "Sync":
			ret |= alpm.UsageSync
		case "Search":
			ret |= alpm.UsageSearch
		case "Install":
			ret |= alpm.UsageInstall
		case "Upgrade":
			ret |= alpm.UsageUpgrade
		case "All":
			ret |= alpm.UsageAll
		}
	}

	return ret
}

func configureAlpm() error {
	// TODO: set SigLevel
	// sigLevel := alpm.SigPackage | alpm.SigPackageOptional | alpm.SigDatabase | alpm.SigDatabaseOptional
	// localFileSigLevel := alpm.SigUseDefault
	// remoteFileSigLevel := alpm.SigUseDefault

	for _, repo := range pacmanConf.Repos {
		// TODO: set SigLevel
		db, err := alpmHandle.RegisterSyncDB(repo.Name, 0)
		if err != nil {
			return err
		}

		db.SetServers(repo.Servers)
		db.SetUsage(toUsage(repo.Usage))
	}

	if err := alpmHandle.SetCacheDirs(pacmanConf.CacheDir); err != nil {
		return err
	}

	// add hook directories 1-by-1 to avoid overwriting the system directory
	for _, dir := range pacmanConf.HookDir {
		if err := alpmHandle.AddHookDir(dir); err != nil {
			return err
		}
	}

	if err := alpmHandle.SetGPGDir(pacmanConf.GPGDir); err != nil {
		return err
	}

	if err := alpmHandle.SetLogFile(pacmanConf.LogFile); err != nil {
		return err
	}

	if err := alpmHandle.SetIgnorePkgs(pacmanConf.IgnorePkg); err != nil {
		return err
	}

	if err := alpmHandle.SetIgnoreGroups(pacmanConf.IgnoreGroup); err != nil {
		return err
	}

	if err := alpmHandle.SetArch(pacmanConf.Architecture); err != nil {
		return err
	}

	if err := alpmHandle.SetNoUpgrades(pacmanConf.NoUpgrade); err != nil {
		return err
	}

	if err := alpmHandle.SetNoExtracts(pacmanConf.NoExtract); err != nil {
		return err
	}

	/*if err := alpmHandle.SetDefaultSigLevel(sigLevel); err != nil {
		return err
	}

	if err := alpmHandle.SetLocalFileSigLevel(localFileSigLevel); err != nil {
		return err
	}

	if err := alpmHandle.SetRemoteFileSigLevel(remoteFileSigLevel); err != nil {
		return err
	}*/

	if err := alpmHandle.SetUseSyslog(pacmanConf.UseSyslog); err != nil {
		return err
	}

	return alpmHandle.SetCheckSpace(pacmanConf.CheckSpace)
}
