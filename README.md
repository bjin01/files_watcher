
files_watcher - A simple file watcher for Linux

config.yaml - Configuration file for files_watcher
In the config.yaml file, you can specify the directories to watch and the log file location. The directories section is a list of directories that files_watcher will monitor for changes. The log_file section specifies the path to the log file where files_watcher will write its logs.

files_watcher.service - Systemd service file for files_watcher


To build the files_watcher binary for Linux, run the following command:

```
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o files_watcher . && file files_watcher && (ldd files_watcher || true)
```