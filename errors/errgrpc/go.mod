// errgrpc is a module of its own so that gRPC stays out of toerr's dependency graph: a
// service that never speaks gRPC should not find grpc in its go.sum or its SBOM.
module github.com/StevenACoffman/toerr/errors/errgrpc

go 1.26.0

// Until toerr v0.3.0 is tagged, build against the working tree. Consumers ignore a replace
// in a module they depend on, so this line is harmless once that tag exists.
replace github.com/StevenACoffman/toerr => ../..

require (
	github.com/StevenACoffman/toerr v0.3.0
	google.golang.org/grpc v1.84.0
)

require (
	golang.org/x/sys v0.47.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
