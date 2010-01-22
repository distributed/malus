package malus


import (
	"http"
	"strings"
	"fmt"
	"os"
	"net"
	"expvar"
	"malloc"
)


type WebInterface struct {
	addr       string
	cm         *CallManager
	sm         *http.ServeMux
	rt         RoutingTable
	reqcounter *expvar.Int
}


func NewWebInterface(addr string, cm *CallManager, rt RoutingTable) *WebInterface {
	wi := new(WebInterface)
	wi.addr = addr
	wi.cm = cm
	wi.rt = rt
	wi.sm = http.NewServeMux()
	wi.reqcounter = expvar.NewInt("")

	wi.sm.Handle("/", http.HandlerFunc(wi.getDummy()))

	return wi
}


func (wi *WebInterface) Run() (err os.Error) {
	err = http.ListenAndServe(wi.addr, wi.sm)
	return
}


// this function wraps handlers defined as methods of a WebInterface
// struct and binds them to a provided *WebInterface
func (wi *WebInterface) wrapHandler(f func(*WebInterface, *http.Conn, *http.Request)) (func(*http.Conn, *http.Request)) {
	fmt.Printf(">> wrapping handler wi %v f %v\n", wi, f)
	return func(c *http.Conn, r *http.Request) {
		fmt.Printf("outer handler called with wi %v c %v\n", wi, c)
		f(wi, c, r)
	}
}


func (wi *WebInterface) getDummy() (func(*http.Conn, *http.Request)) {
	dummy := func(c *http.Conn, req *http.Request) {
		fmt.Printf("incoming request!\n")
		wi.reqcounter.Add(1)
		raddr, _ := net.ResolveUDPAddr("127.0.0.1:8001")
		fmt.Printf("WI calling raddr %v\n", raddr)
		fmt.Fprintf(c, "<tt>\n")
		switch req.FormValue("rpc") {
		case "ping":
			c.Write(strings.Bytes("pinging... <br>"))
			retis, err := wi.cm.Call(raddr, "ping", make([]interface{}, 0))
			fmt.Fprintf(c, "=> ping done! err %v retis %v\n", err, retis)
		case "getsocket":
			retis, err := wi.cm.Call(raddr, "getsocket", make([]interface{}, 0))
			fmt.Fprintf(c, "=> getsocket err %v retis %v<br>\n", err, retis)
		case "resolve":
			saddr := req.FormValue("addr")
			fmt.Printf("resolving addr %q\n", saddr)
			addr, err := net.ResolveUDPAddr(saddr)
			if err == nil {
				fmt.Fprintf(c, "=> addr %v err %v\n", addr, err)
			} else {
				fmt.Fprintf(c, "failed to resolve addr! err %v\n", err)
			}
		case "rt":
			fmt.Fprintf(c, "%s\n", wi.rt.GetHTML())
		case "closest":
			target := SHA1String("8006")
			cl := wi.rt.GetClosest(target, 20).Data()
			for _, el := range cl {
				fmt.Fprintf(c, "%x | %v @ %v<br>\n", el.Host.Id, el.Distance, el.Host.Addr)
			}
		case "seedrt":
			seedmax := 1000
			for i := 0; i < seedmax; i++ {
				h := new(Host)
				ps := fmt.Sprintf("%d", i+5000)
				h.Addr, _ = net.ResolveUDPAddr("127.0.0.1:" + ps)
				h.Id = SHA1String(ps)
				wi.rt.SeeHost(h)
			}
			fmt.Fprintf(c, "seed rt with %d hosts<br>\n", seedmax)
		case "gc":
			malloc.GC()
			stats := malloc.GetStats()
			fmt.Fprintf(c, "stats: %v<br>\n", stats)
			fmt.Fprintf(c, "=&gt; %d kbyte alloc / %d kbyte sys<br>\n", stats.Alloc / 1024, stats.Sys / 1024)
		default:
			c.Write(strings.Bytes("das esch de rap shit: " + req.FormValue("rpc") + "<br> <a href=\"?rpc=ping\">ping now!</a><br>"))
			fmt.Fprintf(c, "fuck\n")
		}
		fmt.Fprintf(c, "<br><br>req counter: %s\n", wi.reqcounter.String())
		fmt.Fprintf(c, "</tt>")
	}

	return dummy
}
