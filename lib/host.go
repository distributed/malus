package malus

import "net"
import "fmt"

const uniquedelimiter = "\x00"

type Host struct {
	Addr   net.Addr
	Id     string
	Unique string
}


func NewHost(addr net.Addr, id string) *Host {
	h := new(Host)
	h.Addr = addr
	h.Id = id
	h.Unique = addr.String() + uniquedelimiter +
		id + uniquedelimiter

	return h
}


func (h *Host) String() string {
	uniquestr := ""
	if h.Unique != "" {
		uniquestr = "*"
	}
	return fmt.Sprintf("<Host <addr %v> <id %x> <unique %v>>", h.Addr,
		h.Id,
		uniquestr)
}
