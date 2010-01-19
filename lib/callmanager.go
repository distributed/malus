package malus


// TODO: taipei torrent makes []byte of "adequate" size when reading a
// string. it does not check if the bencoded string is really that
// long -> hilarious DOS

import (
	"fmt"
	"os"
	"bytes"
	"encoding/hex"
	"jackpal/bencode"
	"reflect"
	"net"
	"time"
	"log"
	"rand"
)

const (
	HASHLEN = 20
	IDLEN   = 8
	TIMEOUT = 1500 * 1000 * 1000
)


type DummyWriter struct{}

func (d *DummyWriter) Write(p []byte) (n int, err os.Error) {
	return len(p), nil
}


type Transceiver interface {
	SendRPC(rpc *RPC) (err os.Error)
	GetReceiveChannel() <-chan *Packet
}


type Header struct {
	Version       uint8
	Sender        string
	Id            uint64
	DHTId         []byte
	Call          uint8
	PayloadLength uint16
	Payload       *Payload
}

func (h *Header) String() string {
	b := bytes.NewBufferString("")

	if h == nil { return "<*Header: nil>" }

	b.WriteString("Header {\n")

	b.WriteString(fmt.Sprintf("  % -8s = %x\n",
		"Sender",
		h.Sender))

	b.WriteString(fmt.Sprintf("  % -8s = 0x%016x\n",
		"Id",
		h.Id))

	b.WriteString(fmt.Sprintf("  % -8s = 0x%02x\n", "Call", h.Call))

	b.WriteString(fmt.Sprintf("  % -8s = 0x%04x\n", "PLength", h.PayloadLength))

	b.WriteString(fmt.Sprintf("  % -8s = %s\n",
		"DHTId",
		hex.EncodeToString(h.DHTId)))

	b.WriteString(fmt.Sprintf("  % -8s = 0x%02x\n", "Version", h.Version))

	b.WriteString("}")

	return b.String()
}

type Payload struct{}


type MalusDecodingError string

func (s MalusDecodingError) String() string { return string(s) }

type RPCError string

func (s RPCError) String() string { return string(s) }


type rpcentry struct {
	fun         *reflect.FuncValue
	funtype     *reflect.FuncType
	strictmatch bool
}


type RPCFrame struct {
	Name    string
	Args    []reflect.Value
	OutArgs []interface{}
}

type RPC struct {
	Header   *Header
	RPCFrame *RPCFrame
	Payload  interface{}
	Packet   *Packet
	From *Host
	To *Host
}


type RunningRPC struct {
	rpc     *RPC
	retchan chan *RPC
}

type Packet struct {
	To   net.Addr
	From net.Addr
	Data []byte
}


type CallManager struct {
	Id string
	rpcmap      map[string](*rpcentry)
	transceiver Transceiver
	logger      *log.Logger
	log         bool
	running     map[uint64]*RunningRPC
	regchan     chan *RunningRPC
	inchan      chan *RPC
	timeout  int64
	rt RoutingTable
}


func NewCallManager(transceiver Transceiver, rt RoutingTable) *CallManager {
	cm := new(CallManager)

	cm.rpcmap = make(map[string](*rpcentry), 8)
	cm.transceiver = transceiver
	cm.logger = log.New(os.Stdout, nil, "CallManager: ", 0)
	cm.log = true
	cm.running = make(map[uint64]*RunningRPC)
	cm.regchan = make(chan *RunningRPC)
	cm.inchan = make(chan *RPC)
	cm.timeout = TIMEOUT
	cm.rt = rt

	return cm
}


func (cm *CallManager) manageRunning() {
	running := cm.running

	timeout := make(chan uint64)

	for {
		select {
		case r := <-cm.regchan:
			running[r.rpc.Header.Id] = r
			// TODO: manage with queue
			go func(id uint64){
				// TODO: check EINTR
				time.Sleep(cm.timeout)
				timeout <- id

			}(r.rpc.Header.Id)
		case rpc := <-cm.inchan:
			Id := rpc.Header.Id
			if runningrpc, ok := running[Id]; ok {
				runningrpc.retchan <- rpc
				running[Id] = nil, false
			}
		case id := <- timeout:
			if runningrpc, ok := running[id]; ok {
				runningrpc.retchan <- nil
				running[id] = nil, false
			}
		}
	}
}

func (cm *CallManager) constructAnswer(req *RPC, retcall uint8, retis []interface{}) (retrpc *RPC) {
	retrpc = new(RPC)

	header := new(Header)
	retrpc.Header = header
	header.Call = retcall
	header.Id = req.Header.Id
	header.DHTId = req.Header.DHTId
	header.Sender = cm.Id
	header.Version = req.Header.Version

	retrpc.Payload = retis

	retrpc.Packet = new(Packet)
	retrpc.Packet.To = req.Packet.From

	return
}

// TODO: errors...
func (cm *CallManager) DispatchRPC(rpc *RPC) {
	if cm.rt != nil {
		cm.rt.SeeHost(rpc.From)
	}
	switch rpc.Header.Call {
	case 0x01:
		retcall, retis, err := cm.DispatchRequest(rpc)
		if err == nil {
			retrpc := cm.constructAnswer(rpc, retcall, retis)
			packet := cm.EncodeRPC(retrpc)
			if packet == nil {
				panic("packet could not be encoded?!")
			}
			retrpc.Packet.Data = packet
			cm.transceiver.SendRPC(retrpc)
		}
	case 0x81, 0x82, 0x83:
		cm.DispatchAnswer(rpc)
	default:
		return
	}

}


func (cm *CallManager) DispatchRequest(rpc *RPC) (retcall uint8, retis []interface{}, err os.Error) {
	err = nil
	retcall = 0x83
	retis = make([]interface{}, 0)

	rpcdesc, ok := cm.rpcmap[rpc.RPCFrame.Name]
	if !ok {
		if cm.log {
			cm.logger.Logf("rpc %q not found!\n", rpc.RPCFrame.Name)
		}
		retcall = 0x82
		return
	}

	args := rpc.RPCFrame.Args
	funtype := rpcdesc.funtype

	if len(args) != funtype.NumIn() {
		err = MalusDecodingError("num args do not match")
		return
	}

	// always strict matching!
	for i := 0; i < len(args); i++ {
		//fmt.Printf("checking type %d ", i)
		ta := reflect.Typeof(args[i].Interface())
		tb := funtype.In(i)
		//fmt.Printf("arg %d ta %v tb %v\n", i, ta, tb)
		if !(ta == tb) {
			if cm.log {
				cm.logger.Logf("type %d does not match!\n", i)
			}
			err = MalusDecodingError("types not matching")
			return
		}
	}

	rets := rpcdesc.fun.Call(args)
	retis = make([]interface{}, len(rets))
	for i, val := range rets {
		retis[i] = val.Interface()
	}

	retcall = 0x81

	return
}


func (cm *CallManager) DispatchAnswer(rpc *RPC) {
	//cm.logger.Logf("%v", rpc)
	cm.inchan <- rpc
}


func (cm *CallManager) AddRPC(name string, fun interface{}) {

	fwrapper, ok := reflect.NewValue(fun).(*reflect.FuncValue)
	if !ok {
		panic("you did not pass a function to *CallManager.AddRPC")
	}

	entry := new(rpcentry)
	entry.fun = fwrapper
	entry.funtype = reflect.Typeof(fun).(*reflect.FuncType)
	entry.strictmatch = true

	cm.rpcmap[name] = entry

}

func (c *CallManager) ParseHeader(raw []byte) (header *Header, payloadpos int, err os.Error) {

	l := len(raw)
	if l < 1 {
		return nil, 0, MalusDecodingError("header too short")
	}

	header = new(Header)

	header.Version = raw[0]

	if header.Version == 1 {
		if l < 32 {
			return nil, 0, MalusDecodingError("invalid header length for type 1 header")
		}

		header.Sender = string(raw[1 : 1+20])
		//copy(header.Id[0:8], raw[21:21+8])
		header.Id = 0
		for i := 0; i < 8; i++ {
			header.Id <<= 8
			header.Id |= uint64(raw[21+i])
		}
		// TODO: nil or {} ?
		header.DHTId = nil

		header.Call = raw[29]

		// payload length is in network byte order
		header.PayloadLength = uint16(raw[30]<<8) | uint16(raw[31])

		return header, 32, nil
	} else {
		return nil, 1, MalusDecodingError("unknown header version")
	}

	return nil, 0, MalusDecodingError("invalid flow")
}


func (c *CallManager) DecodePayload(h *Header, raw []byte, payloadpos int) (payload interface{}, err os.Error) {

	if h.Call != 0x01 && (h.Call < 0x81 || h.Call > 0x83) {
		return nil, MalusDecodingError("unkown call")
	}

	if (h.Call > 0x83 || h.Call < 0x82) && payloadpos == len(raw) {
		return nil, MalusDecodingError("no payload!")
	} else if (h.Call <= 0x83 || h.Call >= 0x82) && payloadpos == len(raw) {
		return make(map[string]interface{}), nil
	}
	payloadbuf := bytes.NewBuffer(raw[payloadpos:])

	data, err := bencode.Decode(payloadbuf)
	if err != nil {
		return nil, MalusDecodingError("bencoding error")
	}

	//fmt.Printf("decode ok data %v\n", data)
	return data, nil
}


func (c *CallManager) ReadRPC(h *Header, payload interface{}) (rpcframe *RPCFrame, err os.Error) {

	rpcframe = nil

	if h.Call == 0x01 {
		switch payload.(type) {
		default:
			err = MalusDecodingError("not a map!\n")
			return
		case map[string]interface{}:

		}
	} else if h.Call == 0x81 {
		if _, ok := payload.([]interface{}); !ok {
			err = MalusDecodingError("retvals not list")
			return
		}
		rpcframe = nil
		err = nil
		return
	}

	rpcdesc := payload.(map[string]interface{})
	irpcname, ok := rpcdesc["name"]
	if !ok {
		err = MalusDecodingError("name not given")
		return
	}

	rpcname, ok := irpcname.(string)
	if !ok {
		err = MalusDecodingError("name not a string")
		return
	}

	arglist, ok := rpcdesc["args"]
	if !ok {
		fmt.Printf("args not given\n")
		return
	}

	switch arglist.(type) {
	default:
		err = MalusDecodingError("arglist not a slice\n")
		return
	case []interface{}:

	}

	argslice := reflect.NewValue(arglist).(*reflect.SliceValue)
	//fmt.Printf("=>\n")
	sl := argslice.Len()
	rpcframe = new(RPCFrame)
	rpcframe.Name = rpcname
	// Args[0] is RPC info
	rpcframe.Args = make([]reflect.Value, sl+1)
	for i := 0; i < sl; i++ {
		elem := argslice.Elem(i)

		value := elem.(*reflect.InterfaceValue).Elem()
		rpcframe.Args[i+1] = value

		//fmt.Printf("  elem %d is type %s\n", i, reflect.Typeof(value).String())

	}
	//fmt.Printf("<=\n")


	err = nil
	return

}


func (cm *CallManager) EncodeRPC(rpc *RPC) []byte {
	epayload := cm.EncodePayload(rpc)
	plen := len(epayload)
	// maximum for payload size in header
	if plen >= (1 << 16) {
		panic("EncodeRPC: payload too long")
	}
	rpc.Header.PayloadLength = uint16(plen)
	//fmt.Printf("plen %d\n", plen)

	eheader := cm.EncodeHeader(rpc)
	if eheader == nil {
		panic("EncodeRPC: could not encode outgoing header!")
	}
	hlen := len(eheader)
	//fmt.Printf("hlen %d\n", hlen)

	packet := make([]byte, plen+hlen)
	copy(packet[0:hlen], eheader)
	copy(packet[hlen:hlen+plen], epayload)

	return packet
}


func (cm *CallManager) EncodeHeader(rpc *RPC) []byte {
	header := rpc.Header
	buf := bytes.NewBuffer(nil)

	buf.WriteByte(header.Version)

	switch header.Version {
	case 1:
		if len(header.Sender) != HASHLEN {
			panic("invalid sender, cannot encode header")
		}
		buf.WriteString(header.Sender)

		t := header.Id
		// MSB first
		for i := 0; i < 8; i++ {
			buf.WriteByte(byte(t >> 56))
			t <<= 8
		}

		buf.WriteByte(header.Call)

		buf.WriteByte(byte(header.PayloadLength >> 8))
		buf.WriteByte(byte(header.PayloadLength & 0xFF))
	default:
		return nil
	}

	return buf.Bytes()

}


// TODO: this is a misnomer...
func (cm *CallManager) EncodePayload(rpc *RPC) []byte {
	buf := bytes.NewBuffer(nil)

	//cm.logger.Logf("encoding payload of call %d\n", rpc.Header.Call)

	bencode.Marshal(buf, rpc.Payload)
	b := buf.Bytes()
	/*if len(b) == 0 {
		b = make([]byte, 2)
		b[0] = 'l'
		b[1] = 'e'
	}*/
	return b
}


func (cm *CallManager) DispatchPacket(packet *Packet) {
	rpc := new(RPC)
	rpc.Packet = packet
	bts := packet.Data

	var t1, t2 int64

	t1 = time.Nanoseconds()
	header, payloadpos, err := cm.ParseHeader(bts)
	if cm.log {
		//cm.logger.Logf("err %v header %v\n", err, header)
		//cm.logger.Logf("header %p\n", header)
	}

	if err != nil {
		panic("panic reason: header could not be parsed")
	}
	rpc.Header = header
	t2 = time.Nanoseconds()
	//fmt.Printf("ParseHeader: %d us\n", (t2-t1)/1000)

	t1 = time.Nanoseconds()
	payload, err := cm.DecodePayload(header, bts, payloadpos)

	if err != nil {
		panic("could not decode payload")
	}
	rpc.Payload = payload
	t2 = time.Nanoseconds()
	//fmt.Printf("DecodePayload: %d us\n", (t2-t1)/1000)

	t1 = time.Nanoseconds()
	rpcframe, err := cm.ReadRPC(header, payload)
	if err != nil {
		fmt.Printf("%v\n", err)
		panic("could not read RPC")
	}
	rpc.RPCFrame = rpcframe
	t2 = time.Nanoseconds()
	//fmt.Printf("ReadRPC: %d us\n", (t2-t1)/1000)

	if rpc.Header.Call == 0x01 {
		rpc.RPCFrame.Args[0] = reflect.NewValue(rpc)
	}

	t1 = time.Nanoseconds()


	from := NewHost(packet.From, rpc.Header.Sender)
	rpc.From = from
	fmt.Printf("FROM: %v\n", from)

	cm.DispatchRPC(rpc)
	t2 = time.Nanoseconds()
	//fmt.Printf("DispatchRPC: %d us\n", (t2-t1)/1000)
	//fmt.Printf("payload %v\n", rpc.Payload)
	_ = t2 - t1
}


func (cm *CallManager) Call(addr net.Addr, name string, args []interface{}) (retis []interface{}, err os.Error) {

	cm.logger.Logf("call called\n")
	fmt.Printf("CALL\n")

	rpc := new(RPC)
	header := new(Header)
	rpc.Header = header

	header.Version = 1
	header.Sender = cm.Id
	// TODO: DHTId!
	header.DHTId = nil
	header.Id = uint64(rand.Uint32())<<32 | uint64(rand.Uint32())
	header.Call = 0x01

	packet := new(Packet)
	rpc.Packet = packet
	packet.To = addr

	rpcframe := new(RPCFrame)
	rpc.RPCFrame = rpcframe
	rpcframe.Name = name
	rpcframe.OutArgs = args

	rpcdesc := make(map[string]interface{})
	rpcdesc["name"] = name
	rpcdesc["args"] = args

	rpc.Payload = rpcdesc

	cm.logger.Logf("Call: encoding packet")
	data := cm.EncodeRPC(rpc)
	if packet == nil {
		return nil, RPCError("could not encode packet")
	}
	packet.Data = data


	// register RPC...
	running := &RunningRPC{rpc, make(chan *RPC)}
	cm.regchan <- running

	err = cm.transceiver.SendRPC(rpc)
	if err != nil {
		return nil, err
	}

	retrpc := <-running.retchan
	if retrpc == nil {
		return nil, RPCError("time out")
	}
	
	if retis, ok := retrpc.Payload.([]interface{}); ok {
		return retis, nil
	}


	if cm.log {
		cm.logger.Logf("return payload was not []interface{}\n")
	}
	return nil, nil


}

func (cm *CallManager) Run() {

	recv := cm.transceiver.GetReceiveChannel()

	go cm.manageRunning()

	for {
		select {
		case packet := <-recv:
			if cm.log {
				cm.logger.Logf("call manager recvd\n")
			}
			go cm.DispatchPacket(packet)
			// help GC
			packet = nil
		}
	}

}

/*

*/


func Ping(rpc *RPC) int64 { return 0x42 }

func Store(rpc *RPC, hash string, data string) int64 {
	return 1
}
