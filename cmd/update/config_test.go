package update

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/devspace-cloud/devspace/cmd/flags"
	"github.com/devspace-cloud/devspace/pkg/devspace/config/configutil"
	"github.com/devspace-cloud/devspace/pkg/devspace/config/constants"
	"github.com/devspace-cloud/devspace/pkg/devspace/config/versions/latest"
	"github.com/devspace-cloud/devspace/pkg/devspace/config/versions/v1alpha1"
	"github.com/devspace-cloud/devspace/pkg/util/fsutil"
	"github.com/devspace-cloud/devspace/pkg/util/log"
	"github.com/devspace-cloud/devspace/pkg/util/ptr"

	"gopkg.in/yaml.v2"
	"gotest.tools/assert"
)

var logOutput string

type testLogger struct {
	log.DiscardLogger
}

func (t testLogger) Info(args ...interface{}) {
	logOutput = logOutput + "\nInfo " + fmt.Sprint(args...)
}
func (t testLogger) Infof(format string, args ...interface{}) {
	logOutput = logOutput + "\nInfo " + fmt.Sprintf(format, args...)
}

func (t testLogger) Done(args ...interface{}) {
	logOutput = logOutput + "\nDone " + fmt.Sprint(args...)
}
func (t testLogger) Donef(format string, args ...interface{}) {
	logOutput = logOutput + "\nDone " + fmt.Sprintf(format, args...)
}

func (t testLogger) Fail(args ...interface{}) {
	logOutput = logOutput + "\nFail " + fmt.Sprint(args...)
}
func (t testLogger) Failf(format string, args ...interface{}) {
	logOutput = logOutput + "\nFail " + fmt.Sprintf(format, args...)
}

func (t testLogger) Warn(args ...interface{}) {
	logOutput = logOutput + "\nWarn " + fmt.Sprint(args...)
}
func (t testLogger) Warnf(format string, args ...interface{}) {
	logOutput = logOutput + "\nWarn " + fmt.Sprintf(format, args...)
}

func (t testLogger) StartWait(msg string) {
	logOutput = logOutput + "\nWait " + fmt.Sprint(msg)
}

func (t testLogger) Write(msg []byte) (int, error) {
	logOutput = logOutput + string(msg)
	return len(msg), nil
}

type updateConfigTestCase struct {
	name string

	globalFlags flags.GlobalFlags
	files       map[string]interface{}

	expectedOutput string
	expectedConfig interface{}
	expectedErr    string
}

func TestRunUpdateConfig(t *testing.T) {
	dir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}

	wdBackup, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current working directory: %v", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Error changing working directory: %v", err)
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		//Delete temp folder
		err = os.Chdir(wdBackup)
		if err != nil {
			t.Fatalf("Error changing dir back: %v", err)
		}
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatalf("Error removing dir: %v", err)
		}
	}()

	testCases := []updateConfigTestCase{
		updateConfigTestCase{
			name: "Safe with profiles",
			files: map[string]interface{}{
				constants.DefaultConfigPath: latest.Config{
					Version: latest.Version,
					Profiles: []*latest.ProfileConfig{
						&latest.ProfileConfig{},
					},
				},
			},
			expectedOutput: "\nWarn 'devspace update config' does NOT update profiles[*].replace or profiles[*].patches. Please manually update any profiles[*].replace and profiles[*].patches\nInfo Successfully converted base config to current version",
			expectedConfig: latest.Config{
				Version: latest.Version,
				Dev:     &latest.DevConfig{},
			},
		},
		updateConfigTestCase{
			name: "Old version",
			files: map[string]interface{}{
				constants.DefaultConfigPath: v1alpha1.Config{
					Version: ptr.String(v1alpha1.Version),
					DevSpace: &v1alpha1.DevSpaceConfig{
						Services: &[]*v1alpha1.ServiceConfig{
							&v1alpha1.ServiceConfig{
								Name: ptr.String("terminalService"),
							},
						},
						Terminal: &v1alpha1.Terminal{
							Disabled:      ptr.Bool(true),
							Service:       ptr.String("terminalService"),
							ResourceType:  ptr.String("terminalRT"),
							LabelSelector: &map[string]*string{"hello": ptr.String("World")},
							Namespace:     ptr.String("someNS"),
							ContainerName: ptr.String("someContainer"),
							Command:       &[]*string{ptr.String("myCommand")},
						},
					},
				},
			},
			expectedOutput: "\nInfo Successfully converted base config to current version",
		},
	}

	log.SetInstance(&testLogger{
		log.DiscardLogger{PanicOnExit: true},
	})

	for _, testCase := range testCases {
		testRunUpdateConfig(t, testCase)
	}
}

func testRunUpdateConfig(t *testing.T, testCase updateConfigTestCase) {
	logOutput = ""

	for path, content := range testCase.files {
		asYAML, err := yaml.Marshal(content)
		assert.NilError(t, err, "Error parsing config to yaml in testCase %s", testCase.name)
		err = fsutil.WriteToFile(asYAML, path)
		assert.NilError(t, err, "Error writing file in testCase %s", testCase.name)
	}

	configutil.ResetConfig()

	err := (&configCmd{
		GlobalFlags: &testCase.globalFlags,
	}).RunConfig(nil, []string{})

	if testCase.expectedErr == "" {
		assert.NilError(t, err, "Unexpected error in testCase %s.", testCase.name)

		/*config, err := configutil.GetConfig(nil)
		assert.NilError(t, err, "Error getting config after init call in testCase %s.", testCase.name)
		configYaml, err := yaml.Marshal(config)
		assert.NilError(t, err, "Error parsing config to yaml after init call in testCase %s.", testCase.name)
		expectedConfigYaml, err := yaml.Marshal(testCase.expectedConfig)
		assert.NilError(t, err, "Error parsing expected config to yaml after init call in testCase %s.", testCase.name)
		assert.Equal(t, string(configYaml), string(expectedConfigYaml), "Initialized config is wrong in testCase %s.", testCase.name)*/
	} else {
		assert.Error(t, err, testCase.expectedErr, "Wrong or no error in testCase %s.", testCase.name)
	}
	assert.Equal(t, logOutput, testCase.expectedOutput, "Unexpected output in testCase %s", testCase.name)

	err = filepath.Walk(".", func(path string, f os.FileInfo, err error) error {
		os.RemoveAll(path)
		return nil
	})
	assert.NilError(t, err, "Error cleaning up in testCase %s", testCase.name)
}
