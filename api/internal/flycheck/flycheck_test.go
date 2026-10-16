// Package flycheck holds the tests of the fly.io configuration files in deploy/ (ADR 0024 steps 2, 5,
// 8 and 9.1): they parse, carry the required keys, and match the production image's layout
// (Dockerfile ENV) and the per-environment differences the ADR sets.
package flycheck_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type flyConfig struct {
	App           string            `toml:"app"`
	PrimaryRegion string            `toml:"primary_region"`
	Build         map[string]any    `toml:"build"`
	Env           map[string]string `toml:"env"`
	Deploy        struct {
		Strategy string `toml:"strategy"`
	} `toml:"deploy"`
	Mounts []struct {
		Source            string `toml:"source"`
		Destination       string `toml:"destination"`
		SnapshotRetention int    `toml:"snapshot_retention"`
	} `toml:"mounts"`
	HTTPService struct {
		InternalPort       int    `toml:"internal_port"`
		AutoStopMachines   string `toml:"auto_stop_machines"`
		AutoStartMachines  bool   `toml:"auto_start_machines"`
		MinMachinesRunning int    `toml:"min_machines_running"`
		ForceHTTPS         bool   `toml:"force_https"`
		Checks             []struct {
			Method string `toml:"method"`
			Path   string `toml:"path"`

			GracePeriod string `toml:"grace_period"`
			Interval    string `toml:"interval"`
			Timeout     string `toml:"timeout"`
		} `toml:"checks"`
	} `toml:"http_service"`
	VM []struct {
		Size   string `toml:"size"`
		Memory string `toml:"memory"`
	} `toml:"vm"`
}

func load(t *testing.T, file string) flyConfig {
	t.Helper()
	raw, err := os.ReadFile("../../../deploy/" + file)
	require.NoError(t, err)
	var c flyConfig
	dec := toml.NewDecoder(strings.NewReader(string(raw))).DisallowUnknownFields()
	require.NoError(t, dec.Decode(&c), "%s must parse, with no key this test does not know", file)
	return c
}

// dockerfileEnv returns the ENV values of the Dockerfile's final stage.
func dockerfileEnv(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("../../../Dockerfile")
	require.NoError(t, err)
	text := string(raw)
	text = text[strings.LastIndex(text, "\nFROM "):]
	block := regexp.MustCompile(`(?m)^ENV ((?:.*\\\n)*.*)`).FindStringSubmatch(text)
	require.NotNil(t, block, "the Dockerfile has no ENV in its final stage")
	env := map[string]string{}
	for _, m := range regexp.MustCompile(`(CRITICAL_[A-Z_]+)=(\S+)`).FindAllStringSubmatch(block[1], -1) {
		env[m[1]] = m[2]
	}
	return env
}

// withoutDSN returns env without CRITICAL_DSN.
func withoutDSN(env map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range env {
		if k != "CRITICAL_DSN" {
			out[k] = v
		}
	}
	return out
}

var files = map[string]string{"fly.staging.toml": "staging", "fly.prod.toml": "prod"}

func TestFly_Step2_RequiredKeysAndImageLayout(t *testing.T) {
	image := dockerfileEnv(t)
	require.NotEmpty(t, image)
	for file, env := range files {
		t.Run(file, func(t *testing.T) {
			c := load(t, file)
			assert.Equal(t, "critical-"+env, c.App)
			assert.Equal(t, "yyz", c.PrimaryRegion)
			assert.Empty(t, c.Build, "the image is passed by the workflow (--image), never named here")
			// The image names no database (ADR 0025 step 1): the DSN is given at run time. The fly
			// configuration still sets the SQLite one of its volume until the fly PostgreSQL
			// configuration replaces both; every other value equals the image's.
			assert.NotContains(t, image, "CRITICAL_DSN", "the production image names no database")
			assert.Equal(t, image, withoutDSN(c.Env), "[env] must equal the image's ENV (Dockerfile), CRITICAL_DSN aside")
			assert.Equal(t, "immediate", c.Deploy.Strategy, "a volume-bound single machine has no blue-green")
			require.Len(t, c.Mounts, 1)
			assert.Equal(t, "data", c.Mounts[0].Source)
			assert.Equal(t, "/data", c.Mounts[0].Destination)
			assert.Equal(t, "/data/critical.db", c.Env["CRITICAL_DSN"], "the database lives on the volume")
			assert.Equal(t, ":8080", c.Env["CRITICAL_ADDR"])
			assert.Equal(t, 8080, c.HTTPService.InternalPort)
			require.Len(t, c.HTTPService.Checks, 1)
			assert.Equal(t, "/api/health", c.HTTPService.Checks[0].Path)
			assert.Equal(t, "GET", c.HTTPService.Checks[0].Method)
			require.Len(t, c.VM, 1)
			assert.Equal(t, "shared-cpu-1x", c.VM[0].Size)
			assert.Contains(t, []string{"256mb", "512mb"}, c.VM[0].Memory)
		})
	}
}

func TestFly_Step2_EnvironmentsDiffer(t *testing.T) {
	staging, prod := load(t, "fly.staging.toml"), load(t, "fly.prod.toml")
	assert.NotEqual(t, staging.App, prod.App)
	// Step 2.1: staging may stop when idle and starts on the next request.
	assert.Equal(t, "stop", staging.HTTPService.AutoStopMachines)
	assert.True(t, staging.HTTPService.AutoStartMachines)
	assert.Zero(t, staging.HTTPService.MinMachinesRunning)
	// Step 2.2: production keeps one machine always running.
	assert.Equal(t, "off", prod.HTTPService.AutoStopMachines)
	assert.Equal(t, 1, prod.HTTPService.MinMachinesRunning)
}

func TestFly_Step9_1_SnapshotRetention(t *testing.T) {
	assert.Equal(t, 7, load(t, "fly.staging.toml").Mounts[0].SnapshotRetention)
	assert.Equal(t, 30, load(t, "fly.prod.toml").Mounts[0].SnapshotRetention)
}

// ADR 0025 step 8.3: the in-memory storage of the e2e runs (CRITICAL_STORAGE)
// can never be enabled in staging or production: no fly configuration and no
// line of the production image names it, so a deployment always runs on the
// database. (The boot also refuses it on a fly machine and in the image:
// TestConfigValidate_Step8_3_InMemoryStorageOnlyOutsideFlyAndTheImage.)
func TestFly_Step8_3_InMemoryStorageNeverEnabled(t *testing.T) {
	for file := range files {
		t.Run(file, func(t *testing.T) {
			c := load(t, file)
			assert.NotContains(t, c.Env, "CRITICAL_STORAGE")
			raw, err := os.ReadFile("../../../deploy/" + file)
			require.NoError(t, err)
			assert.NotContains(t, string(raw), "CRITICAL_STORAGE", "%s must not name the storage mode", file)
		})
	}
	assert.NotContains(t, dockerfileEnv(t), "CRITICAL_STORAGE")
	raw, err := os.ReadFile("../../../Dockerfile")
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "CRITICAL_STORAGE", "the production image must not name the storage mode")
}
