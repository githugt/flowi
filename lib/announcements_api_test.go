// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/announce"
)

func TestAnnouncements_should_fetch_the_configured_feed_and_cache_it_in_the_config_dir(t *testing.T) {
	priv := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize))
	doc, err := announce.Sign(priv, time.Now(), []announce.Item{{ID: "hello", Title: "Hello"}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(doc) }))
	defer srv.Close()
	key := base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))
	e := newEnv(t, "FLOWI_ANNOUNCEMENTS_URL="+srv.URL, "FLOWI_ANNOUNCEMENTS_KEYS="+key)
	w := e.open(t)

	res, fetchErr := w.Announcements(context.Background(), false)

	if fetchErr != nil || len(res.Items) != 1 || res.Items[0].Title != "Hello" {
		t.Fatalf("res %+v err %v", res, fetchErr)
	}
}

func TestAnnouncements_Given_nothing_configured_Then_ErrNotConfigured(t *testing.T) {
	w := newEnv(t).open(t)

	_, err := w.Announcements(context.Background(), false)

	if !errors.Is(err, announce.ErrNotConfigured) {
		t.Fatalf("err %v", err)
	}
}
