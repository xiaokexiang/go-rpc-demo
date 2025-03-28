package codec

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
)

type JsonCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
	dec  *json.Decoder
	enc  *json.Encoder
}

func (jc *JsonCodec) ReadHeader(h *Header) error {
	return jc.dec.Decode(h)
}

func (jc *JsonCodec) ReadBody(body any) error {
	return jc.dec.Decode(body)
}

func (jc *JsonCodec) Write(h *Header, body any) (err error) {
	defer func() {
		e := jc.buf.Flush()
		if err == nil && e != nil {
			err = e
		}
		if err != nil {
			_ = jc.Close()
		}
	}()
	if err := jc.enc.Encode(h); err != nil {
		log.Println("RPC[JsonCodec] Write Encode Header err: ", err)
		return err
	}

	if err := jc.enc.Encode(body); err != nil {
		log.Println("RPC[JsonCodec]Write Encode Body err: ", err)
		return err
	}
	return nil
}

func (jc *JsonCodec) Close() error {
	return jc.conn.Close()
}

func NewJsonCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &JsonCodec{
		conn: conn,
		buf:  buf,
		dec:  json.NewDecoder(conn),
		enc:  json.NewEncoder(buf),
	}
}
