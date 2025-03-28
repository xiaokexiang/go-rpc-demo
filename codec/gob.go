package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer // 带缓冲的写入器，减少系统io调用次数
	dec  *gob.Decoder  // gob反序列化
	enc  *gob.Encoder  // gob序列化
}

func (gc *GobCodec) ReadHeader(h *Header) error {
	return gc.dec.Decode(h)
}

func (gc *GobCodec) ReadBody(body any) error {
	return gc.dec.Decode(body)
}

/*
将headers和body写入到输出流
*/
func (gc *GobCodec) Write(h *Header, body any) (err error) {
	defer func() {
		e := gc.buf.Flush() // 真正将数据写入底层
		if err == nil && e != nil {
			err = e // 只有在前面的操作没出错时，才返回 Flush 的错误
		}
		if err != nil { // return执行先于defer, 如果有异常就执行close()
			_ = gc.Close()
		}
	}()
	if err := gc.enc.Encode(h); err != nil {
		log.Println("RPC[GobCodec] Write Encode Header err: ", err)
		return err
	}
	if err := gc.enc.Encode(body); err != nil {
		log.Println("RPC[GobCodec] Write Encode Body err: ", err)
		return err
	}
	return nil
}

func (gc *GobCodec) Close() error {
	return gc.conn.Close()
}

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn) // 创建带缓冲的写入器
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn), // 对请求连接进行解码
		enc:  gob.NewEncoder(buf),  // 对输出流进行编码
	}
}
