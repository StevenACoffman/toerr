// errgrpc is a module of its own so that gRPC stays out of toerr's dependency graph: a
// service that never speaks gRPC should not find grpc in its go.sum or its SBOM.
module github.com/StevenACoffman/toerr/errors/errgrpc

go 1.26.0

// This sets the require github.com/StevenACoffman/toerr to
// the version AFTER last
// Until that NEXT version of toerr is tagged, this will build against
// the working tree.
//
// Consumers ignore a replace in a module they depend on,
// so the replace line is harmless once that tag exists.
replace github.com/StevenACoffman/toerr => ../..

require (
	github.com/StevenACoffman/toerr v0.4.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
)

require golang.org/x/sys v0.47.0 // indirect
