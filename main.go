package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"bufio"

	"boot.dev/linko/internal/store"
)


func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	httpPort := flag.Int("port", 8899, "port to listen on")
	dataDir := flag.String("data", "./data", "directory to store data")
	flag.Parse()

	status := run(ctx, cancel, *httpPort, *dataDir)
	cancel()
	os.Exit(status)
}

func run(ctx context.Context, cancel context.CancelFunc, httpPort int, dataDir string) int {
	logFile := os.Getenv("LINKO_LOG_FILE")
	logger, closer, err := initializeLogger(logFile)
	defer func() {
		if err := closer(); err != nil{
			fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
		}
	}()

	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		return 1
	}
	st, err := store.New(dataDir, logger)
	if err != nil {
		logger.Printf( "failed to create store: %v\n", err)
		return 1
	}

	s := newServer(logger, *st, httpPort, cancel)
	var serverErr error
	go func() {
		serverErr = s.start()
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.shutdown(shutdownCtx); err != nil {
		logger.Printf("failed to shutdown server: %v\n", err)
		return 1
	}
	if serverErr != nil {
		logger.Printf("server error: %v\n", serverErr)
		return 1
	}
	return 0
}



type closeFunc func() error

func initializeLogger(logFile string) (*log.Logger, closeFunc, error){
	if logFile != ""{
		file, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil , fmt.Errorf("Problem with opening file: %v", err)
		}
		bufferedFile := bufio.NewWriterSize(file, 8192)
		multiW := io.MultiWriter(os.Stderr, bufferedFile)

		closer := func() error {
			err := bufferedFile.Flush()
			if err != nil{
				return fmt.Errorf("problem flushing buffer: %v\n", err)
			}
			err = file.Close()
			if err != nil{
				return fmt.Errorf("problem closing file: %v\n", err)
			}
			return nil
		}
		return log.New(multiW, "", log.LstdFlags),closer, nil
	}
	return log.New(os.Stderr, "", log.LstdFlags), func() error {return nil}, nil
}



