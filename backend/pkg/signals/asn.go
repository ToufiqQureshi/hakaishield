package signals

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"sort"
	"strconv"
	"strings"
)

// hostingASNs are autonomous systems that rent out compute: public
// clouds and VPS hosts. A browser session from one of them is usually a
// script farm, because people browse from home and mobile networks.
//
// Kept deliberately narrow. Consumer VPN exits (M247 and similar) and
// Cloudflare (WARP) are excluded because real people sit behind them.
// Corporate egress through Azure or Google still lands here, which is one
// reason this is evidence only.
var hostingASNs = map[uint32]bool{
	16509: true, 14618: true, // Amazon AWS
	15169: true, 396982: true, // Google, Google Cloud
	8075:   true, // Microsoft Azure
	14061:  true, // DigitalOcean
	24940:  true, // Hetzner
	16276:  true, // OVH
	63949:  true, // Akamai Connected Cloud (Linode)
	20473:  true, // Vultr (Choopa)
	31898:  true, // Oracle Cloud
	45102:  true, // Alibaba Cloud
	37963:  true, // Alibaba (Aliyun, China)
	132203: true, // Tencent Cloud
	51167:  true, // Contabo
	12876:  true, // Scaleway
	60781:  true, // LeaseWeb
}

type ipRange struct{ start, end netip.Addr }

// hostingRanges is sorted by start and non-overlapping. It is written once
// by LoadHostingASNDB during setup and only read afterwards.
var hostingRanges []ipRange

// maxASNLine bounds one TSV line; real lines are under 200 bytes.
const maxASNLine = 4096

// LoadHostingASNDB reads an ip2asn TSV (range_start, range_end, ASN,
// country, description per line, as published by iptoasn.com) and keeps
// only ranges owned by hosting ASNs. Call it once during setup, before
// serving: the table is read without locking on the request path.
//
// Lookups stay in memory, so no request ever waits on a network call.
func LoadHostingASNDB(r io.Reader) (int, error) {
	var ranges []ipRange
	var parsed int
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 512), maxASNLine)
	for sc.Scan() {
		fields := strings.SplitN(sc.Text(), "\t", 4)
		if len(fields) < 3 {
			continue
		}
		asn, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			continue
		}
		start, err1 := netip.ParseAddr(fields[0])
		end, err2 := netip.ParseAddr(fields[1])
		if err1 != nil || err2 != nil || start.BitLen() != end.BitLen() || end.Less(start) {
			continue
		}
		parsed++
		if hostingASNs[uint32(asn)] {
			ranges = append(ranges, ipRange{start.Unmap(), end.Unmap()})
		}
	}
	if err := sc.Err(); err != nil {
		return 0, fmt.Errorf("signals: reading ASN database: %w", err)
	}
	if parsed == 0 {
		return 0, errors.New("signals: ASN database has no valid ip2asn rows")
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start.Less(ranges[j].start) })
	hostingRanges = ranges
	return len(ranges), nil
}

// isHostingIP reports whether ip belongs to a hosting ASN. It is false
// when no database was loaded.
func isHostingIP(ip string) bool {
	if len(hostingRanges) == 0 {
		return false
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	// First range starting after addr; the candidate is the one before it.
	i := sort.Search(len(hostingRanges), func(i int) bool { return addr.Less(hostingRanges[i].start) })
	if i == 0 {
		return false
	}
	r := hostingRanges[i-1]
	return r.start.BitLen() == addr.BitLen() && !r.end.Less(addr)
}
