package fsrepo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/qri-io/qfs"
	"github.com/qri-io/qfs/cafs"
	"github.com/qri-io/qri/config"
	"github.com/qri-io/qri/dscache"
	"github.com/qri-io/qri/logbook"
	"github.com/qri-io/qri/repo"
	"github.com/qri-io/qri/repo/profile"
	"github.com/qri-io/qri/repo/test/spec"
)

func TestRepo(t *testing.T) {
	path := filepath.Join(os.TempDir(), "qri_repo_test")

	rmf := func(t *testing.T) (repo.Repo, func()) {
		if err := os.RemoveAll(path); err != nil {
			t.Fatalf("error removing files: %q", err)
		}

		pro, err := profile.NewProfile(config.DefaultProfileForTesting())
		if err != nil {
			t.Fatal(err)
		}

		fs := qfs.NewMemFS()
		book, err := logbook.NewJournal(pro.PrivKey, pro.Peername, fs, path)
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.Background()
		cache := dscache.NewDscache(ctx, fs, "")

		store := cafs.NewMapstore()
		r, err := NewRepo(store, fs, book, cache, pro, path)
		if err != nil {
			t.Fatalf("error creating repo: %s", err.Error())
		}

		cleanup := func() {
			if err := os.RemoveAll(path); err != nil {
				t.Errorf("error cleaning up after test: %s", err)
			}
		}

		return r, cleanup
	}

	spec.RunRepoTests(t, rmf)

	if err := os.RemoveAll(path); err != nil {
		t.Errorf("error cleaning up after test: %s", err.Error())
	}
}
