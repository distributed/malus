package malus


import (
	"os"
	"log"
	"bytes"
	"fmt"
)

type RoutingTable interface {
	SeeHost(h *Host)
	// may return ""
	GetString() string
	// may return "", subject to change
	GetHTML() string
}


type BRoutingTable struct {
	id        string
	buckets   [](*Bucket)
	maxbucket int

	seehost    chan *Host
	quitchan   chan bool
	stringchan chan chan string
	htmlchan   chan chan string

	Log    bool
	logger *log.Logger
}


type RTHost struct {
	host     *Host
	distance Distance
}

type Bucket struct {
	hosts [](*RTHost)
}


func NewBRoutingTable(id string) (rt *BRoutingTable) {
	rt = new(BRoutingTable)

	rt.id = id

	rt.buckets = make([](*Bucket), HASHLEN*8)
	firstbucket := new(Bucket)
	firstbucket.hosts = make([](*RTHost), K+1)[0:0]

	rt.buckets[0] = firstbucket
	rt.maxbucket = 0

	rt.seehost = make(chan *Host)
	rt.quitchan = make(chan bool)

	rt.Log = true
	rt.logger = log.New(os.Stdout, nil, "BRoutingTable: ", 0)

	go rt.main()

	return
}


func (rt *BRoutingTable) main() {
	for {
		select {
		case h := <-rt.seehost:
			rt.seeHost(h)
		case <-rt.quitchan:
			return
		case r := <-rt.stringchan:
			r <- rt.string()
		case r := <-rt.htmlchan:
			r <- rt.html()
		}
	}
}


func (rt *BRoutingTable) SeeHost(h *Host) { rt.seehost <- h }


func (rt *BRoutingTable) seeHost(h *Host) {
	if rt.Log {
		rt.logger.Logf("see host: %v dist %v\n", h, XOR(h.Id, rt.id)[0:5])
	}
	bucketno, pos, maxbucketno := rt.findHost(h)
	/*rt.logger.Logf("is in %d/%d maxbucketno %d\n", bucketno, pos, maxbucketno)
	rt.logger.Logf("dist %v to %v\n", XOR(h.Id, rt.id), h)
	rt.logger.Logf("own id is %x\n", rt.id)
	rt.logger.Logf("h.Id is %x", h.Id)*/


	bucket := rt.buckets[bucketno]

	// we do not already have the entry
	if pos < 0 {
		if rt.Log {
			rt.logger.Logf("we don't have an entry yet\n")
		}
		rthost := &RTHost{h, XOR(h.Id, rt.id)}
		hl := len(bucket.hosts)
		if hl < K {
			if rt.Log {
				rt.logger.Logf("bucket %d not full yet -> inserting\n", bucketno)
			}
			bucket.hosts = bucket.hosts[0 : hl+1]
			bucket.hosts[hl] = rthost
		} else {
			if maxbucketno == bucketno {
				if rt.Log {
					rt.logger.Logf("bucket %d full -> dropping\n", bucketno)
				}
			} else {
				rt.newBucket()
				bucket.hosts = bucket.hosts[0 : hl+1]
				bucket.hosts[hl] = rthost
				rt.balanceleftright(uint(bucketno))
			}
		}
	} else {
		if rt.Log {
			rt.logger.Logf("we already have an entry\n")
		}
		host := bucket.hosts[pos]
		copy(bucket.hosts[pos:], bucket.hosts[pos+1:])
		bucket.hosts[len(bucket.hosts)-1] = host
	}
}


func (rt *BRoutingTable) newBucket() *Bucket {
	if rt.maxbucket == (HASHLEN*8 - 1) {
		return nil
	}

	b := new(Bucket)
	b.hosts = make([](*RTHost), K+1)[0:0]

	rt.maxbucket++
	rt.buckets[rt.maxbucket] = b

	return b
}


func (rt *BRoutingTable) findHost(h *Host) (bucketno, pos, maxbucketno int) {
	bucketno = -1
	pos = -1

	dist := XOR(h.Id, rt.id)
	maxbucketno = int(BucketNo(dist))

	maxbucket := rt.maxbucket
	if maxbucket < 0 {
		return
	}

	bucketno = maxbucketno

	if bucketno > maxbucket {
		bucketno = maxbucket
	}

	hosts := rt.buckets[bucketno].hosts
	for i, rh := range hosts {
		pos = i
		if rh.host.Id == h.Id {
			return
		}
	}

	pos = -1
	return
}


func (rt *BRoutingTable) balanceleftright(lefti uint) {
	righti := lefti + 1

	left := rt.buckets[lefti]
	right := rt.buckets[righti]

	newleft := make([](*RTHost), K+1)
	newright := make([](*RTHost), K+1)

	nleft := 0
	nright := 0

	for _, rth := range left.hosts {
		if BucketNo(rth.distance) == lefti {
			newleft[nleft] = rth
			nleft++
		} else {
			newright[nright] = rth
			nright++
		}
	}

	for _, rth := range right.hosts {
		if BucketNo(rth.distance) == lefti {
			newleft[nleft] = rth
			nleft++
		} else {
			newright[nright] = rth
			nright++
		}
	}

	rt.logger.Logf("rebalanced to %d/%d\n", nleft, nright)

	newleft = newleft[0:nleft]
	newright = newright[0:nright]

	left.hosts = newleft
	right.hosts = newright

	fmt.Printf("%v", rt)
}


// not goroutine safe
func (rt *BRoutingTable) string() string {
	buf := bytes.NewBuffer(nil)

	buf.WriteString("BRoutingTable ===>\n")
	for b := 0; b <= rt.maxbucket; b++ {
		buf.WriteString(fmt.Sprintf("bucket %d\n", b))
		for _, rth := range rt.buckets[b].hosts {
			buf.WriteString(fmt.Sprintf("\t%x | %v @ %v\n", rth.host.Id, XOR(rth.host.Id, rt.id)[0:5], rth.host.Addr))
		}
	}

	buf.WriteString("<===\n")
	return buf.String()
}

// not goroutine safe
func (rt *BRoutingTable) html() string {
	buf := bytes.NewBuffer(nil)

	buf.WriteString("BRoutingTable<br>\n<table>\n<tr>\n")

	for b := 0; b <= rt.maxbucket; b++ {
		buf.WriteString(fmt.Sprintf("<th>Bucket %d</th>\n", b))
	}

	buf.WriteString("</tr>\n")

	for b := 0; b <= rt.maxbucket; b++ {
		buf.WriteString("<td>")
		for _, rth := range rt.buckets[b].hosts {
			buf.WriteString(fmt.Sprintf("\t%x | %v @ %v<br>\n", rth.host.Id, XOR(rth.host.Id, rt.id)[0:5], rth.host.Addr))
		}
		buf.WriteString("</td>")
	}

	buf.WriteString("</tr>")

	buf.WriteString("</table>")
	return buf.String()
}


// goroutine safe. not for internal use!
func (rt *BRoutingTable) GetString() string {
	r := make(chan string)
	rt.stringchan <- r
	return <-r
}

// goroutine safe. not for internal use!
func (rt *BRoutingTable) GetHTML() string {
	r := make(chan string)
	rt.htmlchan <- r
	return <-r
}
