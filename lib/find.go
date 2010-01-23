package malus



type robotReturn struct {
	rthost *RTHost
	closest *RTHostList
	retvals []interface{}
}


func robot(rh *RTHost, retchan chan *robotReturn) {
	ret := new(robotReturn)

	ret.rthost = rh
	ret.closest = nil
	ret.retvals = nil

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

	closestd := kclosest.At(0).Distance //MaxDistance
	alpha := Alpha

	retchan := make(chan *robotReturn) // dummy
	nrunning := 0
	nqueried := 0
	converging := true

	for converging {
		for (nrunning < alpha) && (kclosest.Len() > 0) {
			rh := kclosest.PopFront()
			go robot(rh, retchan)
			nrunning++
		}

		ret := <-retchan
		nrunning--
		nqueried++

		if ret.closest == nil {
			panicln("handle this more intelligently...")
		}
		kclosest.Append(ret.closest)
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


	// the search is not converging any more. now make sure all k
	// nodes are queried
	for (nqueried < K) || (nrunning > 0) {
		// if we can spawn more goroutines => spawn them
		
		// read results similar to code above
	}
}
