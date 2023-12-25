package cli

import (
	"github.com/PremoWeb/Chadburn/core"
	"github.com/PremoWeb/Chadburn/middlewares"
	defaults "github.com/mcuadros/go-defaults"
	"github.com/mitchellh/hashstructure/v2"
	gcfg "gopkg.in/gcfg.v1"
)

const (
	jobExec       = "job-exec"
	jobRun        = "job-run"
	jobServiceRun = "job-service-run"
	jobLocal      = "job-local"
)

// Config contains the configuration
type Config struct {
	Global struct {
		middlewares.SlackConfig  `mapstructure:",squash"`
		middlewares.SaveConfig   `mapstructure:",squash"`
		middlewares.MailConfig   `mapstructure:",squash"`
		middlewares.GotifyConfig `mapstructure:",squash"`
	}
	ExecJobs      map[string]*ExecJobConfig    `gcfg:"job-exec" mapstructure:"job-exec,squash"`
	RunJobs       map[string]*RunJobConfig     `gcfg:"job-run" mapstructure:"job-run,squash"`
	ServiceJobs   map[string]*RunServiceConfig `gcfg:"job-service-run" mapstructure:"job-service-run,squash"`
	LocalJobs     map[string]*LocalJobConfig   `gcfg:"job-local" mapstructure:"job-local,squash"`
	sh            *core.Scheduler
	configHandler *FileConfigHandler
	dockerHandler *DockerHandler
	logger        core.Logger
}

func NewConfig(logger core.Logger) *Config {
	// Initialize
	c := &Config{}
	c.ExecJobs = make(map[string]*ExecJobConfig)
	c.RunJobs = make(map[string]*RunJobConfig)
	c.ServiceJobs = make(map[string]*RunServiceConfig)
	c.LocalJobs = make(map[string]*LocalJobConfig)
	c.logger = logger
	defaults.SetDefaults(c)
	return c
}

// BuildFromFile builds a scheduler using the config from a file
func BuildFromFile(filename string, logger core.Logger) (*Config, error) {
	c := NewConfig(logger)
	err := gcfg.ReadFileInto(c, filename)
	return c, err
}

// BuildFromString builds a scheduler using the config from a string
func BuildFromString(config string, logger core.Logger) (*Config, error) {
	c := NewConfig(logger)
	if err := gcfg.ReadStringInto(c, config); err != nil {
		return nil, err
	}
	return c, nil
}

// Call this only once at app init
func (c *Config) InitializeApp(daemon *DaemonCommand) error {
	c.sh = core.NewScheduler(c.logger)
	c.buildSchedulerMiddlewares(c.sh)

	var err error
	c.configHandler, err = NewFileConfigHandler(daemon.ConfigFile, c, c.logger)
	if err != nil {
		return err
	}

	if !daemon.DisableDocker {
		c.dockerHandler, err = NewDockerHandler(c, c.logger)
		if err != nil {
			return err
		}

		for name, j := range c.ExecJobs {
			defaults.SetDefaults(j)
			j.Client = c.dockerHandler.GetInternalDockerClient()
			j.Name = name
			j.buildMiddlewares()
			c.sh.AddJob(j)
		}

		for name, j := range c.RunJobs {
			defaults.SetDefaults(j)
			j.Client = c.dockerHandler.GetInternalDockerClient()
			j.Name = name
			j.buildMiddlewares()
			c.sh.AddJob(j)
		}

		for name, j := range c.ServiceJobs {
			defaults.SetDefaults(j)
			j.Name = name
			j.Client = c.dockerHandler.GetInternalDockerClient()
			j.buildMiddlewares()
			c.sh.AddJob(j)
		}
	}

	for name, j := range c.LocalJobs {
		defaults.SetDefaults(j)
		j.Name = name
		j.buildMiddlewares()
		c.sh.AddJob(j)
	}

	return nil
}

func (c *Config) buildSchedulerMiddlewares(sh *core.Scheduler) {
	sh.Use(middlewares.NewSlack(&c.Global.SlackConfig))
	sh.Use(middlewares.NewSave(&c.Global.SaveConfig))
	sh.Use(middlewares.NewMail(&c.Global.MailConfig))
	sh.Use(middlewares.NewGotify(&c.Global.GotifyConfig))
}

func (c *Config) updateJobs(newConfig *Config, isDockerLabels bool) {
	isClobalConfigUpdate := false
	if !isDockerLabels && !c.CompareHash(c.Global, newConfig.Global) {
		isClobalConfigUpdate = true
		c.Global = newConfig.Global
		c.buildSchedulerMiddlewares(c.sh)
		c.logger.Debugf("config global has changed")
	}

	// Calculate the delta
	for name, j := range c.ExecJobs {
		// this prevents deletion of jobs that were added by reading a configuration file
		if (isDockerLabels && !j.FromDockerLabel) || (!isDockerLabels && j.FromDockerLabel) {
			continue
		}

		if _, ok := newConfig.ExecJobs[name]; ok {
			newJob := newConfig.ExecJobs[name]
			defaults.SetDefaults(newJob)
			newJob.Client = c.dockerHandler.GetInternalDockerClient()
			newJob.Name = name
			newJob.FromDockerLabel = isDockerLabels
			if newJob.Hash() != j.Hash() || isClobalConfigUpdate {
				// Remove from the scheduler
				c.sh.RemoveJob(j)
				// Add the job back to the scheduler
				newJob.buildMiddlewares()
				c.sh.AddJob(newJob)
				// Update the job config
				c.ExecJobs[name] = newJob
			}
		} else {
			c.sh.RemoveJob(j)
			delete(c.ExecJobs, name)
		}
	}

	// Check for aditions
	for newJobsName, newJob := range newConfig.ExecJobs {
		if _, ok := c.ExecJobs[newJobsName]; !ok {
			defaults.SetDefaults(newJob)
			newJob.Client = c.dockerHandler.GetInternalDockerClient()
			newJob.Name = newJobsName
			newJob.FromDockerLabel = isDockerLabels
			newJob.buildMiddlewares()
			c.sh.AddJob(newJob)
			c.ExecJobs[newJobsName] = newJob
		}
	}

	for name, j := range c.RunJobs {
		// this prevents deletion of jobs that were added by reading a configuration file
		if (isDockerLabels && !j.FromDockerLabel) || (!isDockerLabels && j.FromDockerLabel) {
			continue
		}

		if _, ok := newConfig.RunJobs[name]; ok {
			newJob := newConfig.RunJobs[name]
			defaults.SetDefaults(newJob)
			newJob.Client = c.dockerHandler.GetInternalDockerClient()
			newJob.Name = name
			newJob.FromDockerLabel = isDockerLabels
			if newJob.Hash() != j.Hash() || isClobalConfigUpdate {
				// Remove from the scheduler
				c.sh.RemoveJob(j)
				// Add the job back to the scheduler
				newJob.buildMiddlewares()
				c.sh.AddJob(newJob)
				// Update the job config
				c.RunJobs[name] = newJob
			}
		} else {
			c.sh.RemoveJob(j)
			delete(c.RunJobs, name)
		}
	}

	// Check for aditions
	for newJobsName, newJob := range newConfig.RunJobs {
		if _, ok := c.RunJobs[newJobsName]; !ok {
			defaults.SetDefaults(newJob)
			newJob.Client = c.dockerHandler.GetInternalDockerClient()
			newJob.Name = newJobsName
			newJob.FromDockerLabel = isDockerLabels
			newJob.buildMiddlewares()
			c.sh.AddJob(newJob)
			c.RunJobs[newJobsName] = newJob
		}
	}

	for name, j := range c.ServiceJobs {
		// this prevents deletion of jobs that were added by reading a configuration file
		if (isDockerLabels && !j.FromDockerLabel) || (!isDockerLabels && j.FromDockerLabel) {
			continue
		}

		if _, ok := newConfig.ServiceJobs[name]; ok {
			newJob := newConfig.ServiceJobs[name]
			defaults.SetDefaults(newJob)
			newJob.Client = c.dockerHandler.GetInternalDockerClient()
			newJob.Name = name
			newJob.FromDockerLabel = isDockerLabels
			if newJob.Hash() != j.Hash() || isClobalConfigUpdate {
				// Remove from the scheduler
				c.sh.RemoveJob(j)
				// Add the job back to the scheduler
				newJob.buildMiddlewares()
				c.sh.AddJob(newJob)
				// Update the job config
				c.ServiceJobs[name] = newJob
			}
		} else {
			c.sh.RemoveJob(j)
			delete(c.ServiceJobs, name)
		}
	}

	// Check for aditions
	for newJobsName, newJob := range newConfig.ServiceJobs {
		if _, ok := c.ServiceJobs[newJobsName]; !ok {
			defaults.SetDefaults(newJob)
			newJob.Client = c.dockerHandler.GetInternalDockerClient()
			newJob.Name = newJobsName
			newJob.FromDockerLabel = isDockerLabels
			newJob.buildMiddlewares()
			c.sh.AddJob(newJob)
			c.ServiceJobs[newJobsName] = newJob
		}
	}

	for name, j := range c.LocalJobs {
		// this prevents deletion of jobs that were added by reading a configuration file
		if (isDockerLabels && !j.FromDockerLabel) || (!isDockerLabels && j.FromDockerLabel) {
			continue
		}

		if _, ok := newConfig.LocalJobs[name]; ok {
			newJob := newConfig.LocalJobs[name]
			defaults.SetDefaults(newJob)
			newJob.Name = name
			newJob.FromDockerLabel = isDockerLabels
			if newJob.Hash() != j.Hash() || isClobalConfigUpdate {
				// Remove from the scheduler
				c.sh.RemoveJob(j)
				// Add the job back to the scheduler
				newJob.buildMiddlewares()
				c.sh.AddJob(newJob)
				// Update the job config
				c.LocalJobs[name] = newJob
			}
		} else {
			c.sh.RemoveJob(j)
			delete(c.LocalJobs, name)
		}
	}

	// Check for aditions
	for newJobsName, newJob := range newConfig.LocalJobs {
		if _, ok := c.LocalJobs[newJobsName]; !ok {
			defaults.SetDefaults(newJob)
			newJob.Name = newJobsName
			newJob.FromDockerLabel = isDockerLabels
			newJob.buildMiddlewares()
			c.sh.AddJob(newJob)
			c.LocalJobs[newJobsName] = newJob
		}
	}
}

func (c *Config) dockerLabelsUpdate(labels map[string]map[string]string) {
	// Get the current labels
	var parsedLabelConfig Config
	parsedLabelConfig.buildFromDockerLabels(labels)

	//labelsData, _ := json.Marshal(labels)
	//c.logger.Debugf("dockerLabelsUpdate labels :%s", string(labelsData))

	newConfig := NewConfig(c.logger)
	newConfig.Global = parsedLabelConfig.Global
	newConfig.ExecJobs = parsedLabelConfig.ExecJobs
	newConfig.RunJobs = parsedLabelConfig.RunJobs
	newConfig.LocalJobs = parsedLabelConfig.LocalJobs
	newConfig.ServiceJobs = parsedLabelConfig.ServiceJobs

	c.updateJobs(newConfig, true)
}

func (c *Config) fileConfigUpdate(newConfig *Config) {
	c.updateJobs(newConfig, false)
}

func (c *Config) Hash(h interface{}) (uint64, error) {
	hash, err := hashstructure.Hash(c, hashstructure.FormatV2, nil)
	if err != nil {
		c.logger.Errorf("hash err:%q", err)
		return hash, err
	}
	return hash, err
}

func (c *Config) CompareHash(v interface{}, n interface{}) bool {
	oldhash, olderr := hashstructure.Hash(v, hashstructure.FormatV2, nil)
	if olderr != nil {
		c.logger.Errorf("old hash err:%q", olderr)
		return false
	}

	newhash, newerr := hashstructure.Hash(n, hashstructure.FormatV2, nil)
	if newerr != nil {
		c.logger.Errorf("new hash err:%q", newerr)
		return false
	}

	if oldhash != newhash {
		return false
	}
	return true
}

// ExecJobConfig contains all configuration params needed to build a ExecJob
type ExecJobConfig struct {
	core.ExecJob              `mapstructure:",squash"`
	middlewares.OverlapConfig `mapstructure:",squash"`
	middlewares.SlackConfig   `mapstructure:",squash"`
	middlewares.SaveConfig    `mapstructure:",squash"`
	middlewares.MailConfig    `mapstructure:",squash"`
	middlewares.GotifyConfig  `mapstructure:",squash"`
	FromDockerLabel           bool `mapstructure:"fromDockerLabel"`
}

func (c *ExecJobConfig) buildMiddlewares() {
	c.ExecJob.Use(middlewares.NewOverlap(&c.OverlapConfig))
	c.ExecJob.Use(middlewares.NewSlack(&c.SlackConfig))
	c.ExecJob.Use(middlewares.NewSave(&c.SaveConfig))
	c.ExecJob.Use(middlewares.NewMail(&c.MailConfig))
	c.ExecJob.Use(middlewares.NewGotify(&c.GotifyConfig))
}

// RunServiceConfig contains all configuration params needed to build a RunJob
type RunServiceConfig struct {
	core.RunServiceJob        `mapstructure:",squash"`
	middlewares.OverlapConfig `mapstructure:",squash"`
	middlewares.SlackConfig   `mapstructure:",squash"`
	middlewares.SaveConfig    `mapstructure:",squash"`
	middlewares.MailConfig    `mapstructure:",squash"`
	middlewares.GotifyConfig  `mapstructure:",squash"`
	FromDockerLabel           bool `mapstructure:"fromDockerLabel"`
}

type RunJobConfig struct {
	core.RunJob               `mapstructure:",squash"`
	middlewares.OverlapConfig `mapstructure:",squash"`
	middlewares.SlackConfig   `mapstructure:",squash"`
	middlewares.SaveConfig    `mapstructure:",squash"`
	middlewares.MailConfig    `mapstructure:",squash"`
	middlewares.GotifyConfig  `mapstructure:",squash"`
	FromDockerLabel           bool `mapstructure:"fromDockerLabel"`
}

func (c *RunJobConfig) buildMiddlewares() {
	c.RunJob.Use(middlewares.NewOverlap(&c.OverlapConfig))
	c.RunJob.Use(middlewares.NewSlack(&c.SlackConfig))
	c.RunJob.Use(middlewares.NewSave(&c.SaveConfig))
	c.RunJob.Use(middlewares.NewMail(&c.MailConfig))
	c.RunJob.Use(middlewares.NewGotify(&c.GotifyConfig))
}

// LocalJobConfig contains all configuration params needed to build a RunJob
type LocalJobConfig struct {
	core.LocalJob             `mapstructure:",squash"`
	middlewares.OverlapConfig `mapstructure:",squash"`
	middlewares.SlackConfig   `mapstructure:",squash"`
	middlewares.SaveConfig    `mapstructure:",squash"`
	middlewares.MailConfig    `mapstructure:",squash"`
	middlewares.GotifyConfig  `mapstructure:",squash"`
	FromDockerLabel           bool `mapstructure:"fromDockerLabel"`
}

func (c *LocalJobConfig) buildMiddlewares() {
	c.LocalJob.Use(middlewares.NewOverlap(&c.OverlapConfig))
	c.LocalJob.Use(middlewares.NewSlack(&c.SlackConfig))
	c.LocalJob.Use(middlewares.NewSave(&c.SaveConfig))
	c.LocalJob.Use(middlewares.NewMail(&c.MailConfig))
	c.LocalJob.Use(middlewares.NewGotify(&c.GotifyConfig))
}

func (c *RunServiceConfig) buildMiddlewares() {
	c.RunServiceJob.Use(middlewares.NewOverlap(&c.OverlapConfig))
	c.RunServiceJob.Use(middlewares.NewSlack(&c.SlackConfig))
	c.RunServiceJob.Use(middlewares.NewSave(&c.SaveConfig))
	c.RunServiceJob.Use(middlewares.NewMail(&c.MailConfig))
	c.RunServiceJob.Use(middlewares.NewGotify(&c.GotifyConfig))
}
