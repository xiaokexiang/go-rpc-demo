package server

import (
	"go-rpc/service"
	"reflect"
	"testing"
)

func TestNewService(t *testing.T) {
	var foo service.Foo
	s := NewService(&foo) // 根据类型反射创建
	mType := s.Method["Sum"]
	argv := mType.NewArgv()
	reply := mType.NewReply()
	argv.Set(reflect.ValueOf(service.Args{Num1: 1, Num2: 2}))
	err := s.Call(mType, argv, reply)
	if err != nil || *reply.Interface().(*int) != 3 || mType.NumCalls != 1 {
		t.Error("failed to call Foo.Sum")
	}
}
