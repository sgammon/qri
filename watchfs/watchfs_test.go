package watchfs

import (
	"github.com/google/go-cmp/cmp"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesysWatcher(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "watchfs")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	// Create a directory, and watch it
	watchdir := filepath.Join(tmpdir, "watch_me")
	_ = os.Mkdir(watchdir, 0755)
	w := NewFilesysWatcher()
	messages := w.Begin([]string{watchdir})
	target := filepath.Join(watchdir, "body.csv")

	// Write a file to the watched directory, get event
	if err := ioutil.WriteFile(target, []byte("test"), os.FileMode(0644)); err != nil {
		t.Fatal(err)
	}
	got := <-messages

	expect := FilesysEvent{
		Type:        CreateNewFileEvent,
		Source:      target,
		Destination: "",
		Time:        got.Time,
	}
	if diff := cmp.Diff(expect, got); diff != "" {
		t.Errorf("filesys event (-want +got):\n%s", diff)
	}
}
