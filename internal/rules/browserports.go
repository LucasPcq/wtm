package rules

// browserBlockedPorts are the ports Chrome, Firefox and Safari refuse to open,
// answering ERR_UNSAFE_PORT rather than making the request. The list is the
// union of Chromium's `kRestrictedPorts` and Gecko's `gBadPortList`, both of
// which have been stable for years; it exists to keep well-known non-HTTP
// protocols from being driven by a web page.
//
// It matters here because the run proxy has exactly one purpose — serving job
// names to a browser — so a port a browser will not open makes it useless while
// looking perfectly healthy from the outside: it binds, it routes, and nothing
// ever reaches it.
var browserBlockedPorts = map[int]bool{
	1: true, 7: true, 9: true, 11: true, 13: true, 15: true, 17: true, 19: true,
	20: true, 21: true, 22: true, 23: true, 25: true, 37: true, 42: true, 43: true,
	53: true, 69: true, 77: true, 79: true, 87: true, 95: true, 101: true, 102: true,
	103: true, 104: true, 109: true, 110: true, 111: true, 113: true, 115: true,
	117: true, 119: true, 123: true, 135: true, 137: true, 139: true, 143: true,
	161: true, 179: true, 389: true, 427: true, 465: true, 512: true, 513: true,
	514: true, 515: true, 526: true, 530: true, 531: true, 532: true, 540: true,
	548: true, 554: true, 556: true, 563: true, 587: true, 601: true, 636: true,
	989: true, 990: true, 993: true, 995: true, 1719: true, 1720: true, 1723: true,
	2049: true, 3659: true, 4045: true, 5060: true, 5061: true, 6000: true,
	6566: true, 6665: true, 6666: true, 6667: true, 6668: true, 6669: true,
	6679: true, 6697: true, 10080: true,
}

// IsBrowserBlockedPort reports a port no browser will open. A proxy asked to
// serve one is asked to serve nothing.
func IsBrowserBlockedPort(port int) bool {
	return browserBlockedPorts[port]
}
