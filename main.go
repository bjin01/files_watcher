package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sergi/go-diff/diffmatchpatch"
	"gopkg.in/yaml.v3"
)

// Config structure matching YAML format
type Config struct {
	Directories []string `yaml:"directories"`
	LogFile     string   `yaml:"log_file"`
}

type synchronizedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *synchronizedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}

// FileContent stores the last known content of a file
type FileContent struct {
	content string
	removed bool
	mu      sync.RWMutex
}

// FileMonitor manages file watching and content tracking
type FileMonitor struct {
	watcher    *fsnotify.Watcher
	files      map[string]*FileContent
	filesMu    sync.RWMutex
	extensions []string // ASCII file extensions to monitor
	output     io.Writer
}

func NewFileMonitor() (*FileMonitor, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &FileMonitor{
		watcher: watcher,
		files:   make(map[string]*FileContent),
		extensions: []string{
			".txt", ".log", ".yaml", ".yml", ".json",
			".xml", ".csv", ".md", ".rst", ".conf", ".cnf",
			".cfg", ".ini", ".sh", ".bash", ".py",
			".go", ".c", ".cpp", ".h", ".hpp",
			".java", ".js", ".ts", ".rb", ".php",
			".html", ".css", ".sql", ".toml",
		},
	}, nil
}

func openLogFile(path string) (*os.File, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("log_file must be specified in the config")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", path, err)
	}
	return file, nil
}

func (fm *FileMonitor) outputWriter() io.Writer {
	if fm.output != nil {
		return fm.output
	}
	return os.Stdout
}

// isASCIIFile checks if a file is likely an ASCII text file
func (fm *FileMonitor) isASCIIFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range fm.extensions {
		if ext == e {
			return true
		}
	}
	return false
}

// readFileContent reads file content safely
func (fm *FileMonitor) readFileContent(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// computeDiff generates a line-based diff with visible change markers.
func (fm *FileMonitor) computeDiff(old, new, filename string) string {
	dmp := diffmatchpatch.New()
	oldLines, newLines, lineArray := dmp.DiffLinesToChars(old, new)
	diffs := dmp.DiffMain(oldLines, newLines, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	var output strings.Builder
	fmt.Fprintf(&output, "--- %s\n+++ %s\n", filename, filename)
	for _, diff := range diffs {
		prefix := " "
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			prefix = "+"
		case diffmatchpatch.DiffDelete:
			prefix = "-"
		}

		lines := strings.SplitAfter(diff.Text, "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			output.WriteString(prefix)
			output.WriteString(line)
			if !strings.HasSuffix(line, "\n") {
				output.WriteByte('\n')
			}
		}
	}
	return output.String()
}

// watchDirectory recursively adds directories to the watcher
func (fm *FileMonitor) watchDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			err = fm.watcher.Add(path)
			if err != nil {
				log.Printf("Warning: cannot watch %s: %v", path, err)
			} else {
				log.Printf("Watching directory: %s", path)
			}
		}
		return nil
	})
}

// initializeFiles reads initial content of all monitored files
func (fm *FileMonitor) initializeFiles(dirs []string) error {
	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && fm.isASCIIFile(path) {
				content, err := fm.readFileContent(path)
				if err != nil {
					log.Printf("Warning: cannot read %s: %v", path, err)
					return nil
				}
				fm.filesMu.Lock()
				fm.files[path] = &FileContent{content: content}
				fm.filesMu.Unlock()
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// handleEvent processes file system events
func (fm *FileMonitor) handleEvent(event fsnotify.Event) {
	// Skip directories
	if filepath.Ext(event.Name) == "" {
		return
	}

	// Skip non-ASCII files
	if !fm.isASCIIFile(event.Name) {
		return
	}

	switch {
	case event.Op&fsnotify.Write == fsnotify.Write:
		fm.processWrite(event.Name)
	case event.Op&fsnotify.Create == fsnotify.Create:
		fm.processCreate(event.Name)
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		fm.processRemove(event.Name)
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		fm.processRemove(event.Name) // Treat rename as removal
	}
}

// processWrite handles file modification events
func (fm *FileMonitor) processWrite(path string) {
	// Small delay to ensure file write is complete
	time.Sleep(50 * time.Millisecond)

	newContent, err := fm.readFileContent(path)
	if err != nil {
		log.Printf("Error reading %s: %v", path, err)
		return
	}

	fm.filesMu.Lock()
	existing, exists := fm.files[path]
	if exists {
		existing.mu.Lock()
		oldContent := existing.content
		existing.content = newContent
		existing.removed = false
		existing.mu.Unlock()
		fm.filesMu.Unlock()

		if newContent != oldContent {
			fmt.Fprintf(fm.outputWriter(), "\n=== File modified: %s ===\n", path)
			fmt.Fprintln(fm.outputWriter(), fm.computeDiff(oldContent, newContent, path))
			fmt.Fprintln(fm.outputWriter())
		}
	} else {
		// File wasn't tracked before (maybe created while we weren't watching)
		fm.files[path] = &FileContent{content: newContent}
		fm.filesMu.Unlock()
		fmt.Fprintf(fm.outputWriter(), "New file detected: %s\n", path)
	}
}

// processCreate handles new file creation
func (fm *FileMonitor) processCreate(path string) {
	// Small delay to ensure file is fully written
	time.Sleep(50 * time.Millisecond)

	content, err := fm.readFileContent(path)
	if err != nil {
		log.Printf("Error reading new file %s: %v", path, err)
		return
	}

	fm.filesMu.Lock()
	existing, exists := fm.files[path]

	if exists {
		existing.mu.Lock()
		oldContent := existing.content
		wasRemoved := existing.removed
		existing.removed = false
		existing.content = content
		existing.mu.Unlock()
		fm.filesMu.Unlock()

		if oldContent != content {
			fmt.Fprintf(fm.outputWriter(), "\n=== File modified: %s ===\n", path)
			fmt.Fprint(fm.outputWriter(), fm.computeDiff(oldContent, content, path))
			fmt.Fprintln(fm.outputWriter())
		} else if wasRemoved {
			fmt.Fprintf(fm.outputWriter(), "\n=== File created: %s ===\n", path)
			fmt.Fprintf(fm.outputWriter(), "Size: %d bytes\n\n", len(content))
		}
		return
	}

	fm.files[path] = &FileContent{content: content}
	fm.filesMu.Unlock()

	fmt.Fprintf(fm.outputWriter(), "\n=== File created: %s ===\n", path)
	fmt.Fprintf(fm.outputWriter(), "Size: %d bytes\n\n", len(content))
}

// processRemove handles file deletion
func (fm *FileMonitor) processRemove(path string) {
	fm.filesMu.Lock()
	existing := fm.files[path]

	if existing != nil {
		existing.mu.Lock()
		existing.removed = true
		existing.mu.Unlock()
	}
	fm.filesMu.Unlock()

	if existing != nil {
		time.AfterFunc(time.Second, func() {
			fm.filesMu.Lock()
			removed := false
			if fm.files[path] == existing {
				existing.mu.RLock()
				removed = existing.removed
				existing.mu.RUnlock()
				if removed {
					delete(fm.files, path)
				}
			}
			fm.filesMu.Unlock()

			if removed {
				fmt.Fprintf(fm.outputWriter(), "\n=== File removed: %s ===\n\n", path)
			}
		})
		return
	}

	fmt.Fprintf(fm.outputWriter(), "\n=== File removed: %s ===\n\n", path)
}

// Run starts the file monitoring loop
func (fm *FileMonitor) Run() error {
	defer fm.watcher.Close()

	done := make(chan bool)

	// Event handler goroutine
	go func() {
		for {
			select {
			case event, ok := <-fm.watcher.Events:
				if !ok {
					return
				}
				fm.handleEvent(event)
			case err, ok := <-fm.watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()

	<-done
	return nil
}

// loadConfig reads and parses the YAML configuration file
func loadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <config.yaml>\n", os.Args[0])
		fmt.Println("\nExample config.yaml:")
		fmt.Println("directories:")
		fmt.Println("  - /path/to/watch1")
		fmt.Println("  - /path/to/watch2")
		fmt.Println("  - ./relative/path")
		os.Exit(1)
	}

	configFile := os.Args[1]

	// Load configuration
	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	if len(config.Directories) == 0 {
		log.Fatal("No directories specified in config")
	}

	logFile, err := openLogFile(config.LogFile)
	if err != nil {
		log.Fatalf("Error setting up log file: %v", err)
	}
	defer logFile.Close()

	logFileWriter := &synchronizedWriter{writer: logFile}
	log.SetOutput(io.MultiWriter(os.Stderr, logFileWriter))
	output := io.MultiWriter(os.Stdout, logFileWriter)

	// Create file monitor
	monitor, err := NewFileMonitor()
	if err != nil {
		log.Fatalf("Error creating monitor: %v", err)
	}
	monitor.output = output

	// Initialize file content
	err = monitor.initializeFiles(config.Directories)
	if err != nil {
		log.Fatalf("Error initializing files: %v", err)
	}

	// Watch all directories recursively
	for _, dir := range config.Directories {
		err = monitor.watchDirectory(dir)
		if err != nil {
			log.Printf("Warning: error watching %s: %v", dir, err)
		}
	}

	fmt.Fprintf(output, "Monitoring %d directories for ASCII file changes...\n", len(config.Directories))
	fmt.Fprintln(output, "Press Ctrl+C to stop")

	// Start monitoring (blocks forever)
	monitor.Run()
}
