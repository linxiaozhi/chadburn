package cli

import (
	"crypto/sha256"
	"fmt"
	"github.com/PremoWeb/Chadburn/core"
	"io"
	"os"
	"time"
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
	cfgHash := c.getCfgHash(c.ConfigFile)
	tick := time.Tick(10000 * time.Millisecond)
	for {
		select {
		case <-tick:
			newCfgHash := c.getCfgHash(c.ConfigFile)
			if cfgHash != newCfgHash {
				c.logger.Debugf("config file has changed,old hash:%s,new hash:%s", cfgHash, newCfgHash)
				config, err := BuildFromFile(c.ConfigFile, c.logger)
				if err != nil {
					c.logger.Debugf("Cannot read config file: %q", err)
				}
				c.notifier.fileConfigUpdate(config)
				cfgHash = newCfgHash
			}
		}
	}
}

func (c *FileConfigHandler) getCfgHash(filename string) string {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		c.logger.Errorf("filename:%s,err:%s", filename, err.Error())
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		c.logger.Errorf("filename:%s,err:%s", filename, err.Error())
	}
	sum := fmt.Sprintf("%x", hash.Sum(nil))

	return sum
}
