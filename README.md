
# files_watcher - A simple file watcher for Linux
This is a simple file watcher for Linux that monitors specified directories for config file changes and logs the events and changes to a log file. It is implemented in Go and can be run as a systemd service.

Supported file extensions:
```
".txt", ".log", ".yaml", ".yml", ".json",
".xml", ".csv", ".md", ".rst", ".conf", ".cnf",
".cfg", ".ini", ".sh", ".bash", ".py",
".go", ".c", ".cpp", ".h", ".hpp",
".java", ".js", ".ts", ".rb", ".php",
".html", ".css", ".sql", ".toml"
```

__config.yaml__ - Configuration file for files_watcher

```
# /etc/files_watcher/config.yaml
directories:
  - /path/to/watch
log_file: /var/log/files_watcher/files_watcher.log
```
In the config.yaml file, you can specify the directories to watch and the log file location. The directories section is a list of directories that files_watcher will monitor for changes. The log_file section specifies the path to the log file where files_watcher will write its logs.

__files_watcher.service__ - Systemd service file for files_watcher

## Install as a systemd service

Download the latest release from the [releases page](https://github.com/bjin01/files_watcher/releases).
Extract the tarball:

```
tar -xzf files_watcher-<version>-linux-amd64.tar.gz
```

Copy `config.example.yaml` to `/etc/files_watcher/config.yaml` and set the
directories to monitor and a writable `log_file` path. The installer will not
overwrite an existing config file.

Run the combined installer as root; it builds and installs the statically
linked Linux binary, installs the systemd unit, and enables and starts the
service:

```sh
sudo ./install_binary.sh
sudo ./install_service.sh
```

The installer builds the binary if Go is available, otherwise it uses the
packaged static Linux binary.

Check service status and logs with:

```sh
sudo systemctl status files_watcher.service
sudo journalctl -u files_watcher.service
```

## Compile
To build the files_watcher binary for Linux, run the following command:

```
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o files_watcher . && file files_watcher && (ldd files_watcher || true)
```

