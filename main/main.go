package main


import (
	"malus"
	"fmt"
	"net"
	//"os"
	//"io/ioutil"
	//"reflect"
	"strconv"
	//	"http"
	//	"strings"
)

func main() {

	laddr, err := net.ResolveUDPAddr("0.0.0.0:7000")
	if err != nil {
		panic("could not resolve addr\n")
	}
	tr := malus.NewUDPTransceiver("udp", laddr)
	if tr == nil {
		panic("could not make transceiver")
	}

	id := malus.SHA1String(strconv.Itoa(laddr.Port))

	rt := malus.NewBRoutingTable(id)

	cm := malus.NewCallManager(tr, rt)
	cm.Id = id
	cm.AddRPC("ping", malus.Ping)
	cm.AddRPC("store", malus.Store)

	findnode := func(rpc *malus.RPC, id string) (hostlist []interface{}) {
		closest := rt.GetClosest(id, malus.K).Data()

		hostlist = make([]interface{}, len(closest))

		for i, ch := range closest {
			wirehost := make([]interface{}, 3)
			// TODO: handle non-UDP case!
			wirehost[0] = ch.Host.Addr.(*net.UDPAddr).IP.String()
			wirehost[1] = ch.Host.Addr.(*net.UDPAddr).Port
			wirehost[2] = ch.Host.Id

			hostlist[i] = wirehost
		}
		
		return
	}
	cm.AddRPC("findnode", findnode)

	fmt.Printf("registered\n")

	/*print(bts)
			fmt.Printf("\n%v\n", bts)*/

	go tr.Run()
	go cm.Run()

	raddr, _ := net.ResolveUDPAddr("127.0.0.1:8001")
	fmt.Printf("calling..\n")
	retis, err := cm.Call(raddr, "ping", make([]interface{}, 0))

	fmt.Printf("=> ping done! <err %v> <retis %v>\n", err, retis)

	{
		fmt.Printf("creating WI\n")
		wi := malus.NewWebInterface(":9000", cm, rt)
		fmt.Printf("now running WI\n")
		err := wi.Run()
		if err != nil {
			fmt.Printf("WI err %v\n", wi)
			panic("WI panic")
		}
		fmt.Printf("WebInterface running\n")
	}

	/*	{
		http.Handle("/", http.HandlerFunc(dummyhandle))
		err := http.ListenAndServe(":9000", nil)
		if err != nil {
			panic("could not ListenAndServe")
		}
	}*/

	<-make(chan bool)

}
