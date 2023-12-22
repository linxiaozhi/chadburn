package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/PremoWeb/Chadburn/core"
	"io"
	"os"
	"path/filepath"
	"time"
)

// SaveConfig configuration for the Save middleware
type SaveConfig struct {
	SaveFolder      string `gcfg:"save-folder" mapstructure:"save-folder"`
	SaveOnlyOnError bool   `gcfg:"save-only-on-error" mapstructure:"save-only-on-error"`
}

// NewSave returns a Save middleware if the given configuration is not empty
func NewSave(c *SaveConfig) core.Middleware {
	var m core.Middleware
	if !IsEmpty(c) {
		m = &Save{*c}
	}

	return m
}

// Save the save middleware saves to disk a dump of the stdout and stderr after
// every execution of the process
type Save struct {
	SaveConfig
}

// ContinueOnStop return allways true, we want always report the final status
func (m *Save) ContinueOnStop() bool {
	return true
}

// Run save the result of the execution to disk
func (m *Save) Run(ctx *core.Context) error {
	err := ctx.Next()
	ctx.Stop(err)

	if ctx.Execution.Failed || !m.SaveOnlyOnError {
		err := m.saveToDisk(ctx)
		if err != nil {
			ctx.Logger.Errorf("Save error: %q", err)
		}
	}

	return err
}

func (m *Save) saveToDisk(ctx *core.Context) error {
	root := filepath.Join(m.SaveFolder, fmt.Sprintf(
		"%s_%s",
		ctx.Job.GetName(), ctx.Execution.Date.Format("20060102"),
	))

	e := ctx.Execution
	err := m.saveReaderToDisk(bytes.NewReader(e.ErrorStream.Bytes()), fmt.Sprintf("%s.log", root))
	if err != nil {
		return err
	}

	err = m.saveReaderToDisk(bytes.NewReader([]byte(fmt.Sprintf("%s [%s]  [%s]\n", time.Now().Format("2006-01-02 15:04:05.000"), ctx.Job.GetName(), ctx.Job.GetCommand()))), fmt.Sprintf("%s.log", root))
	if err != nil {
		return err
	}

	err = m.saveReaderToDisk(bytes.NewReader(e.OutputStream.Bytes()), fmt.Sprintf("%s.log", root))
	if err != nil {
		return err
	}

	err = m.saveContextToDisk(ctx, fmt.Sprintf("%s.json", root))
	if err != nil {
		return err
	}
	return nil
}

func (m *Save) saveContextToDisk(ctx *core.Context, filename string) error {
	js, _ := json.MarshalIndent(map[string]interface{}{
		"Job":       ctx.Job,
		"Execution": ctx.Execution,
	}, "", "  ")

	return m.saveReaderToDisk(bytes.NewBuffer(js), filename)
}

func (m *Save) saveReaderToDisk(r io.Reader, filename string) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return err
	}

	return nil
}
