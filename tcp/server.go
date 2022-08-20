package tcp

/**
 * A tcp server
 */

import (
	"context"
	"fmt"
	"github.com/hdt3213/godis/interface/tcp"
	"github.com/hdt3213/godis/lib/logger"
	reuse "github.com/libp2p/go-reuseport"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// Config stores tcp server properties
type Config struct {
	Address    string        `yaml:"address"`
	MaxConnect uint32        `yaml:"max-connect"`
	Timeout    time.Duration `yaml:"timeout"`
	ReusePort  bool
}

// ListenAndServeWithSignal binds port and handle requests, blocking until receive stop signal
func ListenAndServeWithSignal(cfg *Config, handler tcp.Handler) error {
	closeChan := make(chan struct{})
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		switch sig {
		case syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			close(closeChan)
		}
	}()
	if cfg.ReusePort {
		size := runtime.NumCPU()
		wg := &sync.WaitGroup{}
		wg.Add(size)
		for i := 0; i < size; i++ {
			go func() {
				defer wg.Done()
				listener, err := reuse.Listen("tcp", cfg.Address)
				if err != nil {
					logger.Error("%v", err)
					return
				}
				logger.Info(fmt.Sprintf("bind: %s, start listening...", cfg.Address))
				ListenAndServe(listener, handler, closeChan)
			}()
		}
		wg.Wait()
	} else {
		listener, err := net.Listen("tcp", cfg.Address)
		if err != nil {
			return err
		}
		//cfg.Address = listener.Addr().String()
		logger.Info(fmt.Sprintf("bind: %s, start listening...", cfg.Address))
		// todo: use context instead of closeChan
		ListenAndServe(listener, handler, closeChan)
	}
	return nil
}

// ListenAndServe binds port and handle requests, blocking until close
func ListenAndServe(listener net.Listener, handler tcp.Handler, closeChan <-chan struct{}) {
	// listen signal
	go func() {
		<-closeChan
		logger.Info("shutting down...")
		_ = listener.Close() // listener.Accept() will return err immediately
		_ = handler.Close()  // close connections
	}()

	// listen port
	defer func() {
		// close during unexpected error
		_ = listener.Close()
		_ = handler.Close()
	}()
	ctx := context.Background()
	var waitDone sync.WaitGroup
	for {
		conn, err := listener.Accept()
		if err != nil {
			break
		}
		// handle
		logger.Info("accept link")
		waitDone.Add(1)
		go func() {
			defer func() {
				waitDone.Done()
			}()
			handler.Handle(ctx, conn)
		}()
	}
	waitDone.Wait()
}
