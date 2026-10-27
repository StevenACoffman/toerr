// Package errgrpc is the gRPC adapter for errcode. Outbound, at a server's
// transport boundary, Error turns the status code attached to an error into a gRPC
// status error with a client-safe message. Inbound, FromError turns an error a gRPC
// client call returned into the domain code of the operation that made the call, and
// Attrs keeps the downstream status's fields for operators. gRPC concerns live here,
// not in the domain errcode package.
//
// errgrpc is a module of its own, so only a program that imports it depends on gRPC.
package errgrpc
