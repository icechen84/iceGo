package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func main() {
	g, ctx := errgroup.WithContext(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	// 模拟单个服务错误退出
	serverOut := make(chan struct{})
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		serverOut <- struct{}{}
	})

	server := http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	// g1
	// g1 退出了所有的协程都能退出么？
	// g1 退出后, context 将不再阻塞，g2, g3 都会随之退出
	// 然后 main 函数中的 g.Wait() 退出，所有协程都会退出
	g.Go(func() error {
		return server.ListenAndServe()
	})

	// g2
	// g2 退出了所有的协程都能退出么？
	// g2 退出时，调用了 shutdown，g1 会退出
	// g2 退出后, context 将不再阻塞，g3 会随之退出
	// 然后 main 函数中的 g.Wait() 退出，所有协程都会退出
	g.Go(func() error {
		select {
		case <-ctx.Done():
			log.Println("errgroup exit...")
		case <-serverOut:
			log.Println("server will out...")
		}

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		// 这里不是必须的，但是如果使用 _ 的话静态扫描工具会报错，加上也无伤大雅
		defer cancel()

		log.Println("shutting down server...")
		return server.Shutdown(timeoutCtx)
	})

	// g3
	// g3 捕获到 os 退出信号将会退出
	// g3 退出了所有的协程都能退出么？
	// g3 退出后, context 将不再阻塞，g2 会随之退出
	// g2 退出时，调用了 shutdown，g1 会退出
	// 然后 main 函数中的 g.Wait() 退出，所有协程都会退出
	g.Go(func() error {
		quit := make(chan os.Signal, 0)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig := <-quit:
			return errors.Errorf("get os signal: %v", sig)
		}
	})

	fmt.Printf("errgroup exiting: %+v\n", g.Wait())
}

/*
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

type helloHandler struct {
	ctx  context.Context
	name string
}

func (h *helloHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Write([]byte(fmt.Sprintf("Hello from %s", h.name)))
}

func newHelloServer(
	ctx context.Context,
	name string,
	port int,
) *http.Server {

	mux := http.NewServeMux()
	handler := &helloHandler{ctx: ctx, name: name}
	mux.Handle("/", handler)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return httpServer
}

func main() {
	// HERE
	// setup context and signal handling
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	g, ctx := errgroup.WithContext(ctx)

	// start servers
	server1 := newHelloServer(ctx, "server1", 8080)
	g.Go(func() error {
		log.Println("server 1 listening on port 8080")
		if err := server1.ListenAndServe();
			err != http.ErrServerClosed {
			return err
		}

		return nil
	})

	server2 := newHelloServer(ctx, "server2", 8081)
	g.Go(func() error {
		log.Println("server 2 listening on port 8081")
		if err := server2.ListenAndServe();
			err != http.ErrServerClosed {
			return err
		}

		return nil
	})

	// handle termination
	select {
	case <-quit:
		break
	case <-ctx.Done():
		break
	}

	// gracefully shutdown http servers
	cancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer timeoutCancel()

	log.Println("shutting down servers, please wait...")

	server1.Shutdown(timeoutCtx)
	server2.Shutdown(timeoutCtx)

	// AND THIS
	// wait for shutdown
	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}

	log.Println("a graceful bye")
}
*/
