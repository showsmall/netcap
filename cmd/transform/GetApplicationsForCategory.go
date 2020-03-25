package main

import (
	maltego "github.com/dreadl0ck/netcap/maltego"
	"github.com/dreadl0ck/netcap/types"
	"strconv"
)

func GetApplicationsForCategory() {
	maltego.IPTransform(
		nil,
		func(lt maltego.LocalTransform, trx *maltego.MaltegoTransform, profile  *types.DeviceProfile, min, max uint64, profilesFile string, mac string, ipaddr string) {
			if profile.MacAddr == mac {

				for _, ip := range profile.Contacts {

					if ip.Addr == ipaddr {

						category := lt.Values["description"]
						for protoName, proto := range ip.Protocols {
							if proto.Category == category {
								ent := trx.AddEntity("maltego.Service", protoName)
								ent.SetType("maltego.Service")
								ent.SetValue(protoName)

								// di := "<h3>Application</h3><p>Timestamp first seen: " + ip.TimestampFirst + "</p>"
								// ent.AddDisplayInformation(di, "Netcap Info")

								ent.SetLinkLabel(strconv.FormatInt(int64(proto.Packets), 10) + " pkts")
								ent.SetLinkColor("#000000")
							}
						}

						break
					}
				}
				for _, ip := range profile.DeviceIPs {

					if ip.Addr == ipaddr {

						category := lt.Values["description"]
						for protoName, proto := range ip.Protocols {
							if proto.Category == category {
								ent := trx.AddEntity("maltego.Service", protoName)
								ent.SetType("maltego.Service")
								ent.SetValue(protoName)

								// di := "<h3>Application</h3><p>Timestamp first seen: " + ip.TimestampFirst + "</p>"
								// ent.AddDisplayInformation(di, "Netcap Info")

								ent.SetLinkLabel(strconv.FormatInt(int64(proto.Packets), 10) + " pkts")
								ent.SetLinkColor("#000000")
							}
						}

						break
					}
				}
			}
		},
	)
}