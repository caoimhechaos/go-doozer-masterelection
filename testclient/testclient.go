/**
 * (c) 2014, Caoimhe Chaos <caoimhechaos@protonmail.com>,
 *	     Ancient Solutions. All rights reserved.
 *
 * Redistribution and use in source  and binary forms, with or without
 * modification, are permitted  provided that the following conditions
 * are met:
 *
 * * Redistributions of  source code  must retain the  above copyright
 *   notice, this list of conditions and the following disclaimer.
 * * Redistributions in binary form must reproduce the above copyright
 *   notice, this  list of conditions and the  following disclaimer in
 *   the  documentation  and/or  other  materials  provided  with  the
 *   distribution.
 * * Neither  the  name  of  Ancient Solutions  nor  the  name  of its
 *   contributors may  be used to endorse or  promote products derived
 *   from this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS"  AND ANY EXPRESS  OR IMPLIED WARRANTIES  OF MERCHANTABILITY
 * AND FITNESS  FOR A PARTICULAR  PURPOSE ARE DISCLAIMED. IN  NO EVENT
 * SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL,  EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED  TO, PROCUREMENT OF SUBSTITUTE GOODS OR
 * SERVICES; LOSS OF USE,  DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
 * STRICT  LIABILITY,  OR  TORT  (INCLUDING NEGLIGENCE  OR  OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED
 * OF THE POSSIBILITY OF SUCH DAMAGE.
 */

package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strconv"

	masterelection "github.com/caoimhechaos/go-doozer-masterelection"
	"github.com/ha/doozer"
)

type Printer struct {
	self net.Addr
}

func (p *Printer) BecomeMaster() error {
	log.Print(p.self, ": Became master")
	return nil
}

func (p *Printer) BecomeSlave(new_master string) {
	log.Print(p.self, ": Became slave (", new_master, " is master)")
}

func (p *Printer) ElectionError(err error) {
	log.Print(p.self, ": election error: ", err)
}

func (p *Printer) ElectionFatal(err error) {
	log.Fatal(p.self, ": fatal election error: ", err.Error())
}

func main() {
	var conn *doozer.Conn
	var doozer_buri, doozer_uri string
	var name string
	var addr string
	var naddr net.Addr
	var participating, force_election, wait bool
	var me *masterelection.MasterElectionClient
	var err error

	flag.StringVar(&name, "name", "test",
		"Name of the master election target to participate in")
	flag.BoolVar(&participating, "participate", false,
		"Whether to participate in master elections")
	flag.BoolVar(&force_election, "force-election", false,
		"Force an election when starting")
	flag.BoolVar(&wait, "wait", true,
		"Wait after connecting (set to false if you only want to force an election)")

	flag.StringVar(&addr, "address",
		"[::1]:"+strconv.FormatInt(int64(os.Getpid()), 10),
		"Fake address to provide as server address to the lock server")

	flag.StringVar(&doozer_buri, "doozer-boot-uri",
		os.Getenv("DOOZER_BOOT_URI"),
		"Doozer boot URI for resolving cluster names in doozer-uri")
	flag.StringVar(&doozer_uri, "doozer-uri",
		os.Getenv("DOOZER_URI"),
		"Doozer URI for finding the Doozer server")
	flag.Parse()

	naddr, err = net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Fatal("Error resolving ", addr, ": ", err)
	}

	conn, err = doozer.DialUri(doozer_uri, doozer_buri)
	if err != nil {
		log.Fatal("Error connecting to Doozer: ", err)
	}

	me, err = masterelection.NewMasterElectionClient(
		conn, name, naddr, participating, &Printer{self: naddr})
	if err != nil {
		log.Fatal("Error setting up master election client: ", err)
	}

	if force_election {
		me.ForceMasterElection()
	}

	if wait {
		me.SyncWait()
	}
}
