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

package capture

import (
	"fmt"
	"github.com/dreadl0ck/netcap/decoder/core"
	"github.com/dreadl0ck/netcap/decoder/stream"
	"log"
	"net/http"
	"os"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/felixge/fgprof"
	"github.com/mgutz/ansi"

	// _ "net/http/pprof"
	"github.com/dreadl0ck/netcap"
	"github.com/dreadl0ck/netcap/collector"
	"github.com/dreadl0ck/netcap/decoder/config"
	"github.com/dreadl0ck/netcap/decoder/packet"
	"github.com/dreadl0ck/netcap/defaults"
	"github.com/dreadl0ck/netcap/io"
	"github.com/dreadl0ck/netcap/metrics"
	"github.com/dreadl0ck/netcap/reassembly"
	"github.com/dreadl0ck/netcap/resolvers"
	"github.com/dreadl0ck/netcap/utils"
)

// Run parses the subcommand flags and handles the arguments.
func Run() {

	// parse commandline flags
	fs.Usage = printUsage

	err := fs.Parse(os.Args[2:])
	if err != nil {
		log.Fatal(err)
	}

	// crash if there are two consecutive args provided, to avoid running wrong configurations
	// fs.Parse does not protect against this
	// TODO: move to utils and use in other cli tools
	checkArgs()

	if *flagGenerateConfig {
		io.GenerateConfig(fs, "capture")

		return
	}

	// print a markdown overview of all available decoders and fields
	if *flagPrintProtocolOverview {
		packet.MarkdownOverview()

		return
	}

	if *flagListInterfaces {
		utils.ListAllNetworkInterfaces()

		return
	}

	// configure CPU profiling
	if *flagCPUProfile {
		defer func() func() {
			if *flagCPUProfile {
				f, errCPUProfile := os.Create("netcap-" + netcap.Version + ".cpu.profile")
				if errCPUProfile != nil {
					log.Fatalf("could not open cpu profile file %q, error: %s\n", "netcap.cpu.profile", errCPUProfile)
				}

				if errCPUProfile = pprof.StartCPUProfile(f); errCPUProfile != nil {
					log.Fatalf("failed to start CPU profiling, error: %s\n", errCPUProfile)
				}

				return func() {
					pprof.StopCPUProfile()

					errCPUProfile = f.Close()
					if errCPUProfile != nil {
						panic("failed to write CPU profile: " + errCPUProfile.Error())
					}
				}
			}

			return func() {}
		}()

		// fgprof allows to analyze On-CPU as well as Off-CPU (e.g. I/O) time
		http.DefaultServeMux.Handle("/debug/fgprof", fgprof.Handler())

		go func() {
			log.Println(http.ListenAndServe(":6060", nil))
		}()
	}

	// print decoders and exit
	if *flagDecoders {
		packet.ShowDecoders(true)

		return
	}

	// live mode?
	var live bool
	if *flagInterface != "" {
		live = true
	}

	// set data source
	var source string
	switch {
	case *flagInput != "":
		source = *flagInput
	case *flagInterface != "":
		source = *flagInterface
	default:
		source = "unknown"
	}

	if *flagReassemblyDebug {
		reassembly.Debug = true
	}

	var elasticAddrs []string
	if *flagElasticAddrs != "" {
		elasticAddrs = strings.Split(*flagElasticAddrs, ",")
	}

	if *flagGenerateElasticIndices {
		generateElasticIndices(elasticAddrs)

		return
	}

	// abort if there is no input or no live capture
	if *flagInput == "" && !live {
		printHeader()
		fmt.Println(ansi.Red + "> nothing to do. need a pcap file with the read flag (-read) or live mode and an interface (-iface)" + ansi.Reset)
		os.Exit(1)
	}

	if strings.HasSuffix(*flagInput, defaults.FileExtensionCompressed) || strings.HasSuffix(*flagInput, defaults.FileExtension) {
		printHeader()
		fmt.Println(ansi.Red + "> the capture tool is used to create audit records from live traffic or a pcap dumpfile" + ansi.Reset)
		fmt.Println(ansi.Red + "> use the dump tool to read netcap audit records" + ansi.Reset)
		os.Exit(1)
	}

	var exportMetrics bool
	if *flagMetricsAddr != "" {
		metrics.ServeMetricsAt(*flagMetricsAddr, nil)
		// TODO: make the packet metrics configurable separately, for performance analysis it is faster to only use the core metrics
		// exportMetrics = true
	}

	// init collector
	c := collector.New(collector.Config{
		Workers:               *flagWorkers,
		PacketBufferSize:      *flagPacketBuffer,
		WriteUnknownPackets:   !*flagIgnoreUnknown,
		Promisc:               *flagPromiscMode,
		SnapLen:               *flagSnapLen,
		BaseLayer:             utils.GetBaseLayer(*flagBaseLayer),
		DecodeOptions:         utils.GetDecodeOptions(*flagDecodeOptions),
		DPI:                   *flagDPI,
		ReassembleConnections: *flagReassembleConnections,
		FreeOSMem:             *flagFreeOSMemory,
		LogErrors:             *flagLogErrors,
		NoPrompt:              *flagNoPrompt,
		HTTPShutdownEndpoint:  *flagHTTPShutdown,
		Timeout:               *flagTimeout,
		DecoderConfig: &config.Config{
			Quiet:         *flagQuiet,
			PrintProgress: *flagPrintProgress,
			Buffer:        *flagBuffer,
			MemBufferSize: *flagMemBufferSize,
			Compression:   *flagCompress,
			CSV:           *flagCSV,
			Null:          *flagNull,
			Elastic:       *flagElastic,
			ElasticConfig: io.ElasticConfig{
				ElasticAddrs:   elasticAddrs,
				ElasticUser:    *flagElasticUser,
				ElasticPass:    *flagElasticPass,
				KibanaEndpoint: *flagKibanaEndpoint,
			},
			BulkSizeGoPacket:               *flagBulkSizeGoPacket,
			BulkSizeCustom:                 *flagBulkSizeCustom,
			IncludeDecoders:                *flagInclude,
			ExcludeDecoders:                *flagExclude,
			Out:                            *flagOutDir,
			Proto:                          *flagProto,
			JSON:                           *flagJSON,
			Chan:                           false,
			Source:                         source,
			IncludePayloads:                *flagPayload,
			ExportMetrics:                  exportMetrics,
			AddContext:                     *flagContext,
			FlushEvery:                     *flagFlushevery,
			DefragIPv4:                     *flagDefragIPv4,
			Checksum:                       *flagChecksum,
			NoOptCheck:                     *flagNooptcheck,
			IgnoreFSMerr:                   *flagIgnorefsmerr,
			AllowMissingInit:               *flagAllowmissinginit,
			Debug:                          *flagDebug,
			HexDump:                        *flagHexdump,
			WaitForConnections:             *flagWaitForConnections,
			WriteIncomplete:                *flagWriteincomplete,
			MemProfile:                     *flagMemprofile,
			ConnFlushInterval:              *flagConnFlushInterval,
			ConnTimeOut:                    *flagConnTimeOut,
			FlowFlushInterval:              *flagFlowFlushInterval,
			FlowTimeOut:                    *flagFlowTimeOut,
			CloseInactiveTimeOut:           *flagCloseInactiveTimeout,
			ClosePendingTimeOut:            *flagClosePendingTimeout,
			FileStorage:                    *flagFileStorage,
			CalculateEntropy:               *flagCalcEntropy,
			SaveConns:                      *flagSaveConns,
			TCPDebug:                       *flagTCPDebug,
			UseRE2:                         *flagUseRE2,
			BannerSize:                     *flagBannerSize,
			StreamDecoderBufSize:           *flagStreamDecoderBufSize,
			HarvesterBannerSize:            *flagHarvesterBannerSize,
			StopAfterHarvesterMatch:        *flagStopAfterHarvesterMatch,
			StopAfterServiceProbeMatch:     *flagStopAfterServiceProbeMatch,
			StopAfterServiceCategoryMiss:   *flagStopAfterServiceCategoryMiss,
			CustomRegex:                    *flagCustomCredsRegex,
			StreamBufferSize:               *flagStreamBufferSize,
			IgnoreDecoderInitErrors:        *flagIgnoreInitErrs,
			DisableGenericVersionHarvester: *flagDisableGenericVersionHarvester,
			RemoveClosedStreams:            *flagRemoveClosedStreams,
			CompressionBlockSize:           *flagCompressionBlockSize,
			CompressionLevel:               getCompressionLevel(*flagCompressionLevel),
		},
		ResolverConfig: resolvers.Config{
			ReverseDNS:    *flagReverseDNS,
			LocalDNS:      *flagLocalDNS,
			MACDB:         *flagMACDB,
			Ja3DB:         *flagJa3DB,
			ServiceDB:     *flagServiceDB,
			GeolocationDB: *flagGeolocationDB,
		},
	})

	c.PrintConfiguration()

	// collect traffic live from named interface
	if live {
		err = c.CollectLive(*flagInterface, *flagBPF)
		if err != nil {
			log.Fatal("failed to collect live packets: ", err)
		}

		return
	}

	// start timer
	start := time.Now()

	// in case a BPF should be set, the gopacket/pcap version with libpcap bindings needs to be used
	// setting BPF filters is not yet supported by the pcapgo package
	if *flagBPF != "" {
		if err = c.CollectBPF(*flagInput, *flagBPF); err != nil {
			log.Fatal("failed to set BPF: ", err)
		}

		return
	}

	// if not, use native pcapgo version
	isPcap, err := collector.IsPcap(*flagInput)
	if err != nil {
		// invalid path
		fmt.Println("failed to open file:", err)
		os.Exit(1)
	}

	if isPcap {
		if err = c.CollectPcap(*flagInput); err != nil {
			log.Fatal("failed to collect audit records from pcap file: ", err)
		}
	} else {
		if err = c.CollectPcapNG(*flagInput); err != nil {
			log.Fatal("failed to collect audit records from pcapng file: ", err)
		}
	}

	if *flagTime {
		// stat input file
		stat, _ := os.Stat(*flagInput)
		fmt.Println("size", humanize.Bytes(uint64(stat.Size())), "done in", time.Since(start))
	}

	// memory profiling
	if *flagMemProfile {
		f, errProfile := os.Create("netcap-" + netcap.Version + ".mem.profile")
		if errProfile != nil {
			log.Fatal("failed create memory profile: ", errProfile)
		}

		if errProfile = pprof.WriteHeapProfile(f); errProfile != nil {
			log.Fatal("failed to write heap profile: ", errProfile)
		}

		err = f.Close()
		if err != nil {
			panic("failed to write memory profile: " + err.Error())
		}
	}
}

func generateElasticIndices(elasticAddrs []string) {
	packet.ApplyActionToPacketDecoders(func(d packet.DecoderAPI) {
		io.CreateElasticIndex(&io.WriterConfig{
			CSV:     *flagCSV,
			Proto:   *flagProto,
			JSON:    *flagJSON,
			Name:    d.GetName(),
			Type:    d.GetType(),
			Null:    *flagNull,
			Elastic: *flagElastic,
			ElasticConfig: io.ElasticConfig{
				ElasticAddrs:   elasticAddrs,
				ElasticUser:    *flagElasticUser,
				ElasticPass:    *flagElasticPass,
				KibanaEndpoint: *flagKibanaEndpoint,
				BulkSize:       *flagBulkSizeCustom,
			},
			Buffer:        *flagBuffer,
			Compress:      *flagCompress,
			Out:           *flagOutDir,
			Chan:          false,
			ChanSize:      0,
			MemBufferSize: *flagMemBufferSize,
			Version:       netcap.Version,
			StartTime:     time.Now(),
		})
	})

	packet.ApplyActionToGoPacketDecoders(func(d *packet.GoPacketDecoder) {
		io.CreateElasticIndex(&io.WriterConfig{
			CSV:     *flagCSV,
			Proto:   *flagProto,
			JSON:    *flagJSON,
			Name:    d.Layer.String(),
			Type:    d.Type,
			Null:    *flagNull,
			Elastic: *flagElastic,
			ElasticConfig: io.ElasticConfig{
				ElasticAddrs:   elasticAddrs,
				ElasticUser:    *flagElasticUser,
				ElasticPass:    *flagElasticPass,
				KibanaEndpoint: *flagKibanaEndpoint,
				BulkSize:       *flagBulkSizeCustom,
			},
			Buffer:        *flagBuffer,
			Compress:      *flagCompress,
			Out:           *flagOutDir,
			Chan:          false,
			ChanSize:      0,
			MemBufferSize: *flagMemBufferSize,
			Version:       netcap.Version,
			StartTime:     time.Now(),
		})
	})

	stream.ApplyActionToStreamDecoders(func(d core.StreamDecoderAPI) {
		io.CreateElasticIndex(&io.WriterConfig{
			CSV:     *flagCSV,
			Proto:   *flagProto,
			JSON:    *flagJSON,
			Name:    d.GetName(),
			Type:    d.GetType(),
			Null:    *flagNull,
			Elastic: *flagElastic,
			ElasticConfig: io.ElasticConfig{
				ElasticAddrs:   elasticAddrs,
				ElasticUser:    *flagElasticUser,
				ElasticPass:    *flagElasticPass,
				KibanaEndpoint: *flagKibanaEndpoint,
				BulkSize:       *flagBulkSizeCustom,
			},
			Buffer:        *flagBuffer,
			Compress:      *flagCompress,
			Out:           *flagOutDir,
			Chan:          false,
			ChanSize:      0,
			MemBufferSize: *flagMemBufferSize,
			Version:       netcap.Version,
			StartTime:     time.Now(),
		})
	})

	stream.ApplyActionToAbstractDecoders(func(d core.DecoderAPI) {
		io.CreateElasticIndex(&io.WriterConfig{
			CSV:     *flagCSV,
			Proto:   *flagProto,
			JSON:    *flagJSON,
			Name:    d.GetName(),
			Type:    d.GetType(),
			Null:    *flagNull,
			Elastic: *flagElastic,
			ElasticConfig: io.ElasticConfig{
				ElasticAddrs:   elasticAddrs,
				ElasticUser:    *flagElasticUser,
				ElasticPass:    *flagElasticPass,
				KibanaEndpoint: *flagKibanaEndpoint,
				BulkSize:       *flagBulkSizeCustom,
			},
			Buffer:        *flagBuffer,
			Compress:      *flagCompress,
			Out:           *flagOutDir,
			Chan:          false,
			ChanSize:      0,
			MemBufferSize: *flagMemBufferSize,
			Version:       netcap.Version,
			StartTime:     time.Now(),
		})
	})
}

func checkArgs() {
	var expectArg bool
	for i, a := range os.Args[2:] {
		if strings.HasPrefix(a, "-") {
			expectArg = true
			continue
		}
		if expectArg {
			expectArg = false
		} else {
			args := os.Args[2:]
			index := i-1
			if i == 0 {
				index = 0
			}
			log.Fatal("two consecutive args detected, typo? ", ansi.Red, args[index:], ansi.Reset)
		}
	}
}
