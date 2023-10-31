package codec

import (
	"io"
)

// Header 定义请求与响应的Header结构体
type Header struct {
	ServiceMethod string // 服务名和方法名
	Seq           uint64 // 请求的序号，用来区别请求
	Error         string // 错误信息
}

// Codec 定义接口，规范client和server请求和响应的格式
type Codec interface {
	io.Closer                 // 定义关闭方法
	ReadHeader(*Header) error // 读取header
	ReadBody(any) error       // 读取body
	Write(*Header, any) error // 输出返回
}

type Type string
type NewCodecFunc func(io.ReadWriteCloser) Codec // 用来创建codec的类型，传入conn就能创建codec

const (
	JsonType Type = "application/json" // 使用json作为序列化方式
)

var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[JsonType] = NewJsonCodec // 基于json构造函数
}
