package mr

import (
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"sort"
)

//
// Map functions return a slice of KeyValue.
//
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

var workerId int64 = -1
var taskID int = -1

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.
	for {
		req := &Req{
			TaskID:   taskID,
			WorkerId: workerId,
		}
		rsp := &Rsp{}
		call("Coordinator.AskForTask", req, rsp)
		type_ := rsp.Type
		filenamePrefix := rsp.FileNamePrefix
		nReduce := rsp.NReduce
		taskID = rsp.TaskID
		if workerId == -1 {
			workerId = rsp.WorkerId
		}
		switch type_ {
		case 0:
			reduceFileMap, err := createReduceFile(taskID, nReduce)
			if err != nil {
				log.Fatalf("create reduce files failed, err: %v", err)
				return
			}
			file, err := os.Open(filenamePrefix)
			if err != nil {
				log.Fatalf("open source file %v failed", filenamePrefix)
			} else {
				if bytes, err := ioutil.ReadAll(file); err != nil {
					log.Fatalf("cannot read %v", file)
				} else {
					kvs := mapf(filenamePrefix, string(bytes))
					sort.Sort(ByKey(kvs))
					for _, kv := range kvs {
						key := kv.Key
						val := kv.Value
						idx := ihash(key) % nReduce
						interFilename := fmt.Sprintf("mr-%d-%d-%d", taskID, idx, workerId)
						fmt.Fprintf(reduceFileMap[interFilename], "%v %v\n", key, val)
					}
				}
			}
			file.Close()
			closeReduceFile(reduceFileMap)
		case 1:
		default:
		}
		fmt.Printf("%v\n", rsp)
	}

}

func createReduceFile(taskID int, nReduce int) (map[string]*os.File, error) {
	m := make(map[string]*os.File)
	for i := 0; i < nReduce; i++ {
		filename := fmt.Sprintf("mr-%d-%d-%d", taskID, i, workerId)
		file, err := os.Create(filename)
		if err != nil {
			return nil, err
		}
		m[filename] = file
	}
	return m, nil
}

func closeReduceFile(files map[string]*os.File) error {
	for _, v := range files {
		if err := v.Close(); err != nil {
			return err
		}
	}
	return nil
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
