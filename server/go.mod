module go-rpc/server

go 1.19

require (
	go-rpc/codec v0.0.1
	go-rpc/service v0.0.1
)

replace (
	go-rpc/codec => ../codec
	go-rpc/service => ../service
)
