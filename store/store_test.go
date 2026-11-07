package store

import (
	"context"
	"os"
	"path/filepath"
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
