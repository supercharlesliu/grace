package fork

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/zyxar/grace/sigutil"
)

func ExampleGetArgs() {
	args := GetArgs(nil, func(arg string) bool {
		return arg == "daemon"
	})
	program, _ := os.Executable() // go1.8+
	if _, err := Daemonize(program, &Option{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
	}, args...); err != nil {
		panic(err)
	}
}

func ExampleDaemonize() {
	getArgs := func() []string {
		if flag.NFlag() == 0 {
			return flag.Args()
		}
		flag.Set("daemon", "false")
		args := make([]string, 0, flag.NFlag()+flag.NArg())
		flag.Visit(func(f *flag.Flag) {
			args = append(args, "-"+f.Name+"="+f.Value.String())
		})
		return append(args, flag.Args()...)
	}

	var daemon = flag.Bool("daemon", false, "enable daemon mode")
	flag.Parse()
	if *daemon {
		stderr, err := os.OpenFile("stderr.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		defer stderr.Close()
		program, _ := os.Executable() // go1.8+
		if pid, err := Daemonize(program, &Option{Stderr: stderr}, getArgs()...); err != nil {
			panic(err)
		} else {
			ioutil.WriteFile("example.pid", []byte(strconv.Itoa(pid)+"\n"), 0644)
		}
		return
	}

	defer os.Stderr.Close()

	sigutil.Trap(func(s sigutil.Signal) {
		log.Println("[KILLED] by signal", s)
	}, sigutil.SIGINT, sigutil.SIGTERM)
}

func ExampleListen() {
	ln, err := Listen("tcp", "127.0.0.1:12345")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer ln.Close()
	srv := &http.Server{Addr: ln.Addr().String()}
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%v\n", os.Getpid())
	})
	go func() { log.Fatal(srv.Serve(ln)) }()

	sigutil.Trap(func(s sigutil.Signal) {
		log.Println("[KILLED] by signal", s)
	}, sigutil.SIGINT, sigutil.SIGTERM)
}

func ExampleListenPacket() {
	conn, err := ListenPacket("udp4", "127.0.0.1:12345")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer conn.Close()
	done := make(chan struct{})
	go func() {
		buffer := make([]byte, 64*1024)
		for {
			select {
			case <-done:
				return
			default:
			}
			_, err = io.CopyBuffer(os.Stdout, conn.(*net.UDPConn), buffer)
			if err != nil {
				select {
				case <-done:
					return
				default:
				}
				log.Println(err)
				continue
			}
		}
	}()

	sigutil.Trap(func(s sigutil.Signal) {
		close(done)
		log.Println("[KILLED] by signal", s)
	}, sigutil.SIGINT, sigutil.SIGTERM)
}

func ExampleReload() {
	ln, err := Listen("tcp", "127.0.0.1:12345")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer ln.Close()

	sigutil.Trap(func(s sigutil.Signal) {
		switch s {
		case sigutil.SIGHUP:
			pid, err := Reload(ln)
			if err != nil {
				log.Println(err)
				return
			}
			log.Printf("[RELOAD] %d -> %d", os.Getpid(), pid)
		default:
			log.Println("[KILLED] by signal", s)
		}
	}, sigutil.SIGHUP, sigutil.SIGINT, sigutil.SIGTERM)
}

func ExampleReload_multiple() {
	ln, err := Listen("tcp", "127.0.0.1:12345")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer ln.Close()

	ln1, err := Listen("tcp", "127.0.0.1:12346")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer ln1.Close()

	ln2, err := Listen("tcp", "127.0.0.1:12347")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer ln2.Close()

	conn, err := ListenPacket("udp", "127.0.0.1:12345")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	srv := &http.Server{Addr: ln.Addr().String()}
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "1:%v\n", os.Getpid())
	})
	go func() { log.Fatal(srv.Serve(ln)) }()

	srv1 := &http.Server{Addr: ln1.Addr().String()}
	srv1.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "2:%v\n", os.Getpid())
	})
	go func() { log.Fatal(srv1.Serve(ln1)) }()

	srv2 := &http.Server{Addr: ln2.Addr().String()}
	srv2.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "3:%v\n", os.Getpid())
	})
	go func() { log.Fatal(srv2.Serve(ln2)) }()

	sigutil.Trap(func(s sigutil.Signal) {
		switch s {
		case sigutil.SIGHUP:
			pid, err := Reload(ln, ln1, ln2, conn)
			if err != nil {
				log.Println(err)
				return
			}
			log.Printf("[RELOAD] %d -> %d", os.Getpid(), pid)
		default:
			log.Println("[KILLED] by signal", s)
		}
	}, sigutil.SIGHUP, sigutil.SIGINT, sigutil.SIGTERM)
}
