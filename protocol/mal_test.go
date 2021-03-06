// Copyright 2017, Yahoo Holdings Inc.
// Licensed under the terms of the Apache license. See LICENSE file in project root for terms.

package protocol

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/yahoo/bftkv/crypto/pgp"
	"github.com/yahoo/bftkv/node"
	"github.com/yahoo/bftkv/node/graph"
	"github.com/yahoo/bftkv/quorum/wotqs"
	"github.com/yahoo/bftkv/transport"
	transport_http "github.com/yahoo/bftkv/transport/http"
)

var _ = fmt.Println

func TestMaliciousCollusion(t *testing.T) {
	t.Skip("skip failing test - FIXME")

	// malicious client writes <x, t, v> and <x, t, v'> to colluding servers
	// read is attempted for key x => insufficient responses expected
	sMal = []string{"http://localhost:5705", "http://localhost:5708", "http://localhost:5709", "http://localhost:5706", "http://localhost:5707", "http://localhost:5704", "http://localhost:5703"}
	rwMal = []string{"http://localhost:5602", "http://localhost:5603", "http://localhost:5604", "http://localhost:5605", "http://localhost:5606", "http://localhost:5607", "http://localhost:5608"}
	mal = append(sMal, rwMal...) // used in malserver_test.go
	files, err := ioutil.ReadDir(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	// create test servers
	var servers []*MalServer
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "a") || strings.HasPrefix(f.Name(), "rw") {
			s := newMalServer(keyPath+"/"+f.Name(), dbPrefix+f.Name())
			if err := s.Start(); err != nil {
				t.Fatal(err)
			}
			servers = append(servers, s)
		}
	}

	defer func() {
		for _, s := range servers {
			s.Stop()
		}
	}()

	// create a client
	c := newClient(keyPath + "/" + clientKey)
	c.Joining()
	defer c.Leaving()

	key := []byte(testKey)
	value := []byte("honesttestvalue")
	if err := c.WriteMal(key, value); err != nil {
		t.Fatal(err)
	}

	res, err := c.Read(key, nil)
	time.Sleep(time.Second * 3) // sleep to leave time for revoke check
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Key: %s, Value: %s\n", string(key), string(res))
}

// to test tofu --- run 'go test -run=TOFU -v'

func TestTOFU(t *testing.T) {
	t.Skip("skip failing test - FIXME")

	// tests must run in following order

	//unique time stamp is key
	uts := time.Now().String()

	// exp: successful --- the client will write <x, t, v> for the first time
	// exp: successful --- the client will wrtie <x, t', v'>
	servers := runServers(t, "a", "rw")
	c1 := newClient(keyPath + "/u01")
	c1.Joining()
	defer c1.Leaving()
	c1.checkTofu(uts, t, "original write successful")
	c1.checkTofu(uts, t, "self overwrite successful")
	stopServers(servers)
	c1.Leaving()

	// exp: permission denied --- diff user id
	servers = runServers(t, "a", "rw")
	c2 := newClient(keyPath + "/u02")
	c2.Joining()
	defer c2.Leaving()
	c2.checkTofu(uts, t, "trusted entity overwrite successful (same UId)")
	stopServers(servers)
	c2.Leaving()

	// exp: permission denied --- diff servers will sign for c01 than u1
	servers = runServers(t, "a", "rw")
	c3 := newClient(keyPath + "/u04")
	c3.Joining()
	defer c3.Leaving()
	c3.checkTofu(uts, t, "untrusted entity overwrite successful - expected error")
	stopServers(servers)
	c3.Leaving()
}

func (c *Client) checkTofu(key string, t *testing.T, exp string) {
	if err := c.Write([]byte(key), []byte(testValue), nil); err != nil {
		t.Log(err)
	} else {
		t.Log(exp)
	}
}

func newMalClient(path string) *Client {
	crypt := pgp.New()
	g := graph.New()
	readCerts(g, crypt, path+"/pubring.gpg", false)
	readCerts(g, crypt, path+"/secring.gpg", true)
	qs := wotqs.New(g)
	var tr transport.Transport
	tr = transport_http.New(crypt)
	return NewClient(node.SelfNode(g), qs, tr, crypt)
}

func newMalServer(path string, dbPath string) *MalServer {
	crypt := pgp.New()
	g := graph.New()
	readCerts(g, crypt, path+"/pubring.gpg", false)
	readCerts(g, crypt, path+"/secring.gpg", true)
	qs := wotqs.New(g)
	var tr transport.Transport
	tr = transport_http.MalNew(crypt)
	storage := MalStorageNew(dbPath)
	return NewMalServer(node.SelfNode(g), qs, tr, crypt, storage)
}
