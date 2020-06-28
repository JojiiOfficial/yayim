package main // import "github.com/Jguer/yay"

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	alpm "github.com/Jguer/go-alpm"
	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/text"
)

func setPaths() error {
	if configHome = os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		configHome = filepath.Join(configHome, "yay")
	} else if configHome = os.Getenv("HOME"); configHome != "" {
		configHome = filepath.Join(configHome, ".config/yay")
	} else {
		return errors.New(gotext.Get("%s and %s unset", "XDG_CONFIG_HOME", "HOME"))
	}

	if cacheHome = os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		cacheHome = filepath.Join(cacheHome, "yay")
	} else if cacheHome = os.Getenv("HOME"); cacheHome != "" {
		cacheHome = filepath.Join(cacheHome, ".cache/yay")
	} else {
		return errors.New(gotext.Get("%s and %s unset", "XDG_CACHE_HOME", "HOME"))
	}

	configFile = filepath.Join(configHome, configFileName)
	vcsFile = filepath.Join(cacheHome, vcsFileName)

	return nil
}

func initGotext() {
	if envLocalePath := os.Getenv("LOCALE_PATH"); envLocalePath != "" {
		localePath = envLocalePath
	}

	gotext.Configure(localePath, os.Getenv("LANG"), "yay")
}

func initConfig() error {
	cfile, err := os.Open(configFile)
	if !os.IsNotExist(err) && err != nil {
		return errors.New(gotext.Get("failed to open config file '%s': %s", configFile, err))
	}

	defer cfile.Close()
	if !os.IsNotExist(err) {
		decoder := json.NewDecoder(cfile)
		if err = decoder.Decode(&config); err != nil {
			return errors.New(gotext.Get("failed to read config file '%s': %s", configFile, err))
		}
	}

	aurdest := os.Getenv("AURDEST")
	if aurdest != "" {
		config.BuildDir = aurdest
	}

	return nil
}

func initVCS() error {
	vfile, err := os.Open(vcsFile)
	if !os.IsNotExist(err) && err != nil {
		return errors.New(gotext.Get("failed to open vcs file '%s': %s", vcsFile, err))
	}

	defer vfile.Close()
	if !os.IsNotExist(err) {
		decoder := json.NewDecoder(vfile)
		if err = decoder.Decode(&savedInfo); err != nil {
			return errors.New(gotext.Get("failed to read vcs file '%s': %s", vcsFile, err))
		}
	}

	return nil
}

func initHomeDirs() error {
	if _, err := os.Stat(configHome); os.IsNotExist(err) {
		if err = os.MkdirAll(configHome, 0700); err != nil {
			return errors.New(gotext.Get("failed to create config directory '%s': %s", configHome, err))
		}
	} else if err != nil {
		return err
	}

	if _, err := os.Stat(cacheHome); os.IsNotExist(err) {
		if err = os.MkdirAll(cacheHome, 0700); err != nil {
			return errors.New(gotext.Get("failed to create cache directory '%s': %s", cacheHome, err))
		}
	} else if err != nil {
		return err
	}

	return nil
}

func initBuildDir() error {
	if _, err := os.Stat(config.BuildDir); os.IsNotExist(err) {
		if err = os.MkdirAll(config.BuildDir, 0700); err != nil {
			return errors.New(gotext.Get("failed to create BuildDir directory '%s': %s", config.BuildDir, err))
		}
	} else if err != nil {
		return err
	}

	return nil
}

func initAlpm() error {
	var err error
	var stderr string

	root := "/"
	if value, _, exists := cmdArgs.getArg("root", "r"); exists {
		root = value
	}

	pacmanConf, stderr, err = pacmanconf.PacmanConf("--config", config.PacmanConf, "--root", root)
	if err != nil {
		return fmt.Errorf("%s", stderr)
	}

	if value, _, exists := cmdArgs.getArg("dbpath", "b"); exists {
		pacmanConf.DBPath = value
	}

	if value, _, exists := cmdArgs.getArg("arch"); exists {
		pacmanConf.Architecture = value
	}

	if value, _, exists := cmdArgs.getArg("ignore"); exists {
		pacmanConf.IgnorePkg = append(pacmanConf.IgnorePkg, strings.Split(value, ",")...)
	}

	if value, _, exists := cmdArgs.getArg("ignoregroup"); exists {
		pacmanConf.IgnoreGroup = append(pacmanConf.IgnoreGroup, strings.Split(value, ",")...)
	}

	// TODO
	// current system does not allow duplicate arguments
	// but pacman allows multiple cachedirs to be passed
	// for now only handle one cache dir
	if value, _, exists := cmdArgs.getArg("cachedir"); exists {
		pacmanConf.CacheDir = []string{value}
	}

	if value, _, exists := cmdArgs.getArg("gpgdir"); exists {
		pacmanConf.GPGDir = value
	}

	if err := initAlpmHandle(); err != nil {
		return err
	}

	switch value, _, _ := cmdArgs.getArg("color"); value {
	case "always":
		text.UseColor = true
	case "auto":
		text.UseColor = isTty()
	case "never":
		text.UseColor = false
	default:
		text.UseColor = pacmanConf.Color && isTty()
	}

	return nil
}

func initAlpmHandle() error {
	if alpmHandle != nil {
		if errRelease := alpmHandle.Release(); errRelease != nil {
			return errRelease
		}
	}

	var err error
	if alpmHandle, err = alpm.Initialize(pacmanConf.RootDir, pacmanConf.DBPath); err != nil {
		return errors.New(gotext.Get("unable to CreateHandle: %s", err))
	}

	if err := configureAlpm(); err != nil {
		return err
	}

	alpmHandle.SetQuestionCallback(questionCallback)
	alpmHandle.SetLogCallback(logCallback)
	return nil
}

func exitOnError(err error) {
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		cleanup()
		os.Exit(1)
	}
}

func cleanup() int {
	if alpmHandle != nil {
		if err := alpmHandle.Release(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	return 0
}

func main() {
	initGotext()
	if os.Geteuid() == 0 {
		text.Warnln(gotext.Get("Avoid running yay as root/sudo."))
	}

	exitOnError(setPaths())
	config = defaultSettings()
	exitOnError(initHomeDirs())
	exitOnError(initConfig())
	exitOnError(cmdArgs.parseCommandLine())
	if shouldSaveConfig {
		err := config.saveConfig()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	config.expandEnv()
	exitOnError(initBuildDir())
	exitOnError(initVCS())
	exitOnError(initAlpm())
	exitOnError(handleCmd())
	os.Exit(cleanup())
}
