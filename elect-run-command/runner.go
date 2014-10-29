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

// Runs a command when a given process becomes master or slave.
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"

	masterelection "github.com/caoimhechaos/go-doozer-masterelection"
	"github.com/ha/doozer"
)

type Printer struct {
	self       net.Addr
	master_cmd string
	slave_cmd  string
	error_cmd  string
}

func (p *Printer) BecomeMaster() error {
	log.Print(p.self, ": Became master")
	if p.master_cmd != "" {
		var cmd *exec.Cmd = exec.Command(p.master_cmd)
		var out []byte
		var err error

		os.Setenv("MASTER", p.self.String())
		out, err = cmd.CombinedOutput()
		if err != nil {
			log.Print("Error getting output from error command: ", err)
		} else {
			log.Print(string(out))
		}
	}
	return nil
}

func (p *Printer) BecomeSlave(new_master string) {
	log.Print(p.self, ": Became slave (", new_master, " is master)")
	if p.slave_cmd != "" {
		var cmd *exec.Cmd = exec.Command(p.slave_cmd)
		var out []byte
		var err error

		os.Setenv("MASTER", new_master)
		out, err = cmd.CombinedOutput()
		if err != nil {
			log.Print("Error getting output from error command: ", err)
		} else {
			log.Print(string(out))
		}
	}
}

func (p *Printer) ElectionError(err error) {
	log.Print(p.self, ": election error: ", err)
	if p.error_cmd != "" {
		var cmd *exec.Cmd = exec.Command(p.error_cmd, err.Error())
		var out []byte
		var err error
		out, err = cmd.CombinedOutput()
		if err != nil {
			log.Print("Error getting output from error command: ", err)
		} else {
			log.Print(string(out))
		}
	}
}

func (p *Printer) ElectionFatal(err error) {
	if p.error_cmd != "" {
		var cmd *exec.Cmd = exec.Command(p.error_cmd, err.Error())
		var out []byte
		var err error
		out, err = cmd.CombinedOutput()
		if err != nil {
			log.Print("Error getting output from error command: ", err)
		} else {
			log.Print(string(out))
		}
	}
	log.Fatal(p.self, ": fatal election error: ", err.Error())
}

func main() {
	var conn *doozer.Conn
	var doozer_buri, doozer_uri string
	var name, addr string
	var master_cmd, slave_cmd, error_cmd string
	var naddr net.Addr
	var participating, force_election bool
	var me *masterelection.MasterElectionClient
	var err error

	flag.StringVar(&name, "name", "test",
		"Name of the master election target to participate in")
	flag.BoolVar(&participating, "participate", false,
		"Whether to participate in master elections")
	flag.BoolVar(&force_election, "force-election", false,
		"Force an election when starting")

	flag.StringVar(&addr, "address",
		"[::1]:"+strconv.FormatInt(int64(os.Getpid()), 10),
		"Fake address to provide as server address to the lock server")

	flag.StringVar(&master_cmd, "master-cmd", "",
		"Command which should be run when the service becomes a master. "+
			"If empty, no script will be run.")
	flag.StringVar(&slave_cmd, "slave-cmd", "",
		"Command which should be run when the service becomes slave. "+
			"The master will be set as an environment variable MASTER. "+
			"If empty, no script will be run.")
	flag.StringVar(&error_cmd, "error-cmd", "",
		"Command which should be run when there are errors, with the "+
			"error as a parameter. If empty, errors will merely be logged")

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
		conn, name, naddr, participating, &Printer{
			self:       naddr,
			master_cmd: master_cmd,
			slave_cmd:  slave_cmd,
			error_cmd:  error_cmd,
		})
	if err != nil {
		log.Fatal("Error setting up master election client: ", err)
	}

	if force_election {
		me.ForceMasterElection()
	}

	me.SyncWait()
}
