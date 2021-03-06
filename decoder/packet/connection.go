/*
 * NETCAP - Traffic Analysis Framework
 * Copyright (c) 2017-2020 Philipp Mieden <dreadl0ck [at] protonmail [dot] ch>
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package packet

import (
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dreadl0ck/gopacket"
	"github.com/gogo/protobuf/proto"

	"github.com/dreadl0ck/netcap/types"
)

// connectionID is a bidirectional connection
// between two devices over the network
// that includes the Link, Network and TransportLayer.
type connectionID struct {
	LinkFlowID      uint64
	NetworkFlowID   uint64
	TransportFlowID uint64
}

func (c connectionID) String() string {
	return strconv.FormatUint(c.LinkFlowID, 10) + strconv.FormatUint(c.NetworkFlowID, 10) + strconv.FormatUint(c.TransportFlowID, 10)
}

type connection struct {
	sync.Mutex
	*types.Connection
}

// atomicConnMap contains all connections and provides synchronized access.
type atomicConnMap struct {
	sync.Mutex
	Items map[string]*connection
}

// Size returns the number of elements in the Items map.
func (a *atomicConnMap) Size() int {
	a.Lock()
	defer a.Unlock()

	return len(a.Items)
}

var conns = &atomicConnMap{
	Items: make(map[string]*connection),
}

var connectionDecoder = newPacketDecoder(
	types.Type_NC_Connection,
	"Connection",
	"A connection represents bi-directional network communication between two hosts based on the combined link-, network- and transport layer identifiers",
	nil,
	func(p gopacket.Packet) proto.Message {
		return handlePacket(p)
	},
	func(decoder *Decoder) error {
		conns.Lock()
		for _, f := range conns.Items {
			f.Lock()
			decoder.writeConn(f.Connection)
			f.Unlock()
		}
		conns.Unlock()

		return nil
	},
)

func handlePacket(p gopacket.Packet) proto.Message {
	// assemble connectionID
	connID := connectionID{}
	if ll := p.LinkLayer(); ll != nil {
		connID.LinkFlowID = ll.LinkFlow().FastHash()
	}

	if nl := p.NetworkLayer(); nl != nil {
		connID.NetworkFlowID = nl.NetworkFlow().FastHash()
	}

	if tl := p.TransportLayer(); tl != nil {
		connID.TransportFlowID = tl.TransportFlow().FastHash()
	}

	// lookup flow
	conns.Lock()

	if conn, ok := conns.Items[connID.String()]; ok {

		conn.Lock()

		// check if received packet from the same connection
		// was captured BEFORE the connections first seen timestamp
		if p.Metadata().Timestamp.Before(time.Unix(0, conn.TimestampFirst).UTC()) {

			// rewrite timestamp
			conn.TimestampFirst = p.Metadata().Timestamp.UnixNano()

			// rewrite source and destination parameters
			// since the first packet decides about the connection direction
			if ll := p.LinkLayer(); ll != nil {
				conn.SrcMAC = ll.LinkFlow().Src().String()
				conn.DstMAC = ll.LinkFlow().Dst().String()
			}

			if nl := p.NetworkLayer(); nl != nil {
				conn.SrcIP = nl.NetworkFlow().Src().String()
				conn.DstIP = nl.NetworkFlow().Dst().String()
			}

			if tl := p.TransportLayer(); tl != nil {
				conn.SrcPort = tl.TransportFlow().Src().String()
				conn.DstPort = tl.TransportFlow().Dst().String()
			}
		}

		if al := p.ApplicationLayer(); al != nil {
			conn.AppPayloadSize += int32(len(al.Payload()))
		}

		// check if last timestamp was before the current packet
		if conn.TimestampLast < p.Metadata().Timestamp.UnixNano() {
			// current packet is newer
			// update last seen timestamp
			conn.TimestampLast = p.Metadata().Timestamp.UnixNano()
		} // else: do nothing, timestamp is still the oldest one

		conn.NumPackets++
		conn.TotalSize += int32(p.Metadata().Length)

		conn.Unlock()
	} else { // create a new Connection
		co := &types.Connection{}
		co.UID = calcMd5(connID.String())
		co.TimestampFirst = p.Metadata().Timestamp.UnixNano()
		co.TimestampLast = p.Metadata().Timestamp.UnixNano()
		co.TotalSize = int32(p.Metadata().Length)
		co.NumPackets = 1

		if ll := p.LinkLayer(); ll != nil {
			co.LinkProto = ll.LayerType().String()
			co.SrcMAC = ll.LinkFlow().Src().String()
			co.DstMAC = ll.LinkFlow().Dst().String()
		}
		if nl := p.NetworkLayer(); nl != nil {
			co.NetworkProto = nl.LayerType().String()
			co.SrcIP = nl.NetworkFlow().Src().String()
			co.DstIP = nl.NetworkFlow().Dst().String()
		}
		if tl := p.TransportLayer(); tl != nil {
			co.TransportProto = tl.LayerType().String()
			co.SrcPort = tl.TransportFlow().Src().String()
			co.DstPort = tl.TransportFlow().Dst().String()
		}
		if al := p.ApplicationLayer(); al != nil {
			co.ApplicationProto = al.LayerType().String()
			co.AppPayloadSize = int32(len(al.Payload()))
		}

		conns.Items[connID.String()] = &connection{
			Connection: co,
		}

		// TODO: add dedicated stats structure for decoder pkg
		// conns := atomic.AddInt64(&stream.stats.numConns, 1)

		// flush
		//if conf.ConnFlushInterval != 0 && conns%int64(conf.ConnFlushInterval) == 0 {
		//	cd.flushConns(p)
		//}
	}
	conns.Unlock()

	return nil
}

/*func flushConns(p gopacket.Packet) {
	var selectConns []*types.Connection

	for id, entry := range conns.Items {

		// flush entries whose last timestamp is connTimeOut older than current packet
		if p.Metadata().Timestamp.Sub(time.Unix(0, entry.TimestampLast)) > conf.ConnTimeOut {

			selectConns = append(selectConns, entry.Connection)

			// cleanup
			delete(conns.Items, id)
		}
	}

	// flush selection in background
	go func() {
		for _, selectedConn := range selectConns {
			writeConn(selectedConn)
		}
	}()
}*/

// writeConn writes the connection.
func (d *Decoder) writeConn(conn *types.Connection) {

	// calculate duration
	conn.Duration = time.Unix(0, conn.TimestampLast).Sub(time.Unix(0, conn.TimestampFirst)).Nanoseconds()

	if conf.ExportMetrics {
		conn.Inc()
	}

	atomic.AddInt64(&d.NumRecordsWritten, 1)

	err := d.Writer.Write(conn)
	if err != nil {
		log.Fatal("failed to write proto: ", err)
	}
}
