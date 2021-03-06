package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// RemoveTestRunner holds test info integration tests
type RemoveTestRunner struct {
	TestRunner
	LocOrig *time.Location
}

// newRemoveTestRunner returns a new FSITestRunner.
func newRemoveTestRunner(t *testing.T, peerName, testName string) *RemoveTestRunner {
	run := RemoveTestRunner{
		TestRunner: *NewTestRunner(t, peerName, testName),
	}

	// Set the location to New York so that timezone printing is consistent
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	run.LocOrig = location
	StringerLocation = location

	// Restore the location function
	run.Teardown = func() {
		StringerLocation = run.LocOrig
	}

	return &run
}

func parsePathFromRef(ref string) string {
	pos := strings.Index(ref, "@")
	if pos == -1 {
		return ref
	}
	return ref[pos+1:]
}

// Test that adding two versions, then deleting one, ends up with only the first version
func TestRemoveOneRevisionFromRepo(t *testing.T) {
	run := newRemoveTestRunner(t, "test_peer", "qri_test_remove_one_rev_from_repo")
	defer run.Delete()

	// Save a dataset containing a body.json, no meta, nothing special.
	output := run.MustExec(t, "qri save --body=testdata/movies/body_two.json me/remove_test")
	ref1 := parsePathFromRef(parseRefFromSave(output))
	dsPath1 := run.GetPathForDataset(t, 0)
	if ref1 != dsPath1 {
		t.Fatal("ref from first save should match what is in qri repo")
	}

	// Save another version
	output = run.MustExec(t, "qri save --body=testdata/movies/body_four.json me/remove_test")
	ref2 := parsePathFromRef(parseRefFromSave(output))
	dsPath2 := run.GetPathForDataset(t, 0)
	if ref2 != dsPath2 {
		t.Fatal("ref from second save should match what is in qri repo")
	}

	// Remove one version
	run.MustExec(t, "qri remove --revisions=1 me/remove_test")

	// Verify that dsref of HEAD is the same as the result of the first save command
	dsPath3 := run.GetPathForDataset(t, 0)
	if ref1 != dsPath3 {
		t.Errorf("after delete, ref should match the first version, expected: %s\n, got: %s\n",
			ref1, dsPath3)
	}
}

// Test that adding two versions, then deleting all will end up with nothing left
func TestRemoveAllRevisionsFromRepo(t *testing.T) {
	run := newRemoveTestRunner(t, "test_peer", "qri_test_remove_all_rev_from_repo")
	defer run.Delete()

	// Save a dataset containing a body.json, no meta, nothing special.
	output := run.MustExec(t, "qri save --body=testdata/movies/body_two.json me/remove_test")
	ref1 := parsePathFromRef(parseRefFromSave(output))
	dsPath1 := run.GetPathForDataset(t, 0)
	if ref1 != dsPath1 {
		t.Fatal("ref from first save should match what is in qri repo")
	}

	// Save another version
	output = run.MustExec(t, "qri save --body=testdata/movies/body_four.json me/remove_test")
	ref2 := parsePathFromRef(parseRefFromSave(output))
	dsPath2 := run.GetPathForDataset(t, 0)
	if ref2 != dsPath2 {
		t.Fatal("ref from second save should match what is in qri repo")
	}

	// Remove one version
	run.MustExec(t, "qri remove --all me/remove_test")

	// Verify that dsref of HEAD is the same as the result of the first save command
	dsPath3 := run.GetPathForDataset(t, 0)
	if dsPath3 != "" {
		t.Errorf("after delete, dataset should not exist, got: %s\n", dsPath3)
	}
}

// Test that remove from a repo can't be used with --keep-files flag
func TestRemoveRepoCantUseKeepFiles(t *testing.T) {
	run := newRemoveTestRunner(t, "test_peer", "qri_test_remove_repo_cant_use_keep_files")
	defer run.Delete()

	// Save a dataset containing a body.json, no meta, nothing special.
	output := run.MustExec(t, "qri save --body=testdata/movies/body_two.json me/remove_test")
	ref1 := parsePathFromRef(parseRefFromSave(output))
	dsPath1 := run.GetPathForDataset(t, 0)
	if ref1 != dsPath1 {
		t.Fatal("ref from first save should match what is in qri repo")
	}

	// Save another version
	output = run.MustExec(t, "qri save --body=testdata/movies/body_four.json me/remove_test")
	ref2 := parsePathFromRef(parseRefFromSave(output))
	dsPath2 := run.GetPathForDataset(t, 0)
	if ref2 != dsPath2 {
		t.Fatal("ref from second save should match what is in qri repo")
	}

	// Try to remove with the --keep-files flag should produce an error
	err := run.ExecCommand("qri remove --revisions=1 --keep-files me/remove_test")
	if err == nil {
		t.Fatal("expected error trying to remove with --keep-files, did not get an error")
	}
	expect := `dataset is not linked to filesystem, cannot use keep-files`
	if err.Error() != expect {
		t.Errorf("error mismatch, expect: %s, got: %s", expect, err.Error())
	}
}

// Test removing a revision from a linked directory
func TestRemoveOneRevisionFromWorkingDirectory(t *testing.T) {
	run := NewFSITestRunner(t, "qri_test_remove_one_work_dir")
	defer run.Delete()

	_ = run.CreateAndChdirToWorkDir("remove_one")

	// Init as a linked directory.
	run.MustExec(t, "qri init --name remove_one --format csv")

	// Add a meta.json.
	run.MustWriteFile(t, "meta.json", "{\"title\":\"one\"}\n")

	// Save the new dataset.
	output := run.MustExec(t, "qri save")
	ref1 := parsePathFromRef(parseRefFromSave(output))
	dsPath1 := run.GetPathForDataset(t, 0)
	if ref1 != dsPath1 {
		t.Fatal("ref from first save should match what is in qri repo")
	}

	// Modify meta.json and body.csv.
	run.MustWriteFile(t, "meta.json", "{\"title\":\"two\"}\n")
	run.MustWriteFile(t, "body.csv", "seven,eight,9\n")

	// Save the new dataset.
	output = run.MustExec(t, "qri save")
	ref2 := parsePathFromRef(parseRefFromSave(output))
	dsPath2 := run.GetPathForDataset(t, 0)
	if ref2 != dsPath2 {
		t.Fatal("ref from second save should match what is in qri repo")
	}

	// Remove one revision
	run.MustExec(t, "qri remove --revisions=1")

	// Verify that dsref of HEAD is the same as the result of the first save command
	dsPath3 := run.GetPathForDataset(t, 0)
	if ref1 != dsPath3 {
		t.Errorf("after delete, ref should match the first version, expected: %s\n, got: %s\n",
			ref1, dsPath3)
	}

	// Verify the meta.json contains the original contents, not `{"title":"two"}`
	actual := run.MustReadFile(t, "meta.json")
	expect := `{
 "qri": "md:0",
 "title": "one"
}`
	if diff := cmp.Diff(expect, actual); diff != "" {
		t.Errorf("meta.json contents (-want +got):\n%s", diff)
	}

	// Verify the body.csv contains the original contents, not "seven,eight,9"
	actual = run.MustReadFile(t, "body.csv")
	expect = "one,two,3\nfour,five,6\n"
	if diff := cmp.Diff(expect, actual); diff != "" {
		t.Errorf("body.csv contents (-want +got):\n%s", diff)
	}

	// Verify that status is clean
	output = run.MustExec(t, "qri status")
	expect = `for linked dataset [test_peer/remove_one]

working directory clean
`
	if diff := cmpTextLines(expect, output); diff != "" {
		t.Errorf("qri status (-want +got):\n%s", diff)
	}

	// Verify that we can access the working directory. This would not be the case if the
	// delete operation caused the FSIPath to be moved from the dataset ref in the repo.
	actual = run.MustExec(t, "qri get")
	expect = `for linked dataset [test_peer/remove_one]

bodyPath: /tmp/remove_one/body.csv
meta:
  qri: md:0
  title: one
name: remove_one
peername: test_peer
qri: ds:0
structure:
  format: csv
  formatConfig:
    lazyQuotes: true
  qri: st:0
  schema:
    items:
      items:
      - title: field_1
        type: string
      - title: field_2
        type: string
      - title: field_3
        type: integer
      type: array
    type: array

`
	if diff := cmp.Diff(expect, actual); diff != "" {
		t.Errorf("dataset result from get: (-want +got):\n%s", diff)
	}
}

// Test removing a revision which added a component will cause that component's file to be removed
func TestRemoveOneRevisionWillDeleteFilesThatWereNotThereBefore(t *testing.T) {
	run := NewFSITestRunner(t, "qri_test_remove_one_that_wasnt_there")
	defer run.Delete()

	workDir := run.CreateAndChdirToWorkDir("remove_one")

	// Init as a linked directory.
	run.MustExec(t, "qri init --name remove_one --format csv")

	// Save the new dataset.
	output := run.MustExec(t, "qri save")
	ref1 := parsePathFromRef(parseRefFromSave(output))
	dsPath1 := run.GetPathForDataset(t, 0)
	if ref1 != dsPath1 {
		t.Fatal("ref from first save should match what is in qri repo")
	}

	// Modify meta.json and body.csv.
	run.MustWriteFile(t, "meta.json", "{\"title\":\"two\"}\n")
	run.MustWriteFile(t, "body.csv", "seven,eight,9\n")

	// Save the new dataset.
	output = run.MustExec(t, "qri save")
	ref2 := parsePathFromRef(parseRefFromSave(output))
	dsPath2 := run.GetPathForDataset(t, 0)
	if ref2 != dsPath2 {
		t.Fatal("ref from second save should match what is in qri repo")
	}

	// Remove one revision
	run.MustExec(t, "qri remove --revisions=1")

	// Verify that dsref of HEAD is the same as the result of the first save command
	dsPath3 := run.GetPathForDataset(t, 0)
	if ref1 != dsPath3 {
		t.Errorf("after delete, ref should match the first version, expected: %s\n, got: %s\n",
			ref1, dsPath3)
	}

	// Verify that status is clean
	output = run.MustExec(t, "qri status")
	expect := `for linked dataset [test_peer/remove_one]

working directory clean
`
	if diff := cmpTextLines(expect, output); diff != "" {
		t.Errorf("qri status (-want +got):\n%s", diff)
	}

	// Verify the directory contains the body, but not the meta
	dirContents := listDirectory(workDir)
	// TODO(dlong): meta.json is written, but is empty. Need to figure out how to determine
	// not to write it, without breaking other things.
	expectContents := []string{".qri-ref", "body.csv", "meta.json", "structure.json"}
	if diff := cmp.Diff(expectContents, dirContents); diff != "" {
		t.Errorf("directory contents (-want +got):\n%s", diff)
	}
}

// Test removing a dataset with no history works
func TestRemoveNoHistory(t *testing.T) {
	run := NewFSITestRunner(t, "qri_test_remove_no_history")
	defer run.Delete()

	workDir := run.CreateAndChdirToWorkDir("remove_no_history")

	// Init as a linked directory.
	run.MustExec(t, "qri init --name remove_no_history --format csv")

	// Try to remove, but this will result in an error because working directory is not clean
	err := run.ExecCommand("qri remove --revisions=1")
	if err == nil {
		t.Fatal("expected error because working directory is not clean")
	}
	expect := `dataset not removed`
	if err.Error() != expect {
		t.Errorf("error mismatch, expect: %s, got: %s", expect, err.Error())
	}

	// Remove one revision, forced
	run.MustExec(t, "qri remove --revisions=1 -f")

	// Verify that dsref of HEAD is empty
	dsPath3 := run.GetPathForDataset(t, 0)
	if dsPath3 != "" {
		t.Errorf("after delete, ref should be empty, got: %s", dsPath3)
	}

	// Verify the directory no longer exists
	if _, err = os.Stat(workDir); !os.IsNotExist(err) {
		t.Errorf("expected \"%s\" to not exist", workDir)
	}
}

// Test removing a revision while keeping the files the same
func TestRemoveKeepFiles(t *testing.T) {
	run := NewFSITestRunner(t, "qri_test_remove_one_keep_files")
	defer run.Delete()

	_ = run.CreateAndChdirToWorkDir("remove_one")

	// Init as a linked directory.
	run.MustExec(t, "qri init --name remove_one --format csv")

	// Save the new dataset.
	output := run.MustExec(t, "qri save")
	ref1 := parsePathFromRef(parseRefFromSave(output))
	dsPath1 := run.GetPathForDataset(t, 0)
	if ref1 != dsPath1 {
		t.Fatal("ref from first save should match what is in qri repo")
	}

	// Modify body.csv.
	run.MustWriteFile(t, "body.csv", "seven,eight,9\n")

	// Save the new dataset.
	output = run.MustExec(t, "qri save")
	ref2 := parsePathFromRef(parseRefFromSave(output))
	dsPath2 := run.GetPathForDataset(t, 0)
	if ref2 != dsPath2 {
		t.Fatal("ref from second save should match what is in qri repo")
	}

	// Modify body.csv again.
	run.MustWriteFile(t, "body.csv", "ten,eleven,12\n")

	// Try to remove, but this will result in an error because working directory is not clean
	err := run.ExecCommand("qri remove --revisions=1")
	if err == nil {
		t.Fatal("expected error because working directory is not clean")
	}
	expect := `dataset not removed`
	if err.Error() != expect {
		t.Errorf("error mismatch, expect: %s, got: %s", expect, err.Error())
	}

	// Verify that dsref of HEAD is still the result of the second save
	dsPath3 := run.GetPathForDataset(t, 0)
	if ref2 != dsPath3 {
		t.Errorf("no commits should have been removed, expected: %s\n, got: %s\n",
			ref2, dsPath3)
	}

	// Remove is possible using --keep-files
	run.MustExec(t, "qri remove --revisions=1 --keep-files")

	// Verify that dsref is now the result of the first save because one commit was removed
	dsPath4 := run.GetPathForDataset(t, 0)
	if ref1 != dsPath4 {
		t.Errorf("no commits should have been removed, expected: %s\n, got: %s\n",
			ref1, dsPath4)
	}

	// Verify the body.csv contains the newest version and was not removed
	actual := run.MustReadFile(t, "body.csv")
	expect = "ten,eleven,12\n"
	if diff := cmp.Diff(expect, actual); diff != "" {
		t.Errorf("body.csv contents (-want +got):\n%s", diff)
	}

	// Verify that status is dirty because we kept the files
	output = run.MustExec(t, "qri status")
	expect = `for linked dataset [test_peer/remove_one]

  modified: body (source: body.csv)

run ` + "`qri save`" + ` to commit this dataset
`
	if diff := cmpTextLines(expect, output); diff != "" {
		t.Errorf("qri status (-want +got):\n%s", diff)
	}
}

// Test removing all versions from a working directory
func TestRemoveAllVersionsWorkingDirectory(t *testing.T) {
	run := NewFSITestRunner(t, "qri_test_remove_all_work_dir")
	defer run.Delete()

	workDir := run.CreateAndChdirToWorkDir("remove_all")

	// Init as a linked directory.
	run.MustExec(t, "qri init --name remove_all --format csv")

	// Save the new dataset.
	output := run.MustExec(t, "qri save")
	ref1 := parsePathFromRef(parseRefFromSave(output))
	dsPath1 := run.GetPathForDataset(t, 0)
	if ref1 != dsPath1 {
		t.Fatal("ref from first save should match what is in qri repo")
	}

	// Modify body.csv.
	run.MustWriteFile(t, "body.csv", "seven,eight,9\n")

	// Save the new dataset.
	output = run.MustExec(t, "qri save")
	ref2 := parsePathFromRef(parseRefFromSave(output))
	dsPath2 := run.GetPathForDataset(t, 0)
	if ref2 != dsPath2 {
		t.Fatal("ref from second save should match what is in qri repo")
	}

	// Remove all versions
	run.MustExec(t, "qri remove --all=1")

	// Verify that dsref of HEAD is empty
	dsPath3 := run.GetPathForDataset(t, 0)
	if dsPath3 != "" {
		t.Errorf("after delete, ref should be empty, got: %s", dsPath3)
	}

	// Verify the directory no longer exists
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Errorf("expected \"%s\" to not exist", workDir)
	}
}

// Test removing all versions while keeping files
func TestRemoveAllAndKeepFiles(t *testing.T) {
	run := NewFSITestRunner(t, "qri_test_remove_all_keep_files")
	defer run.Delete()

	workDir := run.CreateAndChdirToWorkDir("remove_all")

	// Init as a linked directory.
	run.MustExec(t, "qri init --name remove_all --format csv")

	// Save the new dataset.
	output := run.MustExec(t, "qri save")
	ref1 := parsePathFromRef(parseRefFromSave(output))
	dsPath1 := run.GetPathForDataset(t, 0)
	if ref1 != dsPath1 {
		t.Fatal("ref from first save should match what is in qri repo")
	}

	// Modify body.csv.
	run.MustWriteFile(t, "body.csv", "seven,eight,9\n")

	// Save the new dataset.
	output = run.MustExec(t, "qri save")
	ref2 := parsePathFromRef(parseRefFromSave(output))
	dsPath2 := run.GetPathForDataset(t, 0)
	if ref2 != dsPath2 {
		t.Fatal("ref from second save should match what is in qri repo")
	}

	// Remove all but --keep-files
	run.MustExec(t, "qri remove --revisions=all --keep-files")

	// Verify that dsref of HEAD is empty
	dsPath3 := run.GetPathForDataset(t, 0)
	if dsPath3 != "" {
		t.Errorf("after delete, ref should be empty, got: %s", dsPath3)
	}

	// Verify the directory contains the files that we expect.
	dirContents := listDirectory(workDir)
	expectContents := []string{"body.csv", "meta.json", "structure.json"}
	if diff := cmp.Diff(expectContents, dirContents); diff != "" {
		t.Errorf("directory contents (-want +got):\n%s", diff)
	}
}

// Test removing a linked dataset after the working directory has already been deleted.
func TestRemoveIfWorkingDirectoryIsNotFound(t *testing.T) {
	run := NewFSITestRunner(t, "qri_test_remove_no_wd")
	defer run.Delete()

	workDir := run.CreateAndChdirToWorkDir("remove_no_wd")

	// Init as a linked directory
	run.MustExec(t, "qri init --name remove_no_wd --format csv")

	// Save the new dataset
	run.MustExec(t, "qri save")

	// Go up one directory
	parentDir := filepath.Dir(workDir)
	os.Chdir(parentDir)

	// Remove the working directory
	err := os.RemoveAll(workDir)
	if err != nil {
		t.Fatal(err)
	}

	// Remove all should still work, even though the working directory is gone.
	if err = run.ExecCommand("qri remove --revisions=all me/remove_no_wd"); err != nil {
		t.Error(err)
	}
}

// Test that a dataset can be removed even if the logbook is missing
func TestRemoveEvenIfLogbookGone(t *testing.T) {
	run := NewFSITestRunner(t, "qri_test_remove_no_logbook")
	defer run.Delete()

	workDir := run.CreateAndChdirToWorkDir("remove_no_logbook")

	// Init as a linked directory
	run.MustExec(t, "qri init --name remove_no_logbook --format csv")

	// Save the new dataset
	run.MustExec(t, "qri save")

	// Go up one directory
	parentDir := filepath.Dir(workDir)
	os.Chdir(parentDir)

	// Remove the logbook
	logbookFile := filepath.Join(run.RepoRoot.RootPath, "qri/logbook.qfb")
	if _, err := os.Stat(logbookFile); os.IsNotExist(err) {
		t.Fatal("logbook does not exist")
	}
	err := os.Remove(logbookFile)
	if err != nil {
		t.Fatal(err)
	}

	// Remove all should still work, even though the logbook is gone.
	if err := run.ExecCommand("qri remove --revisions=all me/remove_no_logbook"); err != nil {
		t.Error(err)
	}
}

// Test that an added dataset can be removed
func TestRemoveEvenIfForeignDataset(t *testing.T) {
	run := NewTestRunnerWithMockRemoteClient(t, "test_peer", "remove_foreign")
	defer run.Delete()

	// Save a foreign dataset
	run.MustExec(t, "qri add other_peer/their_dataset")

	// Remove all should still work, even though the dataset is foreign
	if err := run.ExecCommand("qri remove --revisions=all other_peer/their_dataset"); err != nil {
		t.Error(err)
	}
}

// Test that an added dataset can be removed even if the logbook is missing
func TestRemoveEvenIfForeignDatasetWithNoOplog(t *testing.T) {
	run := NewTestRunnerWithMockRemoteClient(t, "test_peer", "remove_no_oplog")
	defer run.Delete()

	// Save a foreign dataset
	run.MustExec(t, "qri add other_peer/their_dataset")

	// Remove the logbook
	logbookFile := filepath.Join(run.RepoRoot.RootPath, "qri/logbook.qfb")
	if _, err := os.Stat(logbookFile); os.IsNotExist(err) {
		t.Fatal("logbook does not exist")
	}
	err := os.Remove(logbookFile)
	if err != nil {
		t.Fatal(err)
	}

	// Remove all should still work, even though the dataset is foreign with no logbook
	if err := run.ExecCommand("qri remove --revisions=all other_peer/their_dataset"); err != nil {
		t.Error(err)
	}
}
