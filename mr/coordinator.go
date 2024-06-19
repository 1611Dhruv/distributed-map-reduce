package mr

import (
//	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)


type Coordinator struct {
	// Your definitions here.
    mapFiles chan FileTuple
    reduceNums chan int
    numFiles int
    nReduce  int
    nMapDone int
    doneMapFiles []bool
    doneReduces []bool
    mu sync.Mutex
    isDone bool
}

type FileTuple struct {
    name string
    num int
}

// Your code here -- RPC handlers for the worker to call.
func (c *Coordinator) RequestTask(args *RequestTaskArgs, reply *RequestTaskReply) error {
    
    // Go back whenever a channel is closed
    START: 
    // fmt.Println("Someone is requesting a task");
    // Stage 1: all mappings haven't been completed yet
    if c.nMapDone != c.numFiles {
        taskFile, ok := <-c.mapFiles 
        if !ok {
            // fmt.Println("No more maps to assigns diverting req to start")
            goto START
        }

        // fmt.Println("Assigning map task: ", taskFile)
        reply.IsMapTask = true
        reply.MapFile = taskFile.name
        reply.MapNum = taskFile.num
        reply.ReduceNo = c.nReduce

        // Start up a daemon thread to followup later
        go func (fileCheck FileTuple) {
            // fmt.Println(fileCheck," daemon sleeping")
            time.Sleep(time.Second * 10)

            // If the done at this task num updated
            if c.doneMapFiles[fileCheck.num] {
                // fmt.Println(fileCheck," daemon found file")
                return;
            }
            
            // Otherwise reassign
            // fmt.Println(fileCheck," daemon could not find, reassigning")
            c.mapFiles<- fileCheck
        } (taskFile)
        return nil
    }

    if c.nReduce > 0 {
        // Get the reduce number
        // fmt.Println("Assigning Reduce")
        reduceNum, ok := <-c.reduceNums
        if !ok {
            // fmt.Println("No more reduces to assigns diverting req to start")
            goto START
        }

        // fmt.Println("Assigning reduce task: ", reduceNum)
        reply.IsMapTask = false
        reply.ReduceNo = reduceNum
        
        // Start up a daemon thread to followup later
        go func (reduceCheck int) {
            // fmt.Println(reduceCheck," daemon sleeping")
            time.Sleep(time.Second * 10)

            // If the done at this task num updated
            if c.doneReduces[reduceCheck] {
                // fmt.Println(reduceCheck," daemon completed")
                return;
            }
            
            // Otherwise reassign
            // fmt.Println(reduceCheck," daemon could not find, reassigning")
            c.reduceNums<- reduceCheck
        } (reduceNum)
        return nil
    }

    // fmt.Println("Done with all tasks, sending termination")
    // Otherwise send a termination reply
    reply.IsMapTask = false
    // SENTINAL Value -1 to suggest that you can terminate now
    reply.ReduceNo = -1

    return nil;
}

func (c *Coordinator) DoneTask(args *DoneTaskArgs, reply *DoneTaskReply) error {
    if args.IsMapTask {
        if c.doneMapFiles[args.TaskNum] {
            // fmt.Println("Task already completed")
            return nil
        }

        c.mu.Lock()
        defer c.mu.Unlock()
        c.doneMapFiles[args.TaskNum] = true
        c.nMapDone++
        // fmt.Printf("Succesfully completed map task %v\n",c.doneMapFiles)
        if c.nMapDone == c.numFiles {
            // fmt.Println("All tasks done, closing map channel")
            // Close the map channel if we mapped all the files
            close(c.mapFiles)
            // Start a thread which will send reduce tasks
            go func() {
                n := c.nReduce
                for i:=0; i < n; i++ {
                    // fmt.Println("Assigning ", i)
                    c.reduceNums<-i
                }
            }() 
        }
    } else {
        if c.doneReduces[args.TaskNum] {
            // fmt.Println("Task already completed")
            return nil
        }

        c.mu.Lock()
        defer c.mu.Unlock()
        c.doneReduces[args.TaskNum] = true
        c.nReduce--
        // fmt.Printf("Succesfully completed reduce task %v\n",c.doneReduces)
        if c.nReduce == 0 {
            // fmt.Println("All tasks done, closing reduce channel")
            // Close the map channel if we mapped all the files
            close(c.reduceNums)
            c.isDone = true
        }
    }
    return nil
}

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
    // fmt.Printf("Recieved a request %v\n",args)
	reply.Y = args.X + 1
    time.Sleep(time.Second * 5)
	return nil
}


//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	return c.isDone
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
    
    // Store the files and nReduce
    c := Coordinator{
        mapFiles: make(chan FileTuple),
        reduceNums: make(chan int),
        numFiles: len(files),
        nReduce:  nReduce,
        nMapDone: 0,
        mu: sync.Mutex{},
        doneMapFiles: make([]bool, len(files)),
        doneReduces: make([]bool, nReduce),
    }

	// Your code here.

    // Keep sending map files to the channel
    go func() {
        for i,file := range files {
            c.mapFiles <- FileTuple{file,i}
        }
    } ()


	c.server()
	return &c
}
