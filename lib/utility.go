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
