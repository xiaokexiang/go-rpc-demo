package codec

import "io"

type Header struct {
	ServiceMethod string // 服务名和方法名: Service.Method
	Seq           uint64 // 请求id 区分请求
	Error         string // 错误信息
}

// Codec 编解码接口，用于不同的协议实现
type Codec interface {
	io.Closer                 // 用于关闭连接
	ReadHeader(*Header) error // 读取请求头
	ReadBody(any) error       // 读取请求体
	Write(*Header, any) error // 输出响应头和响应体
}

type Func func(closer io.ReadWriteCloser) Codec // 定义一个新的函数类型: ReadWriteCloser ->Codec
type Type string

// 基于init() 内置2个默认的编解码
const (
	GobType  Type = "application/gob"  // gob是一种二进制编码格式 go进程间通信可以使用
	JsonType Type = "application/json" // 更适合跨网络传输
)

var FuncMap map[Type]Func

func init() {
	FuncMap = make(map[Type]Func)
	FuncMap[GobType] = NewGobCodec
	FuncMap[JsonType] = NewJsonCodec
}
