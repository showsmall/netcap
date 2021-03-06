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
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var fieldsNTP = []string{
	"Timestamp",
	"LeapIndicator",
	"Version",
	"Mode",
	"Stratum",
	"Poll",
	"Precision",
	"RootDelay",
	"RootDispersion",
	"ReferenceID",
	"ReferenceTimestamp",
	"OriginTimestamp",
	"ReceiveTimestamp",
	"TransmitTimestamp",
	"ExtensionBytes",
	"SrcIP",
	"DstIP",
	"SrcPort",
	"DstPort",
}

// CSVHeader returns the CSV header for the audit record.
func (n *NTP) CSVHeader() []string {
	return filter(fieldsNTP)
}

// CSVRecord returns the CSV record for the audit record.
func (n *NTP) CSVRecord() []string {
	return filter([]string{
		formatTimestamp(n.Timestamp),
		formatInt32(n.LeapIndicator),                     // int32
		formatInt32(n.Version),                           // int32
		formatInt32(n.Mode),                              // int32
		formatInt32(n.Stratum),                           // int32
		formatInt32(n.Poll),                              // int32
		formatInt32(n.Precision),                         // int32
		strconv.FormatUint(uint64(n.RootDelay), 10),      // uint32
		strconv.FormatUint(uint64(n.RootDispersion), 10), // uint32
		strconv.FormatUint(uint64(n.ReferenceID), 10),    // uint32
		strconv.FormatUint(n.ReferenceTimestamp, 10),     // uint64
		strconv.FormatUint(n.OriginTimestamp, 10),        // uint64
		strconv.FormatUint(n.ReceiveTimestamp, 10),       // uint64
		strconv.FormatUint(n.TransmitTimestamp, 10),      // uint64
		hex.EncodeToString(n.ExtensionBytes),             // []byte
		n.SrcIP,
		n.DstIP,
		formatInt32(n.SrcPort),
		formatInt32(n.DstPort),
	})
}

// Time returns the timestamp associated with the audit record.
func (n *NTP) Time() int64 {
	return n.Timestamp
}

// JSON returns the JSON representation of the audit record.
func (n *NTP) JSON() (string, error) {
	// convert unix timestamp from nano to millisecond precision for elastic
	n.Timestamp /= int64(time.Millisecond)
	n.ReferenceTimestamp /= uint64(time.Millisecond)

	return jsonMarshaler.MarshalToString(n)
}

var ntpMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: strings.ToLower(Type_NC_NTP.String()),
		Help: Type_NC_NTP.String() + " audit records",
	},
	fieldsNTP[1:],
)

// Inc increments the metrics for the audit record.
func (n *NTP) Inc() {
	ntpMetric.WithLabelValues(n.CSVRecord()[1:]...).Inc()
}

// SetPacketContext sets the associated packet context for the audit record.
func (n *NTP) SetPacketContext(ctx *PacketContext) {
	n.SrcIP = ctx.SrcIP
	n.DstIP = ctx.DstIP
	n.SrcPort = ctx.SrcPort
	n.DstPort = ctx.DstPort
}

// Src returns the source address of the audit record.
func (n *NTP) Src() string {
	return n.SrcIP
}

// Dst returns the destination address of the audit record.
func (n *NTP) Dst() string {
	return n.DstIP
}
