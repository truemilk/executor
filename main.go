package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	executionCount int32
	totalTasks    int32
)

func main() {
	command := flag.String("cmd", "", "Command to execute")
	workers := flag.Int("workers", 4, "Number of concurrent workers")
	pattern := flag.String("pattern", "", "Path pattern (e.g., '*/src' or '**.go')")
	dirsOnly := flag.Bool("dirs-only", false, "Only process directories")
	filesOnly := flag.Bool("files-only", false, "Only process files")
	flag.Parse()

	if *command == "" {
		fmt.Println("Please provide a command using -cmd flag")
		os.Exit(1)
	}

	if *pattern == "" {
		fmt.Println("Please provide a path pattern using -pattern flag")
		os.Exit(1)
	}

	if *dirsOnly && *filesOnly {
		fmt.Println("Cannot specify both -dirs-only and -files-only")
		os.Exit(1)
	}

	// Find matching paths
	matches, err := filepath.Glob(*pattern)
	if err != nil {
		fmt.Printf("Error with pattern matching: %v\n", err)
		os.Exit(1)
	}

	// Add tilde expansion before glob matching
	if strings.HasPrefix(*pattern, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Printf("Error getting home directory: %v\n", err)
			os.Exit(1)
		}
		*pattern = filepath.Join(homeDir, strings.TrimPrefix(*pattern, "~"))
		matches, err = filepath.Glob(*pattern)
		if err != nil {
			fmt.Printf("Error with pattern matching: %v\n", err)
			os.Exit(1)
		}
	}

	if len(matches) == 0 {
		fmt.Printf("No matches found for pattern: %s\n", *pattern)
		os.Exit(1)
	}

	// Filter paths based on flags
	var targets []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			fmt.Printf("Warning: Cannot stat %s: %v\n", match, err)
			continue
		}

		isDir := info.IsDir()
		if (*dirsOnly && isDir) || (*filesOnly && !isDir) || (!*dirsOnly && !*filesOnly) {
			targets = append(targets, match)
		}
	}

	if len(targets) == 0 {
		fmt.Println("No matching targets found after filtering")
		os.Exit(1)
	}

	// Set total tasks before creating workers
	atomic.StoreInt32(&totalTasks, int32(len(targets)))
	fmt.Printf("Found %d targets to process\n", len(targets))

	// Create a channel for tasks
	tasks := make(chan string, len(targets))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go worker(i, tasks, &wg, *command)
	}

	// Send tasks to workers
	for _, target := range targets {
		tasks <- target
	}
	close(tasks)

	// Wait for all workers to complete
	wg.Wait()

	// Print final summary
	fmt.Printf("\nExecution Summary: Completed %d operations\n", executionCount)
}

func worker(id int, tasks <-chan string, wg *sync.WaitGroup, command string) {
	defer wg.Done()

	for target := range tasks {
		fmt.Printf("Worker %d: Processing %s\n", id, target)
		
		info, err := os.Stat(target)
		if err != nil {
			fmt.Printf("Error: Cannot stat %s: %v\n", target, err)
			continue
		}

		// Replace placeholder with target path
		cmdStr := strings.ReplaceAll(command, "{}", target)
		
		// Create command using sh
		cmd := exec.Command("/bin/sh", "-c", cmdStr)
		
		// If target is a directory, set working directory
		// If target is a file, set working directory to its parent
		if info.IsDir() {
			cmd.Dir = target
		} else {
			cmd.Dir = filepath.Dir(target)
		}
		
		// Get combined output
		output, err := cmd.CombinedOutput()
		
		// Replace the mutex-based counter with atomic operation
		current := atomic.AddInt32(&executionCount, 1)
		
		// Print simple progress counter
		fmt.Printf("\rProgress: [%d/%d]", current, totalTasks)
		
		if len(output) > 0 {
			fmt.Printf("\nOutput: %s\n", strings.TrimSpace(string(output)))
		}
		if err != nil {
			fmt.Printf("\nError: %v\n", err)
		}
		fmt.Println(strings.Repeat("-", 40))
	}
}