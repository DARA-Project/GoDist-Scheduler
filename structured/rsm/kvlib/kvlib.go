package kvlib

import (
	"net/rpc"
	"errors"
	"time"
	"sync"
)

// args in get(args)
type GetArgs struct {
	Key      string // key to look up
	Clientid uint8  // client id issuing this get
	Clock    uint64 // value of lamport clock at the issuing client
}

// args in put(args)
type PutArgs struct {
	Key      string // key to associate value with
	Val      string // value
	Clientid uint8  // client id issueing this put
	Clock    uint64 // value of lamport clock at the issuing client
}

// args in clockupdate(args)
type ClockUpdateArgs struct {
	Clientid uint8  // client id issueing this put
	Clock    uint64 // value of lamport clock at the issuing client
}

// args in disconnect(args)
type DisconnectArgs struct {
	Clientid uint8 // client id issueing this put
}

// Reply from service for all the API calls above.
type ValReply struct {
	Val string // value; depends on the call
}

type KVService struct {
	Clientid uint8
	ClockUpdateRate uint8
	Clock uint64
	Replicas []*rpc.Client
	Mux sync.Mutex
}

func (kv *KVService) Get(key string) (string, error) {
	var err error
	kv.Mux.Lock()
	kv.Clock += 1
	args := GetArgs{key, kv.Clientid, kv.Clock}
	kv.Mux.Unlock()
	var reply ValReply
	for _, c := range kv.Replicas {
		err = c.Call("KeyValService.Get", &args, &reply)
		if err == nil {
			return reply.Val, nil
		}
	}
	return reply.Val, err
}

func (kv *KVService) Put(key string, val string) error {
	var err error
	kv.Mux.Lock()
	kv.Clock += 1
	putArgs := PutArgs{key, val, kv.Clientid, kv.Clock}
	kv.Mux.Unlock()
	var reply ValReply
	num_errors := 0
	for _, c := range kv.Replicas {
		err = c.Call("KeyValService.Put", &putArgs, &reply)
		if err != nil {
			num_errors += 1
		}
	}
	if num_errors == len(kv.Replicas) {
		return errors.New("All replicas failed.")
	}
	return nil
}

func (kv *KVService) Disconnect() error {
	var err error
	kv.Mux.Lock()
	kv.Clock += 1
	kv.Mux.Unlock()
	dcArgs := DisconnectArgs{kv.Clientid}
	var reply ValReply
	for _, c := range kv.Replicas {
		err = c.Call("KeyValService.Disconnect", &dcArgs, &reply)
	}
	return err
}

func (kv *KVService) clockUpdate() {
	ticker := time.NewTicker(time.Duration(kv.ClockUpdateRate) * time.Millisecond)
	for {
		select {
			case <- ticker.C:
				kv.Mux.Lock()
				kv.Clock += 1
				kv.Mux.Unlock()
				args := ClockUpdateArgs{kv.Clientid, kv.Clock}
				var reply ValReply
				for _, c := range kv.Replicas {
					c.Call("KeyValService.ClockUpdate", &args, &reply)
				}
		}
	}
}

func Init(id uint8, updateRate uint8, addresses []string) (*KVService, error) {
	clients := []*rpc.Client{}
	for _, s := range addresses {
		client, err := rpc.Dial("tcp", s)
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)
	}
	kvService := &KVService{Clientid : id, ClockUpdateRate : updateRate, Clock : 0, Replicas : clients}
	go kvService.clockUpdate()
	return kvService, nil
}
