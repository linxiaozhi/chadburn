package cli

import (
	"github.com/PremoWeb/Chadburn/core"
	"github.com/fsnotify/fsnotify"
)

type FileConfigHandler struct {
	notifier   fileConfigUpdate
	ConfigFile string
	logger     core.Logger
}

type fileConfigUpdate interface {
	fileConfigUpdate(name *Config)
}

func NewFileConfigHandler(configFile string, notifier fileConfigUpdate, logger core.Logger) (*FileConfigHandler, error) {
	c := &FileConfigHandler{}
	c.ConfigFile = configFile
	c.notifier = notifier
	c.logger = logger
	go c.watch()
	return c, nil
}

func (c *FileConfigHandler) watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		c.logger.Errorf("create watcher err:%s", err.Error())
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) && event.Name == c.ConfigFile {

					c.logger.Debugf("config file has changed %s", event.Name)

					config, err := BuildFromFile(c.ConfigFile, c.logger)
					if err != nil {
						c.logger.Debugf("Cannot read config file: %q", err)
					}
					c.notifier.fileConfigUpdate(config)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				c.logger.Errorf("watcher err:%s", err.Error())
			}
		}
	}()

	err = watcher.Add(c.ConfigFile)
	if err != nil {
		c.logger.Errorf("watcher err:%s", err.Error())
	}

	select {}
}
