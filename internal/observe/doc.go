// Package observe defines curated Run observability events, the Sink Agent
// adapters emit into, and the Run-owned Observer that writes Phase logs under
// .ship/runs, dumps a dense tools + in/out strip after each Phase, and prints
// a loud Abort banner with report paths and last tools on Phase failure.
package observe
