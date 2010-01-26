package malus


import (
	"fmt"
	"net"
	)

type robotReturn struct {
	rthost *RTHost
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

	for _, wci := range wireclosest {
		wci, ok := wci.([]interface{})
		if !ok {
			continue
		}

		if len(wci) != 3 {
			continue
		}
		
		host, ok := wci[0].(string)
		if !ok {
			continue
		}

		port64, ok := wci[0].(int64)
		if !ok {
			continue
		}
		var port int = int(uint16(port64))

		id, ok := wci[0].(string)
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
		rth.Host.Addr = addr
		rth.Host.Id = id
		rth.Distance = XOR(oid, id)
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
func find(t string, cm *CallManager, rt RoutingTable, bootstrap *RTHostList) {

	var kclosest *RTHostList
	if bootstrap == nil {
		kclosest = rt.GetClosest(t, K)
	} else {
		kclosest = bootstrap
	}

	if kclosest.Len() == 0 {
		panicln("nobody to ask...")
	}

	kclosest.Sort()
	visited := make(map[string]*RTHost)

	closestd := kclosest.At(0).Distance //MaxDistance
	alpha := Alpha

	retchan := make(chan *robotReturn)
	nrunning := 0
	nqueried := 0
	converging := true

	for converging {
		fmt.Printf("find: convering round\n")
		for (nrunning < alpha) && (kclosest.Len() > 0) {
			fmt.Printf("=> robot w/ %d left\n", kclosest.Len())
			rh := kclosest.PopFront()
			straddr := rh.Host.Addr.String()
			if _, ok := visited[straddr]; ok {
				// host already visited
				continue
			}
			go robot(cm.Id, t, rh, retchan, cm)
			visited[straddr] = rh
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
			if _, ok := visited[addrstr]; !ok {
				kclosest.Push(el)
			}
		}
		//fmt.Printf("find: sort\n")
		kclosest.Sort()
		l := kclosest.Len()
		if l >= K {
			kclosest = kclosest.Slice(0, K)
		}

		if l == 0 {
			break
		}

		el0 := kclosest.At(0)
		if el0.Distance.Less(closestd) {
			closestd = el0.Distance
		} else {
			converging = false
		}
	}

	// TODO: drain channel

	fmt.Printf("find: not converging any more\n")

	// the search is not converging any more. now make sure all k
	// nodes are queried
	/*for (nqueried < K) || (nrunning > 0) {
		// if we can spawn more goroutines => spawn them
		
		// read results similar to code above
	}*/

	// TODO: as above, either drain channel or make channel
	// buffered, so robots actually terminate and channel gets
	// garbitsch collected
}
