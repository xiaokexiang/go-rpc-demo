package codec

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
)

// JsonCodec 作为codec的实现结构和实现方法
type JsonCodec struct {
	conn io.ReadWriteCloser // 连接实例
	buf  *bufio.Writer      // 防止阻塞而创建的带缓冲的writer
	dec  *json.Decoder      // 解码
	enc  *json.Encoder      // 编码
}

func (c *JsonCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}
func (c *JsonCodec) ReadBody(body any) error {
	return c.dec.Decode(body)
}
func (c *JsonCodec) Write(h *Header, body any) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()
	if err := c.enc.Encode(h); err != nil {
		log.Println("rpc code -> json encoding header error: ", err)
		return err
	}
	if err := c.enc.Encode(body); err != nil {
		log.Println("rpc code -> json encoding body error: ", err)
		return err
	}
	return nil

}
func (c *JsonCodec) Close() error {
	return c.conn.Close()
}

// NewJsonCodec jsonCodeC的构造函数
func NewJsonCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &JsonCodec{
		conn: conn,                  // 连接
		buf:  buf,                   // 缓冲区满后才会写入到内核
		dec:  json.NewDecoder(conn), // 从conn中读取
		enc:  json.NewEncoder(buf),  // 会输出到buf中
	}
}
