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

package types

import (
	"encoding/hex"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var fieldsDHCPv4 = []string{
	"Timestamp",    // string
	"Operation",    // int32
	"HardwareType", // int32
	"HardwareLen",  // int32
	"HardwareOpts", // int32
	"Xid",          // uint32
	"Secs",         // int32
	"Flags",        // int32
	"ClientIP",     // string
	"YourClientIP", // string
	"NextServerIP", // string
	"RelayAgentIP", // string
	"ClientHWAddr", // string
	"ServerName",   // []byte
	"File",         // []byte
	"Options",      // []*DHCPOption
	"SrcIP",
	"DstIP",
	"SrcPort",
	"DstPort",
}

// CSVHeader returns the CSV header for the audit record.
func (d *DHCPv4) CSVHeader() []string {
	return filter(fieldsDHCPv4)
}

// CSVRecord returns the CSV record for the audit record.
func (d *DHCPv4) CSVRecord() []string {
	var opts []string
	for _, o := range d.Options {
		opts = append(opts, o.toString())
	}

	return filter([]string{
		formatTimestamp(d.Timestamp),     // string
		formatInt32(d.Operation),         // int32
		formatInt32(d.HardwareType),      // int32
		formatInt32(d.HardwareLen),       // int32
		formatInt32(d.HardwareOpts),      // int32
		formatUint32(d.Xid),              // uint32
		formatInt32(d.Secs),              // int32
		formatInt32(d.Flags),             // int32
		d.ClientIP,                       // string
		d.YourClientIP,                   // string
		d.NextServerIP,                   // string
		d.RelayAgentIP,                   // string
		d.ClientHWAddr,                   // string
		hex.EncodeToString(d.ServerName), // []byte
		hex.EncodeToString(d.File),       // []byte
		strings.Join(opts, ""),           // []*DHCPOption
		d.SrcIP,
		d.DstIP,
		formatInt32(d.SrcPort),
		formatInt32(d.DstPort),
	})
}

// Time returns the timestamp associated with the audit record.
func (d *DHCPv4) Time() int64 {
	return d.Timestamp
}

func (d DHCPOption) toString() string {
	var b strings.Builder
	b.WriteString(StructureBegin)
	b.WriteString(formatInt32(d.Type))
	b.WriteString(FieldSeparator)
	b.WriteString(formatInt32(d.Length))
	b.WriteString(FieldSeparator)
	b.WriteString(d.Data)
	b.WriteString(StructureEnd)
	return b.String()
}

// JSON returns the JSON representation of the audit record.
func (d *DHCPv4) JSON() (string, error) {
	// convert unix timestamp from nano to millisecond precision for elastic
	d.Timestamp /= int64(time.Millisecond)

	return jsonMarshaler.MarshalToString(d)
}

var fieldsDHCPv4Metric = []string{
	"Operation",    // int32
	"HardwareType", // int32
	"HardwareLen",  // int32
	"HardwareOpts", // int32
	"Xid",          // uint32
	"Secs",         // int32
	"Flags",        // int32
	"ClientIP",     // string
	"YourClientIP", // string
	"NextServerIP", // string
	"RelayAgentIP", // string
	"ClientHWAddr", // string
	"SrcIP",
	"DstIP",
	"SrcPort",
	"DstPort",
}

var dhcp4Metric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: strings.ToLower(Type_NC_DHCPv4.String()),
		Help: Type_NC_DHCPv4.String() + " audit records",
	},
	fieldsDHCPv4Metric,
)

func (d *DHCPv4) metricValues() []string {
	return []string{
		formatInt32(d.Operation),
		formatInt32(d.HardwareType),
		formatInt32(d.HardwareLen),
		formatInt32(d.HardwareOpts),
		formatUint32(d.Xid),
		formatInt32(d.Secs),
		formatInt32(d.Flags),
		d.ClientIP,
		d.YourClientIP,
		d.NextServerIP,
		d.RelayAgentIP,
		d.ClientHWAddr,
		d.SrcIP,
		d.DstIP,
		formatInt32(d.SrcPort),
		formatInt32(d.DstPort),
	}
}

// Inc increments the metrics for the audit record.
func (d *DHCPv4) Inc() {
	dhcp4Metric.WithLabelValues(d.metricValues()...).Inc()
}

// SetPacketContext sets the associated packet context for the audit record.
func (d *DHCPv4) SetPacketContext(ctx *PacketContext) {
	d.SrcIP = ctx.SrcIP
	d.DstIP = ctx.DstIP
	d.SrcPort = ctx.SrcPort
	d.DstPort = ctx.DstPort
}

// Src returns the source address of the audit record.
func (d *DHCPv4) Src() string {
	return d.SrcIP
}

// Dst returns the destination address of the audit record.
func (d *DHCPv4) Dst() string {
	return d.DstIP
}
