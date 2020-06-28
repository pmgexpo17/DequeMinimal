// The MIT License
//
// Copyright (c) 2019 Peter A McGill
//
package main
import (
  "context"
  "fmt"
  "time"
  "sync"
)

// -------------------------------------------------------------- //
// NewDeque
// ---------------------------------------------------------------//
func NewDeque(dqitems []interface{}) Deque {
  items := []interface{}{}
  items = append(items,dqitems... )
  return Deque{items}
}

//==================================================================//
//  Deque
//==================================================================//
type Deque struct {
  items []interface{}
}

// -------------------------------------------------------------- //
// IsEmpty
// ---------------------------------------------------------------//
func (dq *Deque) IsEmpty() bool {
  return len(dq.items) == 0
}

// -------------------------------------------------------------- //
// Get
// ---------------------------------------------------------------//
func (dq *Deque) Get(i int) interface{} {
  if dq.IsEmpty() {
    panic("Can't get an item from an empty list")
  }
  if i < 0 || i >= len(dq.items) {
    panic("Invalid index")
  }
  return dq.items[i]
}

// -------------------------------------------------------------- //
// Insert
// ---------------------------------------------------------------//
func (dq *Deque) Insert(i int, item interface{}) {
  if dq.IsEmpty() {
    panic("Can't insert into an empty list")
  }
  if i < 0 || i >= len(dq.items) {
    panic("Invalid index")
  }
  sitem := []interface{}{item}
  temp := append(sitem,dq.items[i:]...)
  dq.items = append(dq.items[:i], temp...)
}

// -------------------------------------------------------------- //
// PopLeft
// ---------------------------------------------------------------//
func (dq *Deque) PopLeft() interface{} {
  if dq.IsEmpty() {
    panic("Can't pop an empty list")
  }
  item := dq.items[0]
  dq.items = dq.items[1:]
  return item
} 

// -------------------------------------------------------------- //
// PopRight
// ---------------------------------------------------------------//
func (dq *Deque) PopRight() interface{} {
  if dq.IsEmpty() {
    panic("Can't pop an empty list")
  }
  last := len(dq.items) -1
  item := dq.items[last] 
  dq.items = dq.items[:last]
  return item
}

// -------------------------------------------------------------- //
// PushLeft
// ---------------------------------------------------------------//
func (dq *Deque) PushLeft(item interface{}) { 
  temp := []interface{}{item} 
  dq.items = append(temp, dq.items...) 
} 

// -------------------------------------------------------------- //
// PushRight
// ---------------------------------------------------------------//
func (dq *Deque) PushRight(item interface{}) {
  dq.items = append(dq.items, item)
}

// -------------------------------------------------------------- //
// Reset
// ---------------------------------------------------------------//
func (dq *Deque) Reset() {
  dq.items = []interface{}{}
}

// -------------------------------------------------------------- //
// Set
// ---------------------------------------------------------------//
func (dq *Deque) Set(i int, item interface{}) {
  if dq.IsEmpty() {
    panic("Can't set into an empty list")
  }
  if i < 0 || i >= len(dq.items) {
    panic("Invalid index")
  }
  dq.items[i] = item
}

// -------------------------------------------------------------- //
// String
// ---------------------------------------------------------------//
func (dq *Deque) String() string {
  return fmt.Sprintf("%v",dq.items)
}

//================================================================//
// TestCase
//================================================================//
type TestCase struct {
  tdesc string
  qkey int
  qval int
}

//================================================================//
// TestSuite
//================================================================//
type TestSuite struct {
  Deque
  suite []TestCase
  testCh chan int
  ctx context.Context
  tnum int
}

// -------------------------------------------------------------- //
// runTests
// ---------------------------------------------------------------//
func (ts *TestSuite) runTests(tnext ...int) {
  if tnext != nil {
    fmt.Printf("Recovered from panic, next id : %d\n",tnext[0])
    ts.testCh<- tnext[0]
  }
  ts.doTests()  
}

// -------------------------------------------------------------- //
// doTests
// ---------------------------------------------------------------//
func (ts *TestSuite) doTests() {
  defer ts.panicHandler()  
  for {
    select {
      case tnum, _ := <-ts.testCh:
        fmt.Printf("Running test case #%d\n",tnum)
        ts.doTest(tnum)
        tnext := tnum + 1
        ts.testCh<- tnext
      case <-ts.ctx.Done():
        fmt.Printf("Testsuite is now complete\n")
        return
    }
  }
}

// -------------------------------------------------------------- //
// panicHandler
// ---------------------------------------------------------------//
func (ts *TestSuite) panicHandler() {
  if r := recover(); r != nil {
    fmt.Println("Recovered by panicHandler : ", r)
    tnext := ts.tnum + 1
    fmt.Printf("Resuming next test case : %d\n",tnext)
    ts.runTests(tnext)
  }
}

// -------------------------------------------------------------- //
// doTest
// ---------------------------------------------------------------//
func (ts *TestSuite) doTest(tnum int) {
  ts.tnum = tnum
  tc := ts.suite[tnum]  

  var item int
  var status bool
  switch tc.tdesc {
  case "NewDeque":    
    q1 := NewDeque([]interface{}{1,2,3})  
    fmt.Printf("test%d, items: %v\n",1,q1.items)
  case "PopLeft":    
    item = ts.PopLeft().(int)
    fmt.Printf("PopLeft, item : %d, items: %v\n",item, ts.items)
  case "PopRight":
    item = ts.PopRight().(int)
    fmt.Printf("PopRight,item : %d, items: %v\n",item, ts.items)
  case "PushLeft":
    ts.PushLeft(tc.qval)
    fmt.Printf("PushLeft, item : %d, items: %v\n",tc.qval, ts.items)
  case "PushRight":
    ts.PushRight(tc.qval)
    fmt.Printf("PushRight, item : %d, items: %v\n",tc.qval, ts.items)
  case "IsEmpty":
    status = ts.IsEmpty()
    if status != true {
      panic("Failed to correctly handle empty status\n")
    } else {
      fmt.Printf("IsEmpty, status : %v, items: %v\n",status, ts.items)
    }
  case "IsNotEmpty":
    status = ts.IsEmpty()
    if status != false {
      panic("Failed to correctly handle empty status\n")
    } else {
      fmt.Printf("IsEmpty, status : %v, items: %v\n",status, ts.items)
    }
  case "Set":
    ts.Set(tc.qkey,tc.qval)
    fmt.Printf("Set, index: %d, value: %d, items: %v\n",tc.qkey, tc.qval, ts.items)
  case "Insert":
    ts.Insert(tc.qkey,tc.qval)
    fmt.Printf("Insert, index: %d, value: %d, items: %v\n",tc.qkey, tc.qval, ts.items)
  case "Reset":
    ts.Reset()
    fmt.Printf("Reset, items: %v\n",ts.items)
  case "Get":
    item = ts.Get(tc.qkey).(int)
    fmt.Printf("Get, index: %d, item : %d, items: %v\n",tc.qkey, item, ts.items)
  case "String":
    stritems := ts.String()
    fmt.Printf("String, items: %v\n",stritems)
  }
}

//================================================================//
// TestRunner
//================================================================//
type TestRunner struct {
  testCh chan int
  ctx context.Context
  wg *sync.WaitGroup
}

// -------------------------------------------------------------- //
// testRunner
// ---------------------------------------------------------------//
func (tr TestRunner) run(tnum, tmax int, cancel context.CancelFunc) {
  tr.testCh<- tnum
  for {
    select {
    case <-tr.ctx.Done():
      fmt.Println("Testing is canceled by timeout\n")      
      return
    case tnext, _ := <-tr.testCh:        
      fmt.Printf("Next testcase, #%d\n",tnext)
      if tnext > tmax {
        tr.wg.Done()
        cancel()
        fmt.Println("Test runner is now complete\n")      
        return
      }
      tr.testCh<- tnext
    }
  }  
}

func main() { 
  testsuite := []TestCase {
    {"PopLeft",0,0},
    {"PopRight",0,0},
    {"PopLeft",0,0},
    {"IsEmpty",0,0},
    {"PopLeft",0,0},
    {"PopRight",0,0},
    {"PushLeft",0,4},
    {"PushRight",0,5},
    {"Reset",0,0},
    {"Get",1,0},    
    {"IsEmpty",0,0},
    {"PushRight",0,15},
    {"Get",0,0},    
    {"IsNotEmpty",0,0},
    {"PushLeft",0,14},
    {"PushLeft",0,24},
    {"PushRight",0,25},        
    {"Set",1,55},
    {"Insert",1,66},
    {"Get",3,0},
    {"String",0,0}}
  var wg sync.WaitGroup
  tnum, tmax := 0, len(testsuite) - 1
  testCh := make(chan int)
  defer close(testCh)
  dq := NewDeque([]interface{}{1,2,3})
  fmt.Printf("Initial deque data : %v\n",dq)
  ctx, cancel := context.WithTimeout(context.Background(),time.Second*5)
  defer cancel()
  tsuite := TestSuite{dq,testsuite,testCh,ctx,tnum}
  go tsuite.doTests()
  trunner := TestRunner{testCh,ctx,&wg}
  wg.Add(1)
  go trunner.run(tnum,tmax,cancel)  
  wg.Wait()
}
