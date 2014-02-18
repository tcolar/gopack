// History: Feb 05 14 tcolar Creation

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	DEFAULT_COLOR = uint8(0)
)

// Propose solutions to the user for dependency issues
func fixDeps(root string) {

	announceGopack()

	checkConfig(root)

	// Analyze deps
	p, err := AnalyzeSourceTree(root)
	if err != nil {
		fail(err)
	}

	_, dependencies := loadConfiguration(root)
	log.Printf("deps: %s", dependencies)
	if dependencies != nil {
		errors := dependencies.Validate(p)
		for _, e := range errors {
			switch e.Kind {
			case UnusedDep:
				fixUnused(root, e)
			case UnmanagedImport:
				fixUnmanaged(root, e)
			default:
				failWith([]*ProjectError{e})
			}
		}
	}

	// TODO: Propose updating dependencies (latest local, latest remote, custom)
}

// Propose solutions to fix an unused entry (Listed in godeps but not used in the project)
func fixUnused(root string, e *ProjectError) {
	printLine(e.Message, Red)
	answer := ask(fmt.Sprintf("What to do about UNUSED dependency: '%s' ?", e.Path),
		map[int]string{
			'N': "Nothing",
			'R': "Remove it from gopack.config",
		})
	switch strings.ToUpper(answer) {
	case "R": // Remove
		removeDep(root, e.Path)
	case "N": // Nothing
	default:
		grue()
		fixUnused(root, e)
	}
}

// Propose solutions to fix an unmanaged entry (Used in project but unknown by godeps)
func fixUnmanaged(root string, e *ProjectError) {
	// lookup if we had a SM project of the same nae in the riginal GoPath
	origSrc := path.Join(originalGoPath, "src")
	origPrjPath := path.Join(origSrc, e.Path)
	prjPath, prjTag, _ := findScmPrj(origPrjPath)
	gpPath := os.Getenv("GOPATH")
	// Prpose to the user some options
	options := map[int]string{}
	options['N'] = "Nothing"
	if len(prjPath) > 0 {
		options['L'] = fmt.Sprintf("Copy the version found in the local *standard* GOPATH (%s)?", prjTag)
	}
	options['R'] = "Download the latest from repo and add the commit hash to gopack.config"
	options['C'] = "Add to gopack.config with a specific commit"
	options['T'] = "Add to gopack.config with a specific tag"
	options['B'] = "Add to gopack.config with a specific branch"
	options['M'] = "Add to gopack.config the 'master' branch"
	answer := ask(fmt.Sprintf("What to do about UNMANAGED dependency: '%s' ?", e.Path), options)
	scmName := askScm()
	switch strings.ToUpper(answer) {
	case "B": // branch
		branch := ask("Branch name", map[int]string{})
		addDep(root, scmName, BranchProp, e.Path, branch)
	case "T": // tag
		tag := ask("Tag name", map[int]string{})
		addDep(root, scmName, TagProp, e.Path, tag)
	case "C": // commit
		commit := ask("Commit hash", map[int]string{})
		addDep(root, scmName, CommitProp, e.Path, commit)
	case "M": // master
		addDep(root, scmName, BranchProp, e.Path, "master")
	case "L": // Copy from local GoPath project
		target, _ := filepath.Rel(origSrc, prjPath)
		target = path.Dir(target)
		target = path.Join(gpPath, target)
		os.MkdirAll(target, 0755)
		printLine(fmt.Sprintf("Copying %s into %s", prjPath, target), Green)
		if err := recursiveCopy(prjPath, target, true); err != nil {
			fail(err)
		}
		addDep(root, scmName, BranchProp, e.Path, "TODO")
	case "R": // repo latest
		defautRepo := fmt.Sprintf("http://%s", e.Path)
		repo := ask(fmt.Sprintf("Repo path to fetch from: [%s]:", defautRepo), map[int]string{})
		if len(repo) == 0 {
			repo = defautRepo
		}
		scm := Scms[scmName]
		printLine(fmt.Sprintf("Downloading from %s", repo), Green)
		if err := scm.DownloadCommand(repo, dependencyPath(e.Path)).Run(); err != nil {
			log.Print(dependencyPath(e.Path))
			log.Fatal(err)
			fail(fmt.Sprintf("Error downloading dependency '%s': %s", repo, err))
		}
		addDep(root, scmName, CommitProp, e.Path, "TODO")
	case "N": // Nothing
	default:
		grue()
		fixUnmanaged(root, e)
	}
}

// Remove a dependency from the config file
func removeDep(root string, pkgPath string) {
	// TBD
	/*configPath := path.Join(root, "gopack.config")
	  content, err := ioutil.ReadFile(configPath)
	  if err != nil {
	    fail(err)
	  }
	  lines := strings.Split(string(content), "\n")
	  var buf, cur []byte

	  ioutil.WriteFile(configPath, []byte{}, 0644)*/
}

// Add a new dependency
// Would be cleaner to use TOML library that supports encoding such as https://github.com/BurntSushi/toml
func addDep(root, scmName, revType, pkgPath, revision string) {
	configPath := path.Join(root, "gopack.config")
	name := pkgPath[strings.LastIndex(pkgPath, "/")+1:]
	str := fmt.Sprintf("\n\n[deps.%s]\nimport = \"%s\"\n%s = \"%s\"\nscm = \"%s\"\n",
		name, pkgPath, revType, revision, scmName)
	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fail(err)
	}
	defer f.Close()
	if _, err = f.WriteString(str); err != nil {
		fail(err)
	}
	printLine("Created new entry [deps."+name+"] in gopack.config file.", Green)
}

func askScm() string {
	answer := ask("SCM", map[int]string{
		'G': "Git",
		'H': "Hg (Mercurial)",
		'S': "Subversion",
		'B': "Bazaar",
	})
	scm := "git"
	switch strings.ToUpper(answer) {
	case "G":
		scm = GitTag
	case "H":
		scm = HgTag
	case "S":
		scm = SvnTag
	case "B":
		scm = BzrTag
	default:
		grue()
		scm = askScm()
	}
	return scm
}

// Check if the config file exists and propose creating it if not
func checkConfig(root string) {
	configPath := path.Join(root, "gopack.config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		printLine("gopack.config was not found ! ", Red)
		answer := ask("Would you like to create a blank gopack.config ?",
			map[int]string{
				'Y': "Yes",
				'N': "No",
			})
		if strings.ToUpper(answer) == "Y" {
			log.Print(originalGoPath)
			curPath, _ := filepath.Abs(root)
			log.Print(curPath)
			rel, _ := filepath.Rel(path.Join(originalGoPath, "src"), curPath)
			repo := ask(fmt.Sprintf("Repo? : [%s]", rel), map[int]string{})
			if len(repo) == 0 {
				repo = rel
			}
			repoStr := fmt.Sprintf("repo = %s", repo)
			printLine("Creating the gopack.config file.", Green)
			ioutil.WriteFile(configPath, []byte(repoStr), 0644)
		} else {
			fail("No gopack.config file, can't contnue.", Red)
		}
	}
}

func ask(question string, options map[int]string) (answer string) {
	printLine(question, Blue)
	for k, v := range options {
		print(fmt.Sprintf("\t'%c'", k), Green)
		printLine(fmt.Sprintf("\t:\t%s", v), DEFAULT_COLOR)
	}
	print("[Answer:] ", Gray)
	fmt.Scanf("%s", &answer)
	return answer
}

func print(msg string, color uint8) {
	if color != DEFAULT_COLOR {
		fmt.Printf("\033[%dm", color)
	}
	fmt.Print(msg)
	if color != DEFAULT_COLOR {
		fmt.Printf(EndColor)
	}
}

func printLine(msg string, color uint8) {
	print(msg, color)
	fmt.Println()
}

func grue() {
	printLine("You are likely to be eaten by a grue.", Green)
}
