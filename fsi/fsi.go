// Package fsi defines qri file system integration: representing a dataset as
// files in a directory on a user's computer. Using fsi, users can edit files
// as an interface for working with qri datasets.
//
// A dataset is "linked" to a directory through a `.qri_ref` dotfile that
// connects the folder to a version history stored in the local qri repository.
//
// files in a linked directory follow naming conventions that map to components
// of a dataset. eg: a file named "meta.json" in a linked directory maps to
// the dataset meta component. This mapping can be used to construct a dataset
// for read and write actions
package fsi

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	golog "github.com/ipfs/go-log"
	"github.com/qri-io/qri/base"
	"github.com/qri-io/qri/base/component"
	"github.com/qri-io/qri/repo"
	reporef "github.com/qri-io/qri/repo/ref"
)

// package level logger
var (
	log = golog.Logger("fsi")

	// ErrNotLinkedToFilesystem is the err implementers should return when we
	// are expecting the dataset to have a file system link, but fsiPath is empty
	ErrNoLink = fmt.Errorf("dataset is not linked to the filesystem")
)

// QriRefFilename is the name of the file that links a folder to a dataset.
// The file contains a dataset reference that declares the link
// ref files are the authoritative definition of weather a folder is linked
// or not
const QriRefFilename = ".qri-ref"

// GetLinkedFilesysRef returns whether a directory is linked to a
// dataset in your repo, and the reference to that dataset.
func GetLinkedFilesysRef(dir string) (string, bool) {
	data, err := ioutil.ReadFile(filepath.Join(dir, QriRefFilename))
	if err == nil {
		return strings.TrimSpace(string(data)), true
	}
	return "", false
}

// RepoPath returns the standard path to an FSI file for a given file-system
// repo location
func RepoPath(repoPath string) string {
	return filepath.Join(repoPath, "fsi.qfb")
}

// FSI is a repo-side struct for coordinating file system integration
type FSI struct {
	// repository for resolving dataset names
	repo repo.Repo
}

// NewFSI creates an FSI instance from a path to a links flatbuffer file
func NewFSI(r repo.Repo) *FSI {
	return &FSI{repo: r}
}

// LinkedRefs returns a list of linked datasets and their connected directories
func (fsi *FSI) LinkedRefs(offset, limit int) ([]reporef.DatasetRef, error) {
	// TODO (b5) - figure out a better pagination / querying strategy here
	allRefs, err := fsi.repo.References(offset, 100000)
	if err != nil {
		return nil, err
	}

	var refs []reporef.DatasetRef
	for _, ref := range allRefs {
		if ref.FSIPath != "" {
			if offset > 0 {
				offset--
				continue
			}
			refs = append(refs, ref)
		}
		if len(refs) == limit {
			return refs, nil
		}
	}

	return refs, nil
}

// CreateLink links a working directory to a dataset. Returns the reference alias, and a
// rollback function if no error occurs
func (fsi *FSI) CreateLink(dirPath, refStr string) (alias string, rollback func(), err error) {
	rollback = func() {}

	ref, err := repo.ParseDatasetRef(refStr)
	if err != nil {
		return "", rollback, err
	}
	err = repo.CanonicalizeDatasetRef(fsi.repo, &ref)
	if err != nil && err != repo.ErrNotFound && err != repo.ErrNoHistory {
		return ref.String(), rollback, err
	}

	if stored, err := fsi.repo.GetRef(ref); err == nil {
		if stored.FSIPath != "" {
			// There is already a link for this dataset, see if that link still exists.
			targetPath := filepath.Join(stored.FSIPath, QriRefFilename)
			if _, err := os.Stat(targetPath); err == nil {
				return "", rollback, fmt.Errorf("'%s' is already linked to %s", ref.AliasString(), stored.FSIPath)
			}
		}
	}

	// Link the FSIPath to the reference before putting it into the repo
	log.Debugf("fsi.CreateLink: linking ref=%q, FSIPath=%q", ref, dirPath)
	ref.FSIPath = dirPath
	if err = fsi.repo.PutRef(ref); err != nil {
		return "", rollback, err
	}
	// If future steps fail, remove the ref we just put
	removeRefFunc := func() {
		log.Debugf("removing repo.ref %q during rollback", ref)
		if err := fsi.repo.DeleteRef(ref); err != nil {
			log.Debugf("error while removing repo.ref %q: %s", ref, err)
		}
	}

	linkFile := ""
	if linkFile, err = writeLinkFile(dirPath, ref.AliasString()); err != nil {
		return "", removeRefFunc, err
	}
	// If future steps fail, remove the link file we just wrote to
	removeLinkAndRemoveRefFunc := func() {
		log.Debugf("removing linkFile %q during rollback", linkFile)
		if err := os.Remove(linkFile); err != nil {
			log.Debugf("error while removing linkFile %q: %s", linkFile, err)
		}
		removeRefFunc()
	}

	return ref.AliasString(), removeLinkAndRemoveRefFunc, err
}

// ModifyLinkDirectory changes the FSIPath in the repo so that it is linked to the directory. Does
// not affect the .qri-ref linkfile in the working directory. Called when the command-line
// interface or filesystem watcher detects that a working folder has been moved.
// TODO(dlong): Add a filesystem watcher that behaves as described
// TODO(dlong): Perhaps add a `qri mv` command that explicitly changes a working directory location
func (fsi *FSI) ModifyLinkDirectory(dirPath, refStr string) error {
	ref, err := repo.ParseDatasetRef(refStr)
	if err != nil {
		return err
	}
	if err = repo.CanonicalizeDatasetRef(fsi.repo, &ref); err != nil && err != repo.ErrNoHistory {
		return err
	}
	if ref.FSIPath == dirPath {
		return nil
	}

	log.Debugf("fsi.ModifyLinkDirectory: modify ref=%q, FSIPath was %q, changing to %q", ref, ref.FSIPath, dirPath)
	ref.FSIPath = dirPath
	return fsi.repo.PutRef(ref)
}

// ModifyLinkReference changes the reference that is in .qri-ref linkfile in the working directory.
// Does not affect the ref in the repo. Called when a rename command is invoked.
func (fsi *FSI) ModifyLinkReference(dirPath, refStr string) error {
	ref, err := repo.ParseDatasetRef(refStr)
	if err != nil {
		return err
	}
	if err = repo.CanonicalizeDatasetRef(fsi.repo, &ref); err != nil && err != repo.ErrNoHistory {
		return err
	}

	log.Debugf("fsi.ModifyLinkReference: modify linkfile at %q, ref=%q", dirPath, ref)
	if _, err = writeLinkFile(dirPath, ref.AliasString()); err != nil {
		return err
	}
	return nil
}

// Unlink breaks the connection between a directory and a dataset
func (fsi *FSI) Unlink(dirPath, refStr string) error {
	ref, err := repo.ParseDatasetRef(refStr)
	if err != nil {
		return err
	}

	if removeLinkErr := removeLinkFile(dirPath); removeLinkErr != nil {
		log.Debugf("removing link file: %s", removeLinkErr.Error())
	}

	defer func() {
		// always attempt to remove the directory, ignoring "directory not empty" errors
		// os.Remove will fail if the directory isn't empty, which is the behaviour
		// we want
		if err := os.Remove(dirPath); err != nil && !strings.Contains(err.Error(), "directory not empty") {
			log.Errorf("removing directory: %s", err.Error())
		}
	}()

	if err = repo.CanonicalizeDatasetRef(fsi.repo, &ref); err != nil {
		if err == repo.ErrNoHistory {
			// if we're unlinking a ref without history, delete it
			return fsi.repo.DeleteRef(ref)
		}
		return err
	}

	ref.FSIPath = ""
	return fsi.repo.PutRef(ref)
}

func (fsi *FSI) getRepoRef(refStr string) (ref reporef.DatasetRef, err error) {
	ref, err = repo.ParseDatasetRef(refStr)
	if err != nil {
		return ref, err
	}

	if err = repo.CanonicalizeDatasetRef(fsi.repo, &ref); err != nil {
		return ref, err
	}

	return fsi.repo.GetRef(ref)
}

func writeLinkFile(dir, linkstr string) (string, error) {
	linkFile := filepath.Join(dir, QriRefFilename)
	return linkFile, base.WriteHiddenFile(linkFile, linkstr)
}

func removeLinkFile(dir string) error {
	dir = filepath.Join(dir, QriRefFilename)
	return os.Remove(dir)
}

// DeleteComponentFiles deletes all component files in the directory. Should only be used if
// removing an entire dataset, or if the dataset is about to be rewritten back to the filesystem.
func DeleteComponentFiles(dir string) error {
	dirComps, err := component.ListDirectoryComponents(dir)
	if err != nil {
		return err
	}
	for _, compName := range component.AllSubcomponentNames() {
		comp := dirComps.Base().GetSubcomponent(compName)
		if comp == nil {
			continue
		}
		err = os.Remove(comp.Base().SourceFile)
		if err != nil {
			log.Errorf("deleting file %q, error: %s", comp.Base().SourceFile, err)
			return err
		}
	}
	return nil
}
