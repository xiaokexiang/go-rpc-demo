module main

go 1.19

require (
	go-rpc/codec v0.0.1
	go-rpc/server v0.0.1
)

replace (
	go-rpc/codec => ./codec
	go-rpc/server => ./server
)