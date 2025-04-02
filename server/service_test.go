package server

import (
	"fmt"
	"reflect"
	"testing"
)

type Foo int

type Args struct {
	Num1, Num2 int
}

func (f *Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}

func TestNewService(t *testing.T) { // 判断是否能够获取结构体的方法
	var foo Foo
	s := newService(&foo)
	_assert(len(s.method) == 1, "wrong service Method, expect 1, but got %d", len(s.method))
	mType := s.method["Sum"]
	_assert(mType != nil, "wrong Method, Sum shouldn't nil")
}

func TestMethodType_NumCalls(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	mType := s.method["Sum"]

	arg := mType.newArgv()
	reply := mType.newReply()
	arg.Set(reflect.ValueOf(Args{Num1: 2, Num2: 3}))
	err := s.call(mType, arg, reply)
	_assert(err == nil && *reply.Interface().(*int) == 5 && mType.NumCalls() == 1, "failed to call Foo.Sum")
}
