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

var fieldsICMPv6RouterSolicitation = []string{
	"Timestamp",
	"Options",
	"SrcIP",
	"DstIP",
}

// CSVHeader returns the CSV header for the audit record.
func (i *ICMPv6RouterSolicitation) CSVHeader() []string {
	return filter(fieldsICMPv6RouterSolicitation)
}

// CSVRecord returns the CSV record for the audit record.
func (i *ICMPv6RouterSolicitation) CSVRecord() []string {
	var opts []string
	for _, o := range i.Options {
		opts = append(opts, o.toString())
	}

	return filter([]string{
		formatTimestamp(i.Timestamp),
		strings.Join(opts, ""),
		i.SrcIP,
		i.DstIP,
	})
}

// Time returns the timestamp associated with the audit record.
func (i *ICMPv6RouterSolicitation) Time() int64 {
	return i.Timestamp
}

func (o ICMPv6Option) toString() string {
	var b strings.Builder
	b.WriteString(StructureBegin)
	b.WriteString(formatInt32(o.Type))
	b.WriteString(FieldSeparator)
	b.WriteString(hex.EncodeToString(o.Data))
	b.WriteString(StructureEnd)

	return b.String()
}

// JSON returns the JSON representation of the audit record.
func (i *ICMPv6RouterSolicitation) JSON() (string, error) {
	// convert unix timestamp from nano to millisecond precision for elastic
	i.Timestamp /= int64(time.Millisecond)

	return jsonMarshaler.MarshalToString(i)
}

var icmp6rsMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: strings.ToLower(Type_NC_ICMPv6RouterSolicitation.String()),
		Help: Type_NC_ICMPv6RouterSolicitation.String() + " audit records",
	},
	fieldsICMPv6RouterSolicitation[1:],
)

// Inc increments the metrics for the audit record.
func (i *ICMPv6RouterSolicitation) Inc() {
	icmp6rsMetric.WithLabelValues(i.CSVRecord()[1:]...).Inc()
}

// SetPacketContext sets the associated packet context for the audit record.
func (i *ICMPv6RouterSolicitation) SetPacketContext(ctx *PacketContext) {
	i.SrcIP = ctx.SrcIP
	i.DstIP = ctx.DstIP
}

// Src returns the source address of the audit record.
func (i *ICMPv6RouterSolicitation) Src() string {
	return i.SrcIP
}

// Dst returns the destination address of the audit record.
func (i *ICMPv6RouterSolicitation) Dst() string {
	return i.DstIP
}
