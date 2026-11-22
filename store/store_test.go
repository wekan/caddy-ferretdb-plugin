package store

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestOpenWriteReadReopen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "files", "db")
	ctx := context.Background()
	s, err := Open(ctx, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Collection("boards").InsertOne(ctx, bson.M{"_id": "b1", "title": "Weddy"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "wekan.sqlite")); err != nil {
		t.Fatalf("the database is files/db/wekan.sqlite: %v", err)
	}
	s, err = Open(ctx, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var b struct{ Title string }
	if err := s.DB.Collection("boards").FindOne(ctx, bson.M{"_id": "b1"}).Decode(&b); err != nil || b.Title != "Weddy" {
		t.Fatalf("after reopening: %+v %v", b, err)
	}
}

func TestAcquireShares(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	ctx := context.Background()
	a, releaseA, err := Acquire(ctx, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, releaseB, err := Acquire(ctx, dir+"/.", nil)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("negative: two FerretDBs over one directory would write the same SQLite files")
	}
	if err := releaseA(); err != nil {
		t.Fatal(err)
	}
	_ = releaseA() // a second release of the same holder does nothing
	if err := b.Client.Ping(ctx, nil); err != nil {
		t.Fatalf("still open for the other holder: %v", err)
	}
	if err := releaseB(); err != nil {
		t.Fatal(err)
	}
	if b.Client.Ping(ctx, nil) == nil {
		t.Fatal("negative: the last release closes it")
	}
}

func TestSQLiteURLKeepsTheDirectory(t *testing.T) {
	for abs, want := range map[string]string{
		"/var/lib/weddy/files/db": "/var/lib/weddy/files/db/",
		"/tmp/a b/c%d":            "/tmp/a b/c%d/",
		`C:/Users/RUNNER~1/AppData/Local/Temp/T/001/db`: "C:/Users/RUNNER~1/AppData/Local/Temp/T/001/db/",
	} {
		got := sqliteURL(abs)
		u, err := url.Parse(got)
		if err != nil {
			t.Fatalf("%s: %s does not parse: %v", abs, got, err)
		}
		if u.Scheme != "file" || u.Host != "" || u.User != nil {
			t.Fatalf("%s: %s must be a file: URI without a host", abs, got)
		}
		path := u.Path
		if path == "" {
			path = u.Opaque
		}
		if path != want {
			t.Fatalf("%s: %s is the directory %q, not %q", abs, got, path, want)
		}
	}
	// Negative: the old file://C:%5C... form, whose drive letter is a host.
	if _, err := url.Parse((&url.URL{Scheme: "file", Path: `C:\Users\x` + "/"}).String()); err == nil {
		t.Fatal("negative: the old form was expected to be refused, so this test would prove nothing")
	}
}

func TestTheSocketIsNotInAURI(t *testing.T) {
	src, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), `"mongodb://"`) || strings.Contains(string(src), "ApplyURI(") {
		t.Fatal("negative: a mongodb:// URI cannot carry a Windows socket path; use SetHosts")
	}
}
