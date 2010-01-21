package malus


import (
	"os"
	//"fmt"
	"net"
)


const (
	MAXPACKLEN = 2048
)

type UDPError string

func (u UDPError) String() string { return string(u) }


type UDPTransceiver struct {
	spool    chan *Packet
	incoming chan *RPC
	conn     *net.UDPConn
}

func NewUDPTransceiver(snet string, laddr *net.UDPAddr) (t *UDPTransceiver) {
	t = new(UDPTransceiver)

	conn, err := net.ListenUDP(snet, laddr)
	if err != nil {
		return nil
	}
	t.conn = conn
	t.spool = make(chan *Packet)
	t.incoming = make(chan *RPC)

	return t
}

func (t *UDPTransceiver) SendRPC(rpc *RPC) (err os.Error) {
	t.incoming <- rpc
	return
}


func (t *UDPTransceiver) GetReceiveChannel() (c <-chan *Packet) {
	return t.spool
}


func (t *UDPTransceiver) sendLoop() {
	for {
		select {
		case rpc := <-t.incoming:
			t.conn.WriteTo(rpc.Packet.Data, rpc.Packet.To)
		}
	}
}


func (t *UDPTransceiver) receiveLoop() {
	for {
		buf := make([]byte, MAXPACKLEN)
		n, addr, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		buf = buf[0:n]
		pack := new(Packet)
		pack.Data = buf
		pack.From = addr

		//fmt.Printf("received pack from %v\n", pack.From)

		t.spool <- pack
	}
}


func (t *UDPTransceiver) Run() {
	go t.sendLoop()
	go t.receiveLoop()
}
