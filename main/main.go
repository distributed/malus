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
		panicln("could not resolve addr")
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
		wi := malus.NewWebInterface(":9000", cm)
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
