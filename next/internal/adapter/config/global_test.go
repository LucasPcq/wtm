package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnAbsentGlobalConfigIsNoSetting(t *testing.T) {
	_, global, err := loadState(t, stateWithConfig(t, minimalConfig))

	require.NoError(t, err)
	assert.Equal(t, project.Global{}, global)
}

func TestTheProxyPortIsDecoded(t *testing.T) {
	global, err := loadWithGlobal(t, "shell = \"zsh\"\n\n[proxy]\nport = 5000\n")

	require.NoError(t, err)
	assert.Equal(t, 5000, global.Proxy.Port)
	assert.Nil(t, global.Proxy.Enabled, "an absent key is no answer")
}

func TestAProxySwitchedOffIsAnExplicitFalse(t *testing.T) {
	global, err := loadWithGlobal(t, "[proxy]\nenabled = false\n")

	require.NoError(t, err)
	require.NotNil(t, global.Proxy.Enabled)
	assert.False(t, *global.Proxy.Enabled)
}

func TestEveryKeyWtmWritesInTheGlobalConfigIsAccepted(t *testing.T) {
	_, err := loadWithGlobal(t, "schema = 1\nshell = \"fish\"\n\n[ui]\nanimations = false\n\n[proxy]\nport = 11080\nenabled = true\n\n[update]\ncheck = false\n")

	assert.NoError(t, err)
}

func TestAnUnknownKeyInTheGlobalConfigIsRefused(t *testing.T) {
	_, err := loadWithGlobal(t, "[proxy]\nprot = 5000\n")

	assert.ErrorContains(t, err, "proxy.prot")
}

func TestAnUnknownShellIsRefused(t *testing.T) {
	_, err := loadWithGlobal(t, "shell = \"tcsh\"\n")

	assert.ErrorContains(t, err, "invalid shell type: must be zsh, bash, or fish")
}

func TestABadProxyPortDoesNotStopTheLoad(t *testing.T) {
	global, err := loadWithGlobal(t, "[proxy]\nport = 70000\n")

	require.NoError(t, err)
	assert.Equal(t, 70000, global.Proxy.Port)
}

func TestNoGlobalPathMeansNoGlobalConfig(t *testing.T) {
	_, global, err := Project{StateDir: stateWithConfig(t, minimalConfig)}.Load()

	require.NoError(t, err)
	assert.Equal(t, project.Global{}, global)
}

func TestAGlobalConfigThatIsNotTOMLIsRefused(t *testing.T) {
	_, err := loadWithGlobal(t, "[proxy\n")

	assert.ErrorContains(t, err, "global config")
}

func TestTheProjectIsCheckedBeforeTheShellAndTheShellErrorStandsAlone(t *testing.T) {
	globalPath := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, globalPath, "shell = \"powershell\"\n")

	_, _, bad := Project{StateDir: stateWithConfig(t, "[env]\nstrategy = \"bogus\"\n"), GlobalPath: globalPath}.Load()
	_, _, shell := Project{StateDir: stateWithConfig(t, minimalConfig), GlobalPath: globalPath}.Load()

	assert.EqualError(t, bad, "invalid env strategy: must be example, main, or parent")
	assert.EqualError(t, shell, "invalid shell type: must be zsh, bash, or fish")
}

func TestTheGlobalConfigIsLoadedAloneWithItsProxyValidated(t *testing.T) {
	dir := t.TempDir()
	write := func(content string) string {
		path := filepath.Join(dir, "config.toml")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}

	global, err := LoadGlobal(write("[proxy]\nport = 5000\n"))
	require.NoError(t, err)
	_, invalid := LoadGlobal(write("[proxy]\nport = 70000\n"))
	absent, none := LoadGlobal(filepath.Join(dir, "absent.toml"))

	assert.Equal(t, 5000, global.ProxyPort())
	assert.ErrorContains(t, invalid, "invalid [proxy] port 70000")
	require.NoError(t, none)
	assert.Equal(t, 11080, absent.ProxyPort(), "the default port")
}
