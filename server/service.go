package server

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

// 通过反射创建service及其中的方法用于客户端调用执行
type methodType struct {
	method    reflect.Method // 调用方法
	ArgType   reflect.Type   // 请求参数类型
	ReplyType reflect.Type   // 返回类型，会传入指针类型
	NumCalls  uint64         // 方法被调用次数
}

type Service struct {
	name   string                 // 映射的结构体名称
	typ    reflect.Type           // 结构体类型
	self   reflect.Value          // 结构体实例本身
	Method map[string]*methodType // 结构体中所有符合条件的方法
}

// NewArgv 入参可以是或不是引用类型
func (m *methodType) NewArgv() reflect.Value {
	var arg reflect.Value
	if m.ArgType.Kind() == reflect.Ptr {
		arg = reflect.New(m.ArgType.Elem())
	} else {
		arg = reflect.New(m.ArgType).Elem()
	}
	return arg
}

// NewReply reply一定是引用类型，需要额外判断map和slice
func (m *methodType) NewReply() reflect.Value {
	reply := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Kind() {
	case reflect.Map:
		reply.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		reply.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return reply
}

// NewService 注册service及其中的方法
func NewService(v any) *Service {
	s := new(Service)
	s.self = reflect.ValueOf(v)
	s.typ = reflect.TypeOf(v)
	s.name = reflect.Indirect(s.self).Type().Name()
	if !ast.IsExported(s.name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	s.registerMethods() // 注册service中的方法
	return s
}

// 注册service中符合条件的方法
func (s *Service) registerMethods() {
	s.Method = make(map[string]*methodType)
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type
		// 过滤service中method入参数不是3（第0个参数是方法本身）,返回数不是1
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		// 过滤方法返回不是error的
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		aType, rType := mType.In(1), mType.In(2)
		if !isExportedOrBuildInType(aType) || !isExportedOrBuildInType(rType) {
			continue
		}
		s.Method[method.Name] = &methodType{
			ArgType:   aType,
			ReplyType: rType,
			method:    method,
		}
		log.Printf("rpc server: register %s.%s\n", s.name, method.Name)
	}
}

// 判断参数是exported的并且是内建类型
func isExportedOrBuildInType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

func (s *Service) Call(m *methodType, arg, reply reflect.Value) error {
	atomic.AddUint64(&m.NumCalls, 1)
	f := m.method.Func
	result := f.Call([]reflect.Value{s.self, arg, reply}) // 调用service中的方法，第一个参数是方法本身
	if err := result[0].Interface(); err != nil {
		return err.(error)
	}
	return nil
}
