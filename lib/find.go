package malus


import (
	"fmt"
	"net"
	"time"
)

type robotReturn struct {
	rthost  *RTHost
	closest *RTHostList
	retvals []interface{}
}


func robotParse(oid string, t string, retis []interface{}) (closest *RTHostList, retvals []interface{}) {

	closest = nil
	retvals = nil

	if len(retis) != 1 {
		fmt.Printf("find: len(retis) != 1 ?\n")
		return
	}


	// TODO: disambiguate between direct findnode return values
	// and findvalue style mapped retval/closest return values
	wireclosest, ok := retis[0].([]interface{})
	if !ok {
		fmt.Printf("wire closest is not []interface{}")
	}

	closest = NewRTHostList()
	fmt.Printf("robotparse: got %d wireclosest\n", len(wireclosest))
	for _, wci := range wireclosest {
		wci, ok := wci.([]interface{})
		if !ok {
			fmt.Printf("robotparse: entry not interface slice\n")
			continue
		}

		if len(wci) != 3 {
			fmt.Printf("robotparse: len(wci) != 3\n")
			continue
		}

		host, ok := wci[0].(string)
		if !ok {
			fmt.Printf("robotparse: wci[0] not string\n")
			continue
		}

		port64, ok := wci[1].(int64)
		if !ok {
			continue
		}
		var port int = int(uint16(port64))

		id, ok := wci[2].(string)
		if !ok {
			continue
		}
		if len(id) != HASHLEN {
			continue
		}

		addrstring := fmt.Sprintf("%s:%d", host, port)
		addr, err := net.ResolveUDPAddr(addrstring)
		if err != nil {
			continue
		}

		rth := new(RTHost)
		rth.Host = new(Host)
		rth.Host.Addr = addr
		rth.Host.Id = id
		//continue // just debugging...
		rth.Distance = XOR(t, id) // why oid instead of t?
		fmt.Printf("dist: %x xor %x -> %v (%v)\n", oid, id, rth.Distance, rth.Host.Addr)
		closest.Push(rth)
	}

	return
}


func robot(oid string, t string, rh *RTHost, retchan chan *robotReturn, cm *CallManager) {
	ret := new(robotReturn)

	args := []interface{}{t}
	retis, err := cm.Call(rh.Host.Addr, "findnode", args)

	ret.rthost = rh
	if err != nil {
		ret.closest = nil
		ret.retvals = nil
	} else {
		fmt.Printf("robot parsing\n")
		ret.closest, ret.retvals = robotParse(oid, t, retis)
		fmt.Printf("robot parsing done\n")
	}

	fmt.Printf("robot done <=\n")

	retchan <- ret
}

// bootstrap is used destructively. you have been warned.
func find(t string, cm *CallManager, rt RoutingTable, bootstrap *RTHostList) *RTHostList {

	var kclosest *RTHostList
	if bootstrap == nil {
		bootstrap = rt.GetClosest(t, K)
	} else {

	}
	kclosest = NewRTHostList()
	known := make(map[string]*RTHost)
	//kclosest = bootstrap
	for i := 0; i < bootstrap.Len(); i++ {
		el := bootstrap.At(i)
		el.Distance = XOR(el.Host.Id, t) // cm.Id, t?
		kclosest.Push(el)

		straddr := el.Host.Addr.String()
		known[straddr] = el
	}
	
	if kclosest.Len() == 0 {
		panicln("nobody to ask...")
	}

	kclosest.Sort()
	visited := make(map[string]*RTHost)

	closestd := kclosest.At(0).Distance //MaxDistance
	alpha := Alpha
	k := K

	retchan := make(chan *robotReturn)
	nrunning := 0
	nqueried := 0
	converging := true
	finishing := false

	for converging || finishing {
		switch {
		case converging && finishing:
			panicln("find logic error")
		case converging:
			fmt.Printf("find: convering round\n")
		case finishing:
			fmt.Printf("find: finishing round\n")
		}

		for (converging && (nrunning < alpha) && (kclosest.Len() > 0)) || (finishing && ((nqueried + nrunning) < k && kclosest.Len() > 0)) {
			fmt.Printf("=> robot w/ %d left\n", kclosest.Len())
			rh := kclosest.PopFront()
			straddr := rh.Host.Addr.String()
			if _, ok := visited[straddr]; ok {
				// host already visited
				continue
			}
			go robot(cm.Id, t, rh, retchan, cm)
			visited[straddr] = rh
			known[straddr] = rh
			nrunning++
		}

		if nrunning == 0 {
			break
		}

		ret := <-retchan
		nrunning--
		nqueried++

		if ret.closest == nil {
			//panicln("handle this more intelligently...")
			continue
		}
		//fmt.Printf("find: append\n")
		//kclosest.Append(ret.closest)
		for i := 0; i < ret.closest.Len(); i++ {
			el := ret.closest.At(i)
			addrstr := el.Host.Addr.String()
			if _, ok := known[addrstr]; !ok {
				if _, ok := visited[addrstr]; !ok {
					kclosest.Push(el)
					known[addrstr] = el
				}
			}
		}
		//fmt.Printf("find: sort\n")
		kclosest.Sort()
		l := kclosest.Len()
		if l >= K {
			kclosest = kclosest.Slice(0, k)
		}

		if l == 0 {
			break
		}

		if converging {
			el0 := kclosest.At(0)
			if el0.Distance.Less(closestd) {
				closestd = el0.Distance
			} else {
				converging = false
				finishing = true
			}
		}
	}

	// for output ordering... hack-a-thon
	time.Sleep(100*1000*1000)


	finalclosest := NewRTHostList()
	for key, v := range visited {
		fmt.Printf("pushing for key %q\n", key)
		finalclosest.Push(v)
	}
	for i := 0; i < kclosest.Len(); i++ {
		finalclosest.Push(kclosest.At(i))
	}
	finalclosest.Sort()
	if finalclosest.Len() > k {
		finalclosest = finalclosest.Slice(0, k)
	}

	fmt.Printf("find res %d elements\n", finalclosest.Len())
	for i := 0; i < finalclosest.Len(); i++ {
		el := finalclosest.At(i)
		fmt.Printf("Host %02d: d %s @ %v\n", i, el.Distance, el.Host.Addr)
	}

	return  finalclosest

}
