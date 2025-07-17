package kattis

import (
	"context"
	// "fmt"
	// "fmt"
	// "net/url"
	// "os"
	// "sort"
	"path/filepath"
	"testing"

	. "github.com/eolymp/go-problems/connector/testing"
	atlaspb "github.com/eolymp/go-sdk/eolymp/atlas"
	// ecmpb "github.com/eolymp/go-sdk/eolymp/ecm"
	// executorpb "github.com/eolymp/go-sdk/eolymp/executor"
	// "github.com/google/go-cmp/cmp"
	// "google.golang.org/protobuf/proto"
)

// used to test the problem packages in the kattis directory
func (p *ProblemLoader) testFetch(ctx context.Context, link string) (*atlaspb.Snapshot, error) {
	return p.Snapshot(ctx, link) // nothing to download/unpack
}

func TestProblemLoader_Snapshot_maximal(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	ctx := context.Background()
	URL := filepath.Join("problems", "maximal")

	ldr := NewProblemLoader(MockUploader(), MockLogger(t))
	snap, err := ldr.testFetch(ctx, URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	t.Run("statements", func(t *testing.T) {
		if n := len(snap.GetStatements()); n == 0 {
			t.Fatalf("want >=1 statement, got 0")
		}
		for _, st := range snap.GetStatements() {
			if st.GetLocale() == "" || st.GetTitle() == "" || st.GetContent() == nil {
				t.Errorf("incomplete statement: %+v", st)
			}
		}
	})

	t.Run("tests", func(t *testing.T) {
		if got := len(snap.GetTests()); got != 4 {
			t.Fatalf("want 4 tests, got %d", got)
		}
	})

	t.Run("validator", func(t *testing.T) {
		if snap.GetValidator() == nil {
			t.Fatalf("maximal should import a validator")
		}
	})
}

func TestProblemLoader_Snapshot_passfail(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	ctx := context.Background()
	URL := filepath.Join("problems", "passfail")

	ldr := NewProblemLoader(MockUploader(), MockLogger(t))
	snap, err := ldr.testFetch(ctx, URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	t.Run("tests", func(t *testing.T) {
		if got := len(snap.GetTests()); got != 3 {
			t.Fatalf("want 3 tests, got %d", got)
		}
	})

	t.Run("statements", func(t *testing.T) {
		if n := len(snap.GetStatements()); n == 0 {
			t.Fatalf("want >=1 statement, got 0")
		}
		for _, st := range snap.GetStatements() {
			if st.GetLocale() == "" || st.GetTitle() == "" || st.GetContent() == nil {
				t.Errorf("incomplete statement: %+v", st)
			}
		}
	})

	t.Run("validator", func(t *testing.T) {
		if snap.GetValidator() == nil {
			t.Fatalf("passfail should import a validator")
		}
	})

	t.Run("checker", func(t *testing.T) {
		if snap.GetChecker() == nil {
			t.Fatalf("passfail should import a checker")
		}
	})
}

func TestProblemLoader_Snapshot_scoring(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	ctx := context.Background()
	URL := filepath.Join("problems", "scoring")

	ldr := NewProblemLoader(MockUploader(), MockLogger(t))
	snap, err := ldr.testFetch(ctx, URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	t.Run("tests", func(t *testing.T) {
		if got := len(snap.GetTests()); got != 6 {
			t.Fatalf("want 6 tests, got %d", got)
		}
	})

	t.Run("statements", func(t *testing.T) {
		if n := len(snap.GetStatements()); n == 0 {
			t.Fatalf("want >=1 statement, got 0")
		}
		for _, st := range snap.GetStatements() {
			if st.GetLocale() == "" || st.GetTitle() == "" || st.GetContent() == nil {
				t.Errorf("incomplete statement: %+v", st)
			}
		}
	})

	t.Run("validator", func(t *testing.T) {
		if snap.GetValidator() == nil {
			t.Fatalf("scoring should import a validator")
		}
	})
}

func TestProblemLoader_Snapshot_submit_answer(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	ctx := context.Background()
	URL := filepath.Join("problems", "submit_answer")

	ldr := NewProblemLoader(MockUploader(), MockLogger(t))
	snap, err := ldr.testFetch(ctx, URL)
	t.Run("expect-error", func(t *testing.T) {
		if err == nil {
			t.Fatalf("want error because data/secret missing, got <nil>")
		}
	})
	t.Run("tests", func(t *testing.T) {
		if got := len(snap.GetTests()); got != 0 {
			t.Fatalf("want 0 tests, got %d", got)
		}
	})
}
