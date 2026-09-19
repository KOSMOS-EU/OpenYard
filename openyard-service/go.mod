module github.com/kosmos-eu/openyard

go 1.23.8

require (
	github.com/cs3org/go-cs3apis v0.0.0-20250908152307-4ca807afe54e
	github.com/go-chi/chi/v5 v5.2.1
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/rs/zerolog v1.33.0
	google.golang.org/grpc v1.68.0
)

require (
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go-micro.dev/v4 => github.com/butonic/go-micro/v4 v4.11.1-0.20241115112658-b5d4de5ed9b3
