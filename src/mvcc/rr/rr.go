package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var tnxVersion int64
var ACTIVE_TXN []int64
var kvEngine map[string]*Record
var lock sync.Mutex

func acquireMinMaxId() (int64, int64) {
	return ACTIVE_TXN[0], ACTIVE_TXN[len(ACTIVE_TXN)-1] + 1
}

func acquireNextVersion() int64 {
	add := atomic.AddInt64(&tnxVersion, 1)
	return add
}

type Log struct {
	key   string // 记录行标志
	value string // 实际数据
	txnId int64  // 绑定的事务ID
}

type Record struct {
	mux     sync.Mutex
	single  chan int
	undoLog []Log
}

type Transaction struct {
	keys       []string
	kvEngine   map[string]*Record // 以前事务版本的Record 也要能读到,每次启动都拷贝一份;
	currTxnId  int64              // 当前事务版本号
	activeTxns []int64            // 当前事务启动时,活跃事务ID列表
	minTxnId   int64
	maxTxnId   int64
}

func newTransaction() *Transaction {
	return &Transaction{
		currTxnId:  0,
		activeTxns: make([]int64, 0),
	}
}

func (t *Transaction) begin() *Transaction {
	t.currTxnId = acquireNextVersion() // 获得全局递增唯一事务id
	lock.Lock()
	newKv := make(map[string]*Record)
	for key, record := range kvEngine {
		newKv[key] = record
		copy(newKv[key].undoLog, record.undoLog)
	}
	t.kvEngine = newKv
	ACTIVE_TXN = append(ACTIVE_TXN, t.currTxnId) // 将自己添加到活跃列表中
	copy(t.activeTxns, ACTIVE_TXN)
	t.minTxnId, t.maxTxnId = acquireMinMaxId()
	lock.Unlock()
	return t
}

func (t *Transaction) get(key string) Log {
	record := kvEngine[key]
	if len(record.undoLog) <= 0 {
		return Log{}
	}
	var log Log
	for i := len(record.undoLog) - 1; i >= 0; i-- {
		log = record.undoLog[i]
		if log.txnId < t.minTxnId || log.txnId == t.currTxnId {
			return log
		}
		if log.txnId >= t.maxTxnId {
			continue
		}
		// (t.minTxnId, ..., t.maxTxnId)
		// [t.activeTxns] // 判断是否在活跃列表中,存在,不能读;
		// 并且在可重复读的情况下, 也不会出现 id没有在活跃列表中, 因为都是提前拷贝的,没有进行删除操作;
		for _, txn := range t.activeTxns {
			if log.txnId == txn {
				continue
			}
		}
	}
	return log
}

func (t *Transaction) write(key, value string) {
	record := kvEngine[key]
	if record == nil {
		record = &Record{
			single:  make(chan int, 1),
			undoLog: make([]Log, 0),
		}
		record.undoLog = append(record.undoLog, Log{
			key:   key,
			value: value,
			txnId: t.currTxnId,
		})
		if lock.TryLock() {
			rec := kvEngine[key]
			if rec != nil {
				return
			}
			kvEngine[key] = record
			lock.Unlock()
			return
		} else {
			t.write(key, value)
		}
	}
	<-record.single
	record = kvEngine[key]
	t.keys = append(t.keys, key)
	record.undoLog = append(record.undoLog, Log{
		key:   key,
		value: value,
		txnId: t.currTxnId,
	})
	lock.Lock()
	kvEngine[key] = record
	lock.Unlock()
}

func (t *Transaction) commit() {
	for _, key := range t.keys {
		record := kvEngine[key]
		lock.Lock()
		RemoveTxnId(&ACTIVE_TXN, t.currTxnId)
		lock.Unlock()
		record.single <- 1
	}
}

func RemoveTxnId(src *[]int64, txn int64) {
	for i, txnId := range *src {
		if txnId == txn {
			*src = append((*src)[:i], (*src)[i+1:]...)
			return
		}
	}
}

func (t *Transaction) rollback() {
	for _, key := range t.keys {
		record := kvEngine[key]
		isMy := record.undoLog[len(record.undoLog)-1]
		if isMy.txnId == t.currTxnId {
			record.undoLog = record.undoLog[:len(record.undoLog)-1]
			lock.Lock()
			RemoveTxnId(&ACTIVE_TXN, t.currTxnId)
			kvEngine[key] = record
			lock.Unlock()
			record.single <- 1
		} else {
			return
		}
	}
}

func main() {
	kvEngine = make(map[string]*Record)
	kvEngine["a"] = &Record{
		single: make(chan int, 1),
		undoLog: append(make([]Log, 0), Log{
			key:   "a",
			value: "a0",
			txnId: -1,
		}),
	}
	kvEngine["a"].single <- 1
	go func() {
		t1 := newTransaction()
		t1.begin()
		fmt.Printf("0. t1.get(a):%s \n", t1.get("a").value)
		t1.write("a", "a1")
		fmt.Printf("1. t1.get(a):%s \n", t1.get("a").value)
		time.Sleep(2 * time.Second)
		fmt.Printf("2. t1.get(a):%s \n", t1.get("a").value)
		t1.commit()
		t3 := newTransaction()
		t3.begin()
		fmt.Printf("5. t3.get(a):%s \n", t3.get("a").value)
	}()

	go func() {
		t2 := newTransaction()
		t2.begin()
		fmt.Printf("3. t2.get(a):%s \n", t2.get("a").value)
		t2.write("a", "a2")
		fmt.Printf("4. t2.get(a):%s \n", t2.get("a").value)
		t2.commit()
		t4 := newTransaction()
		t4.begin()
		fmt.Printf("6. t4.get(a):%s \n", t4.get("a").value)
	}()
	for {
		select {
		case <-time.After(3 * time.Second):
			fmt.Printf("kvEngine[a].undoLog:%v \n", kvEngine["a"].undoLog)
		}
	}
}
