package imageguard

import "sync"

// Bound peak decoded-pixel memory independently of the HTTP worker count.
var Decode sync.Mutex
