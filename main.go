package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	maxFileSize = 2 * 1024 * 1024 * 1024
)

var (
	timeout = flag.Duration("timeout", 3*time.Minute, "timeout")
)

func main() {
	if len(os.Args) != 4 {
		fmt.Println("Usage: replacer <search> <replace> <path>")
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, *timeout)
	defer cancel()

	search := os.Args[1]
	replace := os.Args[2]
	rootPath := os.Args[3]

	workers := runtime.GOMAXPROCS(0)
	largeFiles := make(chan string, workers)
	smallFiles := make(chan string, workers)
	errs := make([]error, 0)

	go func() {
		err := walk(ctx, rootPath, largeFiles, smallFiles, errs)
		if err != nil {
			fmt.Println(err)
		}
	}()

	wg := sync.WaitGroup{}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range smallFiles {
				log.Printf("Processing %s", path)
				select {
				case <-ctx.Done():
					errs = append(errs, ctx.Err())
					return
				default:
				}

				err := replaceInFile(path, search, replace)
				if err != nil {
					errs = append(errs, err)
				}
			}
		}()
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range largeFiles {
				log.Printf("Processing %s", path)
				select {
				case <-ctx.Done():
					errs = append(errs, ctx.Err())
					return
				default:
				}

				err := replaceInLargeFile(ctx, path, search, replace)
				if err != nil {
					errs = append(errs, err)
				}
			}
		}()
	}

	wg.Wait()

	for _, err := range errs {
		fmt.Println(err)
	}

	os.Exit(0)
}

// walk wraps walkDir and closes the channels when the walk is done.
func walk(
	ctx context.Context,
	path string,
	largeFiles,
	smallFiles chan string,
	errs []error,
) error {
	err := walkDir(ctx, path, largeFiles, smallFiles, errs)

	close(largeFiles)
	close(smallFiles)

	return err
}

// walkDir walks the directory tree rooted at path and sends the paths of large
// files to largeFiles and the paths of small files to smallFiles.
func walkDir(
	ctx context.Context,
	path string,
	largeFiles,
	smallFiles chan string,
	errs []error,
) error {
	initialPath := path

	return filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if path == initialPath {
			return nil
		}

		log.Printf("Walking %s", path)

		if err != nil {
			errs = append(errs, err)
			return nil
		}

		if info.IsDir() {
			err := walkDir(ctx, path, largeFiles, smallFiles, errs)
			if err != nil {
				errs = append(errs, err)
			}

			return nil
		}

		if info.Size() > maxFileSize {
			largeFiles <- path
			return nil
		}

		smallFiles <- path

		return nil
	})
}

func replaceInFile(path, search, replace string) error {
	input, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	defer input.Close()

	b, err := io.ReadAll(input)
	if err != nil {
		return err
	}

	output := strings.ReplaceAll(string(b), search, replace)

	return os.WriteFile(path, []byte(output), 0644)
}

func replaceInLargeFile(ctx context.Context, path, search, replace string) error {
	inputFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	tempFile, err := os.CreateTemp("", "replacer")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	writer := bufio.NewWriter(tempFile)
	scanner := bufio.NewScanner(inputFile)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		newLine := strings.ReplaceAll(line, search, replace)
		_, err := writer.WriteString(newLine + "\n")
		if err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	writer.Flush()
	tempFile.Close()
	inputFile.Close()

	return os.Rename(tempFile.Name(), path)
}
