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

package io

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
	"github.com/klauspost/pgzip"

	"github.com/dreadl0ck/netcap/defaults"
	"github.com/dreadl0ck/netcap/delimited"
	"github.com/dreadl0ck/netcap/types"
)

// JSONWriter is a structure that supports writing JSON audit records to disk.
type JSONWriter struct {
	bWriter *bufio.Writer
	gWriter *pgzip.Writer
	dWriter *delimited.Writer
	jWriter *JSONProtoWriter

	file *os.File
	mu   sync.Mutex
	wc   *WriterConfig
}

// NewJSONWriter initializes and configures a new JSONWriter instance.
func NewJSONWriter(wc *WriterConfig) *JSONWriter {
	w := &JSONWriter{}
	w.wc = wc

	if wc.MemBufferSize <= 0 {
		wc.MemBufferSize = defaults.BufferSize
	}

	// create file
	if wc.Compress {
		w.file = createFile(filepath.Join(wc.Out, w.wc.Name), ".json.gz")
	} else {
		w.file = createFile(filepath.Join(wc.Out, w.wc.Name), ".json")
	}

	if wc.Buffer {
		w.bWriter = bufio.NewWriterSize(w.file, wc.MemBufferSize)

		if wc.Compress {
			var errGzipWriter error
			w.gWriter, errGzipWriter = pgzip.NewWriterLevel(w.bWriter, defaults.CompressionLevel)

			if errGzipWriter != nil {
				panic(errGzipWriter)
			}

			w.jWriter = NewJSONProtoWriter(w.gWriter)
		} else {
			w.jWriter = NewJSONProtoWriter(w.bWriter)
		}
	} else {
		if wc.Compress {
			var errGzipWriter error
			w.gWriter, errGzipWriter = pgzip.NewWriterLevel(w.file, defaults.CompressionLevel)
			if errGzipWriter != nil {
				panic(errGzipWriter)
			}
			w.jWriter = NewJSONProtoWriter(w.gWriter)
		} else {
			w.jWriter = NewJSONProtoWriter(w.file)
		}
	}

	if w.gWriter != nil {
		// To get any performance gains, you should at least be compressing more than 1 megabyte of data at the time.
		// You should at least have a block size of 100k and at least a number of blocks that match the number of cores
		// your would like to utilize, but about twice the number of blocks would be the best.
		if err := w.gWriter.SetConcurrency(defaults.CompressionBlockSize, runtime.GOMAXPROCS(0)*2); err != nil {
			log.Fatal("failed to configure compression package: ", err)
		}
	}

	return w
}

// WriteCSV writes a CSV record.
func (w *JSONWriter) Write(msg proto.Message) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	_, err := w.jWriter.WriteRecord(msg)

	return err
}

// WriteHeader writes a CSV header.
func (w *JSONWriter) WriteHeader(t types.Type) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	_, err := w.jWriter.WriteHeader(NewHeader(t, w.wc.Source, w.wc.Version, w.wc.IncludesPayloads, w.wc.StartTime))

	return err
}

// Close flushes and closes the writer and the associated file handles.
func (w *JSONWriter) Close() (name string, size int64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.wc.Buffer {
		flushWriters(w.bWriter)
	}

	if w.wc.Compress {
		closeGzipWriters(w.gWriter)
	}

	return closeFile(w.wc.Out, w.file, w.wc.Name)
}

// NullWriter is a writer that writes nothing to disk.
type NullWriter struct{}

// NewNullWriter initializes and configures a new NullWriter instance.
func NewNullWriter() *NullWriter {
	return &NullWriter{}
}

// WriteCSV writes a CSV record.
func (w *NullWriter) Write(msg proto.Message) error {
	return nil
}

// WriteHeader writes a CSV header.
func (w *NullWriter) WriteHeader(t types.Type) error {
	return nil
}

// Close flushes and closes the writer and the associated file handles.
func (w *NullWriter) Close() (name string, size int64) {
	return "", 0
}

// JSONProtoWriter implements writing audit records to disk in the JSON format.
type JSONProtoWriter struct {
	w io.Writer
	sync.Mutex
}

// NewJSONProtoWriter returns a new JSON writer instance.
func NewJSONProtoWriter(w io.Writer) *JSONProtoWriter {
	return &JSONProtoWriter{
		w: w,
	}
}

// WriteHeader writes the CSV header to the underlying file.
func (w *JSONProtoWriter) WriteHeader(h *types.Header) (int, error) {
	w.Lock()
	defer w.Unlock()

	marshaled, errMarshal := json.Marshal(h)
	if errMarshal != nil {
		return 0, fmt.Errorf("failed to marshal json: %w", errMarshal)
	}

	n, err := w.w.Write(marshaled)
	if err != nil {
		return n, err
	}

	return w.w.Write([]byte("\n"))
}

// WriteRecord writes a protocol buffer into the JSON writer.
func (w *JSONProtoWriter) WriteRecord(msg proto.Message) (int, error) {
	w.Lock()
	defer w.Unlock()

	if j, ok := msg.(types.AuditRecord); ok {
		js, err := j.JSON()
		if err != nil {
			return 0, err
		}

		return w.w.Write([]byte(js + "\n"))
	}

	spew.Dump(msg)
	panic("can not write as JSON")
}