// Package errhttp is the HTTP adapter for errcode: at the transport boundary it
// maps the status code attached to an error to an HTTP status and a client-safe
// message. HTTP concerns live here, not in the domain errcode package.
package errhttp
