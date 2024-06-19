package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"path/filepath"
)

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string, coordinatorAddress string) {

	// Your worker implementation here.

	// uncomment to send the Example RPC to the coordinator.
	// CallExample()
	log.Println("Worker started")

	for {
		task := RequestTask(coordinatorAddress)

		log.Printf("Received task: %+v\n", task)
		var num int
		// fmt.Println(task.MapFile, task.IsMapTask, task.ReduceNo)

		// Quit the loop if it is sentinal
		if !task.IsMapTask && task.ReduceNo == -1 {
			log.Println("No more tasks, terminating worker")
			return
		}

		if task.IsMapTask {
			log.Printf("Processing map task: %s\n", task.MapFile)

			// Makes n-reduce intermideates
			intermediate := make([]map[string][]string, task.ReduceNo)
			file, err := os.Open(task.MapFile)

			if err != nil {
				log.Fatalf("cannot open %v", task.MapFile)
			}

			content, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatalf("cannot read %v", task.MapFile)
			}

			file.Close()
			kva := mapf(task.MapFile, string(content))

			// Sort and partially reduce
			partial_reduce := make(map[string][]string)

			for _, kv := range kva {
				partial_reduce[kv.Key] = append(partial_reduce[kv.Key], kv.Value)
			}

			// Assign respective buckets
			for key, values := range partial_reduce {
				i := ihash(key) % task.ReduceNo
				if intermediate[i] == nil {
					intermediate[i] = make(map[string][]string)
				}
				// Append the key value pair in the respective bucket
				// output := reducef(key, values)
				intermediate[i][key] = values
			}

			// Write out all the intermediate buckets (CAN'T CUZ STUPID)
			for i, buckets := range intermediate {
				if buckets == nil {
					continue
				}
				o_name := fmt.Sprintf("mr-%d-%d", i, task.MapNum)
				jsonData, err := json.Marshal(buckets)
				if err != nil {
					log.Fatalf("Error marshalling to JSON: %v", err)
				}

				o_file, _ := os.Create(o_name)
				if err != nil {
					log.Fatalf("Error creating file %s: %v", o_name, err)
				}
				fmt.Fprintf(o_file, string(jsonData))
				log.Printf("Wrote intermediate data to %s\n", o_name)
			}
			num = task.MapNum

		} else {
			log.Printf("Processing reduce task: %d\n", task.ReduceNo)
			// it is a reduce task and we shall handle it differently
			pattern := fmt.Sprintf("mr-%d-*", task.ReduceNo)
			files, err := filepath.Glob(pattern)
			if err != nil {
				log.Fatalf("Error matching files: %v", err)
			}

			inter_map := make(map[string][]string)

			for _, file := range files {
				data, err := ioutil.ReadFile(file)
				if err != nil {
					log.Printf("Error reading file %s: %v\n", file, err)
					return
				}

				var fileMap map[string][]string

				err = json.Unmarshal(data, &fileMap)
				if err != nil {
					log.Printf("Error unmarshaling file %s: %v\n", file, err)
					return
				}

				for key, val := range fileMap {
					inter_map[key] = append(inter_map[key], val...)
				}

			}

			o_name := fmt.Sprintf("mr-out-%d", task.ReduceNo)
			of, _ := os.Create(o_name)

			// Reduce the contents and output the result
			for key, values := range inter_map {
				out := reducef(key, values)
				// fmt.Printf("%v %v %v %v\n",key, values, values[0] , out)
				fmt.Fprintf(of, "%v %v\n", key, out)
			}
			log.Printf("Wrote reduce output to %s\n", o_name)
			num = task.ReduceNo
		}

		// Done Task
		args := DoneTaskArgs{
			IsMapTask: task.IsMapTask,
			TaskNum:   num,
		}
		// Send done task
		DoneTask(args, coordinatorAddress)
		log.Printf("Completed task: %d (is map task: %v)\n", num, task.IsMapTask)
	}

}

func RequestTask(coordinatorAddress string) RequestTaskReply {

	args := RequestTaskArgs{}
	reply := RequestTaskReply{}

	ok := call("Coordinator.RequestTask", &args, &reply, coordinatorAddress)
	if ok {
		log.Println("Successfully requested task from coordinator")
		return reply
	} else {
		log.Println("Failed to request task from coordinator")
		// fmt.Printf("call failed!\n")
	}
	return reply
}

func DoneTask(args DoneTaskArgs, coordinatorAddress string) {

	reply := DoneTaskReply{}

	ok := call("Coordinator.DoneTask", &args, &reply, coordinatorAddress)
	if ok {
		log.Println("Successfully reported task completion to coordinator")
	} else {
		log.Println("Failed to report task completion to coordinator")
		// fmt.Printf("call failed!\n")
	}
}

// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
func CallExample(coordinatorAddress string) {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply, coordinatorAddress)
	if ok {
		// reply.Y should be 100.
		// fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		// fmt.Printf("call failed!\n")
	}
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}, coordinatorAddress string) bool {
	c, err := rpc.DialHTTP("tcp", coordinatorAddress)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}
	log.Printf("RPC call %s failed: %v\n", rpcname, err)

	return false
}
