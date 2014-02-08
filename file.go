// History: Feb 07 14 tcolar Creation

package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

// Recursively copy "from" directory into "into" directory
func recursiveCopy(from, into string, overwrite bool) error {
	base := filepath.Base(from)
	return filepath.Walk(from, func(fpath string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		rel, err := filepath.Rel(from, fpath)
		if err != nil {
			return err
		}
		target := path.Join(into, base, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0775)
		} else {
			if !overwrite {
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					return fmt.Errorf("File %s already exists !", target)
				}
			}
			return fileCopy(fpath, target)
		}
	})
}

// Copy a file to a target location
// Theparent dirs of "to" must exist.
func fileCopy(from, to string) error {
	f, err := os.Open(from)
	if err != nil {
		return err
	}
	defer f.Close()
	t, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, t); err != nil {
		t.Close()
		return err
	}
	return t.Close()
}

func fileExists(f string) (found bool) {
	if _, err := os.Stat(f); os.IsNotExist(err) {
		return false
	}
	return true
}

// Try to find a scm project at the given path or any of it's parent
// Error if none found
func findScmPrj(dir string) (scmTag string, prjDir string, err error) {
	printLine(dir, Red)
	if !fileExists(dir) || dir == "." || dir == "/" {
		return scmTag, prjDir, fmt.Errorf("No SCM project found.")
	}

	if fileExists(path.Join(dir, HiddenGit)) {
		return dir, GitTag, err
	} else if fileExists(path.Join(dir, HiddenHg)) {
		return dir, HgTag, err
	} else if fileExists(path.Join(dir, HiddenSvn)) {
		return dir, SvnTag, err
	} else if fileExists(path.Join(dir, HiddenBzr)) {
		return dir, BzrTag, err
	} else {
		// recurse into parent
		return findScmPrj(filepath.Dir(dir))
	}
}
