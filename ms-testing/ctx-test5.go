// The MIT License
//
// Copyright (c) 2020 Peter A McGill
//
// A testing program to test requirements for handling a microservice 
// concurrently running actorGroup. Test objectives :
// 1. Prove that a signal channel can correctly notify the handler when  
// each actor is either successfully complete or has failed
// 2. Prove that a single context instance can effectively broadcast
// a cancelation signal to all group actors.
// 3. Prove that a shared signal channel can handle concurrent writes
// when 2 or more actors attempt to write "simultaneously"
// 4. Prove that goroutine cancelation is stable such that resourse leakage
// due to orphaned goroutines doesn't happen
// 5. Prove that a handler is able to successfully dispatch an actorGroup 
// concurrently then wait for all members to complete or error so that the  
// companion service can be resumed seemlessly
// 6. Prove that ctrl-c successfully sends SIGTERM and that handler shutdowm
// is successful
package main

import (
  "context"
  crypto_rand "crypto/rand"
  "encoding/binary"	
  "fmt"
  "os"
  "os/signal"
  "math/rand"
  "sort"
  "sync"
  "syscall"
  "time"
)

func init() {
  var b [8]byte
  _, err := crypto_rand.Read(b[:])
  if err != nil {
	panic("cannot seed math/rand package with crypto random number generator")
  }
  seed := int64(binary.LittleEndian.Uint64(b[:]))
  if seed < 0 {
    seed = -seed
  }
  fmt.Printf("@@@@@@@@ init rand.seed : %v @@@@@@@@@\n",seed)
  rand.Seed(seed)
}

//================================================================//
// RandomRange
//================================================================//
type RandomRange struct {
	seed, min, max int
	src *rand.Rand
}

func (rr *RandomRange) rand(size int, newSrc ...bool) int {
  if rr.src == nil || (newSrc != nil && newSrc[0]) {
    src := rand.NewSource(time.Now().UnixNano())
    rr.src = rand.New(src)
  }
  return rr.src.Intn(size) // return a random number between 0 - size-1
}

// get next random value within the interval including min and max
func (rr *RandomRange) randSet(n int) ([]int, int) {
  arr := make([]int, n)
  total := 0
  next := 0
  for i,_ := range arr {
    next = (rand.Intn(rr.max - rr.min) + rr.min) * 50
    arr[i] = next
    total += next
  }
  // reset the seed value to ensure next use is randomized
  rr.seed = arr[0] 
  return arr, total
}

func (rr *RandomRange) init() {
  rand.Seed(time.Now().UnixNano() + int64(rr.seed))
}

//================================================================//
// DoneSignal
//================================================================//
type DoneSignal struct {
  id int
  error error
}

//================================================================//
// Actor
//================================================================//
type Actor struct {
  id int
  turnsLimit int
  randSet []int
  verbose bool	
  simError bool	
  active bool
}

//------------------------------------------------------------------//
// print
//------------------------------------------------------------------//
func (r *Actor) printf(logTxt string, args ...interface{}) {
  if r.verbose {
    fmt.Printf(logTxt, args...)
  }
}

//------------------------------------------------------------------//
// run
//------------------------------------------------------------------//
func (r *Actor) run(doneCh SignalChannel) {
  turn := 0
  naptime := 0
  for r.active {
    naptime = r.randSet[turn]
    r.printf("actor[%d] is napping for %d milliseconds\n",r.id,naptime)	
    for r.active {
      time.Sleep(50 * time.Millisecond)
      if naptime <= 0 {
        break
      }
      naptime -= 50
    }
    turn += 1
    if turn == r.turnsLimit {
      if r.simError {
	doneCh.write(DoneSignal{r.id,fmt.Errorf("actor[%d] produced an error\n",r.id)})
      } else {
        doneCh.write(DoneSignal{r.id,nil})
      }
      break
    }
  }
  fmt.Printf("actor[%d] run is now complete\n",r.id)
}

//------------------------------------------------------------------//
// start
//------------------------------------------------------------------//
func (r *Actor) start(ctx context.Context, doneCh SignalChannel) {
  fmt.Printf("actor[%d] is running ...\n",r.id)	
  r.active = true	
  go r.run(doneCh)
  for {
    select {
      case <-ctx.Done():
        r.active = false
	fmt.Printf("actor[%d] is canceled!\n",r.id)
	return
    }
  }
}

//================================================================//
// Handler
//================================================================//
type Handler struct {	
  groupSize int
  status int
}

//------------------------------------------------------------------//
// handle
//------------------------------------------------------------------//
func (h *Handler) handle(ctx context.Context, doneCh SignalChannel) {
  doneCounter := 0
  active := true
  for active {
    select {
      case signal, ok := <-doneCh.ch:
	if !ok {
	  fmt.Printf("!!!!!!! doneCh is closed !!!!!!!!")
	  break
	}
        if signal.error != nil {
	  active = false
	  fmt.Printf("microservice[%d] has failed, all actors are now cancelled\n%v\n",signal.id,signal.error)
	  h.status = 500
	  break
	}
	fmt.Printf("@@@@@@@@@@@ Microservice[%d] is complete @@@@@@@@@@\n",signal.id)
	doneCounter += 1
	if doneCounter == h.groupSize {
	  active = false
	  fmt.Printf("!!! Microservice group is now complete and successful !!!\n")
	  h.status = 200
	  break
	}
      case <-ctx.Done():
	active = false
	fmt.Printf("handler is canceled!\n")
	h.status = 409
        break
    }
  }
  fmt.Println("!!!!!!!!!! handler is complete !!!!!!!!!")	
}

//------------------------------------------------------------------//
// run
//------------------------------------------------------------------//
func (h *Handler) run(jpacket jobPacket, ctx10 context.Context) {
  //ctx, cancel := context.WithCancel(context.Background())
  ctx11, cancel := context.WithCancel(ctx10)
  doneCh := SignalChannel{
    ch: make(chan DoneSignal, h.groupSize)}
  defer close(doneCh.ch)
  actorGroup := NewActorGroup(jpacket)
  actorGroup.run(ctx11, doneCh)
  go h.sigterm(cancel)
  h.handle(ctx11, doneCh)
  if h.status != 200 {
    cancel()
  }
  // wait for actors to respond to cancelation before closing the doneCh
  time.Sleep(2 * time.Second)
}

//------------------------------------------------------------------//
// sigterm
//------------------------------------------------------------------//
func (h *Handler) sigterm(cancel context.CancelFunc) {
  defer cancel()
  sigCh := make(chan os.Signal, 1)
  signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
  fmt.Printf("ready to shutdown on SIGTERM signal ...\n")	
  signal := <-sigCh
  fmt.Printf("signal %v detected, shutting down ...\n",signal)
}

//================================================================//
// SignalChannel
//================================================================//
type SignalChannel struct {
  sync.Mutex
  ch chan DoneSignal
}

func (ch SignalChannel) write(signal DoneSignal) {
  ch.Lock()
  defer ch.Unlock()
  ch.ch <- signal
}

//================================================================//
// ActorGroup
//================================================================//
type ActorGroup struct {
  group []Actor	
}

//------------------------------------------------------------------//
// run
//------------------------------------------------------------------//
func (ag *ActorGroup) run(ctx context.Context, doneCh SignalChannel) {
  for i,_ := range ag.group {
    actor := ag.group[i]
    go actor.start(ctx, doneCh)
  }
}

//------------------------------------------------------------------//
// NewActorGroup
//------------------------------------------------------------------//
func NewActorGroup(pkt jobPacket) ActorGroup {
  randRange := RandomRange{0,pkt.randMin,pkt.randMax,nil}	
  actorGrp := make([]Actor, pkt.groupSize)
  totals := make([]int, pkt.groupSize)
  time2run := make(map[int]int, pkt.groupSize)	
  randSet := make([]int, pkt.turnsLimit)
  r := Actor{}
  var i, j, total, total_i int
  for i,_ := range actorGrp {
    j = i+1
    randRange.init()
    randSet, total = randRange.randSet(pkt.turnsLimit)
    total_i = total + i
    totals[i] = total_i
    // this is just to make sure total_i is unique, for the corner-case
    // where 2 or more totals are equal - simDupTot simulates this case
    time2run[total_i] = i
    r = Actor{j,pkt.turnsLimit,randSet,pkt.verbose,false,false}
    fmt.Printf("actor[%d] randset : %v\n",r.id,r.randSet)
    actorGrp[i] = r
  }
  if pkt.simError {
    i = randRange.rand(pkt.groupSize)
    actorGrp[i].simError = true
    fmt.Printf("actor[%d] will simulate an error, which triggers terminate all\n",actorGrp[i].id)
  } else if pkt.simDupTot {
    i = randRange.rand(pkt.groupSize)
    for k:=0; k < 1000; k++ {
      j = randRange.rand(pkt.groupSize)
      if j >= pkt.groupSize {
        fmt.Printf("rand returned a value out of range : %d\n",j)
      } else if i != j {
	fmt.Printf("found alt i index value j : %d, %d\n",i,j)
	break
      }
    }
    fmt.Printf("actors[%d][%d] will simulate duplicate totals, which creates a doneCh race condition\n",i,j)		
    total_j := totals[j]
    delete(time2run,total_j)
    for k,value := range actorGrp[i].randSet {
      fmt.Printf("copying actorGrp[%d].randSet[%d] to actorGrp[%d].randSet[%d]\n",i,k,j,k)
      actorGrp[j].randSet[k] = value
    }
    total_j = totals[i] - i + j
    totals[j] = total_j
    time2run[total_j] = j
  }
  sort.Ints(totals)
  fmt.Printf("//================== Expected finish order =========================//\n\n")
  for _,total_i := range totals {
    i := time2run[total_i]
    total = total_i - i
    j = i + 1
    fmt.Printf("//------------------ Microservice-%d, total : %d\n",j,total)
  }
  fmt.Printf("\n//==================================================================//\n")
  return ActorGroup{actorGrp}
}

//------------------------------------------------------------------//
// jobPacket
//------------------------------------------------------------------//
type jobPacket struct {
  groupSize int
  turnsLimit int
  randMin int
  randMax int
  verbose bool
  simError bool
  simDupTot bool
}

//------------------------------------------------------------------//
// main
//------------------------------------------------------------------//
func main() {
  groupSize := 7
  turnsLimit := 10
  randMin := 10
  randMax := 50
  verbose := false
  simError := true
  simDupTot := false
  var wg sync.WaitGroup
  packet := jobPacket{groupSize,turnsLimit,randMin,randMax,verbose,simError,simDupTot}
  handler := Handler{groupSize}
  ctx, cancel := context.WithCancel(context.Background())
  defer cancel()
  wg.Add(1)	
  go func() {
    handler.run(packet, ctx)
    wg.Done()
  }()
  fmt.Println("Waiting for completion ...")
  wg.Wait()
  fmt.Println("@@@@@@@@@@@ complete @@@@@@@@@@@@")
}
