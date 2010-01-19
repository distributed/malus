package malus

import (
	"crypto/sha1"
	"strings"
	"encoding/hex"
)


type Distance []byte;


func (d Distance) String() string {
	return hex.EncodeToString(d)
}
const K = 20
func SHA1Bytes(b []byte) string {
	h := sha1.New()
	h.Write(b)
	return string(h.Sum())
}

func SHA1String(s string) string {
	h := sha1.New()
	h.Write(strings.Bytes(s))
	return string(h.Sum())
}


func XOR(a, b string) Distance {
	l := len(a)
	if l != len(b) {
		return nil
	}
	
	d := make(Distance, l)

	for i := 0; i < l; i ++ {
		d[i] = a[i] ^ b[i]
	}

	return d
}


// Returns into which bucket the given Distance d falls. BucketNo
// counts upwards from MSB to LSB, from d[0] to d[len(d)-1], e.g. if
// the MSB of d[0] is set it returns 0, if the MSB of d[1] is set, it
// returns 8. If the distance is zero, it returns len(d) * 8.
func BucketNo(d Distance) uint {
	var basebitnr uint = 0

	for _, b := range d {
		if b == 0 {
			basebitnr += 8
			continue
		}
		var bitnr uint = 0
		for i := 0; i < 8; i ++ {
			if (b & (0x80 >> bitnr)) != 0 {
				return basebitnr + bitnr
			}
			bitnr++
		}
	}
	
	return basebitnr
}
