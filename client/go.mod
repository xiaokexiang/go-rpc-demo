module client

go 1.19

replace (
	go-rpc/codec => ../codec
	go-rpc/server => ../server
	go-rpc/service => ../service
	go-rpc/registry => ../registry
)

require (
	go-rpc/codec v0.0.1
	go-rpc/server v0.0.1
	go-rpc/service v0.0.1
	go-rpc/registry v0.0.1
)
