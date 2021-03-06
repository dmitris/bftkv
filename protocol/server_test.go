// Copyright 2017, Yahoo Holdings Inc.
// Licensed under the terms of the Apache license. See LICENSE file in project root for terms.

package protocol

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/yahoo/bftkv/crypto"
	"github.com/yahoo/bftkv/crypto/pgp"
	"github.com/yahoo/bftkv/node"
	"github.com/yahoo/bftkv/node/graph"
	"github.com/yahoo/bftkv/quorum/wotqs"
	storage_plain "github.com/yahoo/bftkv/storage/plain"
	"github.com/yahoo/bftkv/transport"
	transport_http "github.com/yahoo/bftkv/transport/http"
)

const (
	scriptPath      = "../scripts" // any way to specify the absolute path?
	keyPath         = scriptPath + "/run/keys"
	serverKeyPrefix = "a"
	clientKey       = "u01"
	dbPrefix        = scriptPath + "/run/db."
	testKey         = "test"
	testValue       = "test"
)

func TestServer(t *testing.T) {
	t.Skip("skip failing test - FIXME")

	servers := runServers(t, "a", "rw")

	defer func(servers []*Server) {
		stopServers(servers)
	}(servers)

	// create a client
	c := newClient(keyPath + "/" + clientKey)
	c.Joining()

	key := []byte(testKey)
	value := []byte(testValue)
	if err := c.Write(key, value, nil); err != nil {
		t.Fatal(err)
	}
	res, err := c.Read(key, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(res, value) {
		t.Errorf("Got %v\n", res)
	}
}

func newServer(path string, dbPath string) *Server {
	crypt := pgp.New()
	g := graph.New()
	readCerts(g, crypt, path+"/pubring.gpg", false)
	readCerts(g, crypt, path+"/secring.gpg", true)
	qs := wotqs.New(g)
	var tr transport.Transport
	tr = transport_http.New(crypt)
	storage := storage_plain.New(dbPath)
	return NewServer(node.SelfNode(g), qs, tr, crypt, storage)
}

func newClient(path string) *Client {
	crypt := pgp.New()
	g := graph.New()
	readCerts(g, crypt, path+"/pubring.gpg", false)
	readCerts(g, crypt, path+"/secring.gpg", true)
	qs := wotqs.New(g)
	var tr transport.Transport
	tr = transport_http.New(crypt)
	return NewClient(node.SelfNode(g), qs, tr, crypt)
}

func runServers(t *testing.T, prefixes ...string) []*Server {
	files, err := ioutil.ReadDir(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	var servers []*Server
	for _, f := range files {
		for _, prefix := range prefixes {
			if strings.HasPrefix(f.Name(), prefix) {
				s := newServer(keyPath+"/"+f.Name(), dbPrefix+f.Name())
				if err := s.Start(); err != nil {
					t.Fatal(err)
				}
				servers = append(servers, s)
			}
		}
	}

	return servers
}

func stopServers(servers []*Server) {
	for _, s := range servers {
		s.Stop()
	}
}

func readCerts(g *graph.Graph, crypt *crypto.Crypto, path string, sec bool) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
		return
	}
	certs, err := crypt.Certificate.ParseStream(f)
	if err != nil {
		f.Close()
		log.Fatal(err)
	}
	if sec {
		g.SetSelfNodes(certs)
	} else {
		g.AddNodes(certs)
	}
	crypt.Keyring.Register(certs, sec, true)
	f.Close()
}
