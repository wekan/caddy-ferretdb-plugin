// Package store is a database of wekan/FerretDB embedded in Weddy, over a SQLite
// directory - for WeKan, a domain's files/db/, where its database is wekan.sqlite,
// the same files Node.js WeKan with FerretDB uses - reached with the official
// MongoDB Go driver. No second process: FerretDB listens on a Unix socket only
// Weddy knows, and on TCP only when asked to (for MongoDB tools, backups).
//
// Acquire shares one FerretDB per directory between every plugin that asks for
// it - caddy-wekan-plugin's domain, the ferretdb app's named database - so two
// FerretDBs never write the same SQLite files.
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/FerretDB/FerretDB/ferretdb"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DatabaseName is WeKan's database: MONGO_URL .../wekan, wekan.sqlite.
const DatabaseName = "wekan"

// Store is one domain's database.
type Store struct {
	Client *mongo.Client
	DB     *mongo.Database
	cancel context.CancelFunc
	done   chan struct{}
	socket string
}

// socketPath is short on purpose: a Unix socket path is limited to about 100
// bytes, and a domain's directory can be longer than that.
func socketPath(dbDir string) string {
	sum := sha256.Sum256([]byte(dbDir))
	return filepath.Join(os.TempDir(), "weddy-"+hex.EncodeToString(sum[:6])+".sock")
}

// Options of a database.
type Options struct {
	// TCP also listens there, e.g. 127.0.0.1:27017, for MongoDB tools.
	TCP    string
	Logger *slog.Logger
}

// Open starts FerretDB over dbDir and connects to it. Use Acquire unless the
// database is surely nobody else's.
func Open(ctx context.Context, dbDir string, opts *Options) (*Store, error) {
	if opts == nil {
		opts = &Options{}
	}
	logger := opts.Logger
	if err := os.MkdirAll(dbDir, 0o750); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dbDir)
	if err != nil {
		return nil, err
	}
	sock := socketPath(abs)
	_ = os.Remove(sock) // a stale socket of a previous run
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	f, err := ferretdb.New(&ferretdb.Config{
		Listener:  ferretdb.ListenerConfig{Unix: sock, TCP: opts.TCP},
		Handler:   "sqlite",
		SQLiteURL: (&url.URL{Scheme: "file", Path: abs + "/"}).String(),
		Logger:    logger,
	})
	if err != nil {
		return nil, fmt.Errorf("FerretDB over %s: %w", abs, err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = f.Run(runCtx)
	}()
	uri := "mongodb://" + url.PathEscape(sock) + "/" + DatabaseName
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(15 * time.Second))
	if err != nil {
		cancel()
		<-done
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		cancel()
		<-done
		return nil, fmt.Errorf("FerretDB did not answer: %w", err)
	}
	return &Store{Client: client, DB: client.Database(DatabaseName), cancel: cancel, done: done, socket: sock}, nil
}

// Close disconnects and stops FerretDB, which finishes its SQLite writes.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	err := s.Client.Disconnect(context.Background())
	s.cancel()
	select {
	case <-s.done:
	case <-time.After(30 * time.Second):
		err = errors.Join(err, errors.New("FerretDB did not stop in 30 s"))
	}
	_ = os.Remove(s.socket)
	return err
}

var shared struct {
	sync.Mutex
	open map[string]*entry
}

type entry struct {
	store *Store
	refs  int
}

// Acquire gives the database over dbDir - started by the first caller, shared
// with the later ones - and the function that releases it: the last release
// closes it. opts count only for the first caller.
func Acquire(ctx context.Context, dbDir string, opts *Options) (*Store, func() error, error) {
	abs, err := filepath.Abs(dbDir)
	if err != nil {
		return nil, nil, err
	}
	shared.Lock()
	defer shared.Unlock()
	if shared.open == nil {
		shared.open = map[string]*entry{}
	}
	e := shared.open[abs]
	if e == nil {
		s, err := Open(ctx, abs, opts)
		if err != nil {
			return nil, nil, err
		}
		e = &entry{store: s}
		shared.open[abs] = e
	}
	e.refs++
	var once sync.Once
	release := func() (err error) {
		once.Do(func() {
			shared.Lock()
			defer shared.Unlock()
			e.refs--
			if e.refs == 0 {
				delete(shared.open, abs)
				err = e.store.Close()
			}
		})
		return err
	}
	return e.store, release, nil
}
