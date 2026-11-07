package caddyferretdb

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func freeAddr(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

// A real Caddy opens a named database on TCP, and a MongoDB client - as mongodump
// or mongosh would - writes and reads it.
func TestCaddyServesADatabaseOnTCP(t *testing.T) {
	addr := freeAddr(t)
	cfg := map[string]any{"admin": map[string]any{"disabled": true},
		"apps": map[string]any{"ferretdb": map[string]any{"databases": map[string]any{
			"wekan": map[string]any{"dir": t.TempDir(), "listen": addr}}}}}
	raw, _ := json.Marshal(cfg)
	if err := caddy.Load(raw, true); err != nil {
		t.Fatal(err)
	}
	defer caddy.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := mongo.Connect(options.Client().ApplyURI("mongodb://" + addr + "/wekan"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Disconnect(ctx)
	if _, err := c.Database("wekan").Collection("cards").InsertOne(ctx, bson.M{"title": "from a MongoDB tool"}); err != nil {
		t.Fatal(err)
	}
	n, err := c.Database("wekan").Collection("cards").CountDocuments(ctx, bson.M{})
	if err != nil || n != 1 {
		t.Fatalf("count %d %v", n, err)
	}
}

func TestCaddyfile(t *testing.T) {
	adapter := caddyfile.Adapter{ServerType: httpcaddyfile.ServerType{}}
	out, _, err := adapter.Adapt([]byte("{\n\tferretdb {\n\t\tdatabase wekan /srv/db {\n\t\t\tlisten 127.0.0.1:27017\n\t\t}\n\t\tdatabase other /srv/other\n\t}\n}\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"ferretdb":{"databases":{"other":{"dir":"/srv/other"},"wekan":{"dir":"/srv/db","listen":"127.0.0.1:27017"}}}`) {
		t.Fatalf("adapted: %s", out)
	}
	for name, bad := range map[string]string{
		"an unknown option":  "{\n\tferretdb {\n\t\tdrop wekan\n\t}\n}\n",
		"a database twice":   "{\n\tferretdb {\n\t\tdatabase a /x\n\t\tdatabase a /y\n\t}\n}\n",
		"a database, no dir": "{\n\tferretdb {\n\t\tdatabase a\n\t}\n}\n",
	} {
		if _, _, err := adapter.Adapt([]byte(bad), nil); err == nil {
			t.Errorf("negative: %s must be refused", name)
		}
	}
}
