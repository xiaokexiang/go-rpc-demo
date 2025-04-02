package server

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

type methodType struct {
	method    reflect.Method // 方法本身
	ArgType   reflect.Type   // 参数类型
	ReplyType reflect.Type   // 响应类型
	numCalls  uint64         // 方法被调用次数
}

func (m *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls) // 原子获取numCalls
}

func (m *methodType) newArgv() reflect.Value {
	var arg reflect.Value
	if m.ArgType.Kind() == reflect.Pointer {
		arg = reflect.New(m.ArgType.Elem())
	} else {
		arg = reflect.New(m.ArgType).Elem() // 传入的是非指针，也需要转换回来
	}
	return arg
}

// reply必须是指针类型
func (m *methodType) newReply() reflect.Value {
	reply := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		reply.Elem().Set(reflect.MakeMap(m.ReplyType.Elem())) // reflect.New()创建的map是nil，所以需要创建一个可用的空map
	case reflect.Slice:
		reply.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0)) // 同理map
	default:
	}
	return reply
}

// 用于将结构体转换为service
type service struct {
	name   string                 // 映射的结构体名称
	typ    reflect.Type           // 结构体类型
	self   reflect.Value          // service实例本身作为反射参数
	method map[string]*methodType // 结构体中的方法
}

// 服务端定义的结构体映射为服务
func newService(self any) *service {
	s := new(service)
	s.self = reflect.ValueOf(self)
	s.name = reflect.Indirect(s.self).Type().Name()
	s.typ = reflect.TypeOf(self)
	if !ast.IsExported(s.name) { // 判断结构体是否大写
		log.Fatalf("RPC Server: %s is not a valid service name", s.name)
	}
	s.registryMethods()
	return s
}

// 注册方法到service中
func (s *service) registryMethods() {
	s.method = make(map[string]*methodType)

	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type
		if mType.NumIn() != 3 || mType.NumOut() != 1 { // 限制3个入参，其中第一个参数是本身
			continue
		}
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() { // 判断返回值是否是error
			continue
		}
		argType, replyType := mType.In(1), mType.In(2)                                // 0是本身
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) { // 判断是否是内置类型
			continue
		}
		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("RPC server: register %s.%s\n", s.name, method.Name)
	}
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == "" // 内置类型会是空
}

// 执行方法
func (s *service) call(m *methodType, argv, reply reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	returnValues := f.Call([]reflect.Value{s.self, argv, reply})
	if err := returnValues[0].Interface(); err != nil { // 判断是否返回error
		return err.(error) // 类型转换
	}
	return nil
}
