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

// Master election algorithm which uses Doozer as a lock server to
// determine whether or not a process is master.
package masterelection

import (
	"net"

	"github.com/ha/doozer"
)

// Interface for notifying the caller about changes in the master state.
type MasterElectionEventReceiver interface {
	// BecomeMaster() will be invoked when the process has been elected as
	// master. If an error is returned by BecomeMaster(), a new master
	// election will be forced.
	BecomeMaster() error

	// BecomeSlave() will be invoked every time the master election is lost.
	// It may also be invoked right before BecomeMaster() in case a master
	// election is forced. The name of the new master will be passed as
	// a host:port pair.
	//
	// It will also be inoked even if the process is already a slave, but
	// another master election has taken place. Due to this property,
	// BecomeSlave() may be used to receive notifications about changes
	// of the master.
	BecomeSlave(new_master string)

	// This callback will be invoked to report non-fatal errors in the
	// master election process to the client.
	ElectionError(err error)

	// This callback will be invoked to report fatal errors in the master
	// election process to the client.
	ElectionFatal(err error)
}

// Client for the master election procedure using a Doozer lock server.
// This can be used both in participating mode, where the process can become
// master, or in passive mode, where the current master may be discovered
// through the API and master elections may be forced, but the process will
// not participate in master elections and thus never itself become master.
type MasterElectionClient struct {
	conn          *doozer.Conn
	participating bool
	own_addr      net.Addr
	old_rev       int64
	cb            MasterElectionEventReceiver
	path          string
}

// Create a new master election client for the elections with the given
// "name". The host and port of the master will be set to "addr".
// If "participating" is set to true, the client will participate in master
// elections, otherwise the client will just listen for changes of the
// current master.
//
// All notifications of being a master or slave will be done on the
// specified "callback".
func NewMasterElectionClient(conn *doozer.Conn, name string, addr net.Addr,
	participating bool, callback MasterElectionEventReceiver) (
	*MasterElectionClient, error) {
	var ret *MasterElectionClient = &MasterElectionClient{
		cb:            callback,
		conn:          conn,
		participating: participating,
		own_addr:      addr,
		path:          "/ns/service/master/" + name,
	}

	go ret.run()
	return ret, nil
}

// This is essentially the main loop of the master election process.
// Changes to the master election file are detected and reported.
// If we want to be eligible as masters, participation in master
// elections will also take place here.
func (m *MasterElectionClient) run() {
	var data []byte
	var err error

	data, m.old_rev, err = m.conn.Get(m.path, nil)
	if err == doozer.ErrNoEnt {
		m.old_rev, err = m.conn.Rev()
		if err != nil {
			m.cb.ElectionFatal(err)
			return
		} else if m.participating {
			// There's no master, we'll have to find one.
			m.runMasterElection()
		}
	} else if err != nil {
		m.cb.ElectionFatal(err)
		return
	} else {
		m.cb.BecomeSlave(string(data))
	}

	data = make([]byte, 0) // clear data

	for {
		var ev doozer.Event
		ev, err = m.conn.Wait(m.path, m.old_rev)
		if err != nil {
			m.cb.ElectionError(err)
			continue
		}

		// Make sure our path matches exactly
		if ev.Path != m.path {
			continue
		}

		if ev.IsDel() && m.participating {
			// Master election has been forced.
			m.runMasterElection()
		} else if ev.IsSet() {
			// We're just receiving a master update.
			m.cb.BecomeSlave(string(ev.Body))
		}

		// Update our idea of the revision.
		m.old_rev = ev.Rev
	}
}

// Attempt to be elected as a master.
func (m *MasterElectionClient) runMasterElection() {
	var new_master []byte
	var rev int64
	var err error

	rev, err = m.conn.Set(m.path, m.old_rev, []byte(m.own_addr.String()))
	if err == nil {
		err = m.cb.BecomeMaster()
		if err != nil {
			m.old_rev = rev

			// We failed to become master, so we must force a new election.
			m.ForceMasterElection()
			return
		}

		// We are now a new master!
		m.old_rev = rev
		return
	} else if err != doozer.ErrTooLate {
		m.cb.ElectionError(err)
	}

	// Let's do a read-current.
	new_master, rev, err = m.conn.Get(m.path, nil)
	if err != nil {
		m.cb.ElectionError(err)
		return
	}
	m.old_rev = rev
	m.cb.BecomeSlave(string(new_master))
}

// Force a master election to take place right now.
func (m *MasterElectionClient) ForceMasterElection() error {
	var err error = m.conn.Del(m.path, m.old_rev)
	if err != nil && err != doozer.ErrTooLate {
		m.cb.ElectionError(err)
	}
	return err
}
