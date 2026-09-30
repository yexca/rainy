package metasearch

import (
	"math/rand/v2"
	"net/netip"
)

// chinaProviders are the catalogues that receive the optional X-Real-IP header.
var chinaProviders = map[string]bool{"netease": true, "qq": true, "kugou": true, "kuwo": true}

// chinaPrefixes are /16 blocks of large mainland-China consumer ISPs (China Telecom and China
// Unicom). chinaIP picks a random host address inside one of them, the same approach as
// other open-source clients of these catalogues; it identifies no particular user.
var chinaPrefixes = [][2]byte{
	{116, 25},  // China Telecom, Guangdong
	{183, 14},  // China Telecom, Guangdong
	{220, 181}, // China Telecom, Beijing
	{123, 125}, // China Unicom, Beijing
	{112, 64},  // China Unicom, Shanghai
	{61, 152},  // China Telecom, Shanghai
}

// chinaIP returns a random address in chinaPrefixes (never a .0 or .255 host).
func chinaIP() string {
	p := chinaPrefixes[rand.IntN(len(chinaPrefixes))]
	return netip.AddrFrom4([4]byte{p[0], p[1], byte(rand.IntN(256)), byte(1 + rand.IntN(254))}).String()
}
