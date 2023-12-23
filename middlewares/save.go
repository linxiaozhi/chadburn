package middlewares

import (
	"bufio"
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
	SaveFolder         string `gcfg:"save-folder" mapstructure:"save-folder"`
	SaveOnlyOnError    bool   `gcfg:"save-only-on-error" mapstructure:"save-only-on-error"`
	SaveJobExecContext bool   `gcfg:"save-job-exec-context" mapstructure:"save-job-exec-context"`
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

	if m.SaveJobExecContext {
		err := m.saveJobExecContextToDisk(ctx)
		if err != nil {
			ctx.Logger.Errorf("Save error: %q", err)
		}
	}

	return err
}

func (m *Save) saveJobExecContextToDisk(ctx *core.Context) error {
	root := filepath.Join(m.SaveFolder, fmt.Sprintf(
		"%s_%s",
		ctx.Job.GetName(), ctx.Execution.Date.Format("20060102"),
	))

	err := m.saveContextToDisk(ctx, fmt.Sprintf("%s.json", root))
	if err != nil {
		return err
	}

	return nil
}

func (m *Save) saveToDisk(ctx *core.Context) error {
	root := filepath.Join(m.SaveFolder, fmt.Sprintf(
		"%s_%s",
		ctx.Job.GetName(), ctx.Execution.Date.Format("20060102"),
	))

	errText := "none"
	if ctx.Execution.Error != nil {
		errText = ctx.Execution.Error.Error()
	}

	msgText := fmt.Sprintf("%s [Job \"%s\" (%s)] Started - %s\n", time.Now().Format("2006-01-02 15:04:05.000"), ctx.Job.GetName(), ctx.Execution.ID, ctx.Job.GetCommand())
	msgText += fmt.Sprintf("Output: %s", string(ctx.Execution.OutputStream.Bytes()))
	msgText += fmt.Sprintf("Finished in %q, failed: %t, skipped: %t, error: %s\n\n", ctx.Execution.Duration, ctx.Execution.Failed, ctx.Execution.Skipped, errText)

	err := m.saveStringToDisk(msgText, fmt.Sprintf("%s.log", root))
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

	return m.saveStringToDisk(fmt.Sprintf("%s\n\n", string(js)), filename)
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

func (m *Save) saveBytesToDisk(p []byte, filename string) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	writer.Write(p)
	writer.Flush()

	return nil
}

func (m *Save) saveStringToDisk(s string, filename string) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	writer.WriteString(s)
	writer.Flush()

	return nil
}
