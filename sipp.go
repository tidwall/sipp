package sipp

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

// Plugin represents an interprocess plugin
type Plugin interface {
	// Send the plugin some input
	Send(input []byte) Response
}

// Response is the response from Send
type Response interface {
	Output() []byte
}

// Open a plugin
func Open(path string) (Plugin, error) {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path, "--sipp-plugin")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go io.Copy(os.Stderr, stderr)
	ch := make(chan *request, 1024)
	go func() (perr error) {
		defer func() {
			if perr != nil {
				panic(perr)
			}
			go func() {
				stdin.Close()
				cmd.Wait()
			}()
		}()
		rd := bufio.NewReader(stdout)
		var dst []byte
		var num [10]byte
		var reqs []*request
		for {
			reqs = append(reqs[:0], <-ch)
			var done bool
			for !done {
				select {
				case req := <-ch:
					reqs = append(reqs, req)
				default:
					done = true
				}
			}
			dst = dst[:0]
			for _, req := range reqs {
				if req == nil {
					return
				}
				n := binary.PutUvarint(num[:], uint64(len(req.input)))
				dst = append(dst, num[:n]...)
				dst = append(dst, req.input...)
				if len(dst) > 0xFFFF {
					stdin.Write(dst)
					dst = dst[:0]
				}
			}
			stdin.Write(dst)
			for _, req := range reqs {
				sz, err := binary.ReadUvarint(rd)
				if err != nil {
					if err == io.EOF {

					}
					return err
				}
				req.output = make([]byte, sz)
				if _, err := io.ReadFull(rd, req.output); err != nil {
					return err
				}
				req.wg.Done()
			}
			if cap(dst) > 0xFFFFF {
				dst = nil
			}
		}
	}()
	p := &plugin{ch: ch}
	runtime.SetFinalizer(p, func(p *plugin) { ch <- nil })
	return p, nil
}

type request struct {
	input  []byte
	output []byte
	err    error
	wg     sync.WaitGroup
}

func (r *request) Output() []byte {
	r.wg.Wait()
	return r.output
}

type plugin struct {
	ch chan *request
}

func (p *plugin) Send(input []byte) Response {
	req := &request{input: input}
	req.wg.Add(1)
	p.ch <- req
	return req
}

var stdout *os.File
var isPlugin bool

func init() {
	stdout = os.Stdout
	for _, arg := range os.Args {
		if arg == "--sipp-plugin" {
			os.Stdout = os.Stderr
			isPlugin = true
			return
		}
	}
}

// Handle inputs from programs. This must be from the plugin main() function.
func Handle(fn func(input []byte) []byte) {
	if !isPlugin {
		panic("not a plugin")
	}
	rd := bufio.NewReader(os.Stdin)
	var dst []byte
	for {
		sz, err := binary.ReadUvarint(rd)
		if err != nil {
			if err == io.EOF {
				return
			}
			panic(err)
		}
		input := make([]byte, sz)
		if _, err := io.ReadFull(rd, input); err != nil {
			panic(err)
		}
		var output []byte
		if fn != nil {
			output = fn(input)
		}
		var num [10]byte
		n := binary.PutUvarint(num[:], uint64(len(output)))
		dst = append(dst, num[:n]...)
		dst = append(dst, output...)
		if len(dst) > 0xFFFF {
			stdout.Write(dst)
			dst = dst[:0]
		}
		if rd.Buffered() == 0 {
			if len(dst) > 0 {
				stdout.Write(dst)
				if len(dst) > 0xFFFFF {
					dst = nil
				} else {
					dst = dst[:0]
				}
			}
		}
	}
}
