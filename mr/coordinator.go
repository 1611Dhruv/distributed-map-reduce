package mr

import (
	//	"fmt"
	"context"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type Coordinator struct {
	// Your definitions here.
	mapFiles     chan FileTuple
	gcpBucket    string
	reduceNums   chan int
	numFiles     int
	nReduce      int
	nMapDone     int
	doneMapFiles []bool
	doneReduces  []bool
	mu           sync.Mutex
	isDone       bool
}

type FileTuple struct {
	name string
	num  int
}

func (c *Coordinator) CleanUp() {
	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithCredentialsFile("key.json"))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Cleaning Up")
	bucket := client.Bucket(c.gcpBucket)
	it := bucket.Objects(ctx, &storage.Query{
		MatchGlob: "mr-[0-9]*-*",
	})

	for {
		objAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		log.Println(objAttrs.Name)
		if bucket.Object(objAttrs.Name).Delete(ctx) != nil {
			log.Fatal(err)
		}
	}
}

// Your code here -- RPC handlers for the worker to call.
func (c *Coordinator) RequestTask(args *RequestTaskArgs, reply *RequestTaskReply) error {
	log.Println("Worker requested a task")

	// give the gcp bucket to the worker
	reply.GcpBucket = c.gcpBucket

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

		log.Printf("Assigning map task: %s (number %d)\n", taskFile.name, taskFile.num)
		reply.IsMapTask = true
		reply.MapFile = taskFile.name
		reply.MapNum = taskFile.num
		reply.ReduceNo = c.nReduce

		// Start up a daemon thread to followup later
		go func(fileCheck FileTuple) {
			// fmt.Println(fileCheck," daemon sleeping")
			time.Sleep(time.Second * 10)

			// If the done at this task num updated
			if c.doneMapFiles[fileCheck.num] {
				// fmt.Println(fileCheck," daemon found file")
				return
			}

			// Otherwise reassign
			// fmt.Println(fileCheck," daemon could not find, reassigning")
			c.mapFiles <- fileCheck
		}(taskFile)
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

		log.Printf("Assigning reduce task: %d\n", reduceNum)
		reply.IsMapTask = false
		reply.ReduceNo = reduceNum

		// Start up a daemon thread to followup later
		go func(reduceCheck int) {
			// fmt.Println(reduceCheck," daemon sleeping")
			time.Sleep(time.Second * 10)

			// If the done at this task num updated
			if c.doneReduces[reduceCheck] {
				// fmt.Println(reduceCheck," daemon completed")
				return
			}

			// Otherwise reassign
			// fmt.Println(reduceCheck," daemon could not find, reassigning")
			c.reduceNums <- reduceCheck
		}(reduceNum)
		return nil
	}

	log.Println("All tasks completed, sending termination signal")
	// Otherwise send a termination reply
	reply.IsMapTask = false
	// SENTINAL Value -1 to suggest that you can terminate now
	reply.ReduceNo = -1

	return nil
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
		log.Printf("Map task %d completed\n", args.TaskNum)
		if c.nMapDone == c.numFiles {
			log.Println("All map tasks completed")
			// Close the map channel if we mapped all the files
			close(c.mapFiles)
			// Start a thread which will send reduce tasks
			go func() {
				n := c.nReduce
				for i := 0; i < n; i++ {
					// fmt.Println("Assigning ", i)
					c.reduceNums <- i
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
		log.Printf("Reduce task %d completed\n", args.TaskNum)
		if c.nReduce == 0 {
			log.Println("All reduce tasks completed")
			// Close the map channel if we mapped all the files
			close(c.reduceNums)
			// Cleanup the intermediate files
			c.CleanUp()
			c.isDone = true
		}
	}
	return nil
}

// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	// fmt.Printf("Recieved a request %v\n",args)
	reply.Y = args.X + 1
	time.Sleep(time.Second * 5)
	return nil
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	sockname := coordinatorSock()
	l, e := net.Listen("tcp", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	log.Println("Coordinator server started on", l.Addr().String())
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	return c.isDone
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {

	// Store the files and nReduce
	c := Coordinator{
		mapFiles:     make(chan FileTuple),
		reduceNums:   make(chan int),
		numFiles:     len(files) - 1,
		nReduce:      nReduce,
		nMapDone:     0,
		mu:           sync.Mutex{},
		gcpBucket:    files[0],
		doneMapFiles: make([]bool, len(files)-1),
		doneReduces:  make([]bool, nReduce),
	}

	// Your code here.

	log.Printf("Coordinator created with %d map tasks and %d reduce tasks\n", c.numFiles, c.nReduce)
	// Keep sending map files to the channel
	go func() {
		for i, file := range files {
			// Skip the first argument
			if i == 0 {
				continue
			}
			c.mapFiles <- FileTuple{file, i - 1}
			log.Printf("Added map file %s (number %d) to the queue\n", file, i)
		}
	}()

	c.server()
	return &c
}
