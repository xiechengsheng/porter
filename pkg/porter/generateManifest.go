package porter

import (
	"os"

	"get.porter.sh/porter/pkg/build"
	"get.porter.sh/porter/pkg/config"
	"github.com/mikefarah/yq/v3/pkg/yqlib"
	"github.com/pkg/errors"
	"gopkg.in/op/go-logging.v1"
	"gopkg.in/yaml.v3"
)

// metadataOpts contain manifest fields eligible for dynamic
// updating prior to saving Porter's internal version of the manifest
type metadataOpts struct {
	Name    string
	Version string
	Tag     string // This may be set via Publish
}

// generateInternalManifest decodes the manifest designated by filepath and applies
// the provided generateInternalManifestOpts, saving the updated manifest to the path
// designated by build.LOCAL_MANIFEST
func (p *Porter) generateInternalManifest(opts metadataOpts) error {
	p.initYqLogger()
	var lib = yqlib.NewYqLib()

	// Decode the user's manifest file into a yaml.Node
	var node yaml.Node
	input, err := p.FileSystem.Open(config.Name)
	if err != nil {
		return errors.Wrapf(err, "error opening %s", config.Name)
	}
	defer input.Close()

	var decoder = yaml.NewDecoder(input)
	err = decoder.Decode(&node)
	if err != nil {
		return errors.Wrap(err, "unable to decode manifest")
	}

	// Assemble update commands
	var updateCommands []yqlib.UpdateCommand
	if opts.Name != "" {
		updateCommands = append(updateCommands, createUpdateCommand("name", opts.Name))
	}
	if opts.Version != "" {
		updateCommands = append(updateCommands, createUpdateCommand("version", opts.Version))
	}

	// Make updates
	for _, updateCommand := range updateCommands {
		err = lib.Update(&node, updateCommand, true)
		if err != nil {
			return errors.Wrapf(err, "could not update manifest path %q with value %q", updateCommand.Path, updateCommand.Value.Value)
		}
	}

	// Create the local app dir if it does not already exist
	err = p.FileSystem.MkdirAll(build.LOCAL_APP, 0755)
	if err != nil {
		return errors.Wrapf(err, "unable to create directory %s", build.LOCAL_APP)
	}

	output, err := p.FileSystem.OpenFile(build.LOCAL_MANIFEST, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return errors.Wrapf(err, "error creating %s", build.LOCAL_MANIFEST)
	}
	defer output.Close()

	// Encode the updated manifest to the proper location
	// yqlib.NewYamlEncoder takes: dest (io.Writer), indent spaces (int), colorised output (bool)
	var encoder = yqlib.NewYamlEncoder(output, 2, false)
	return errors.Wrapf(encoder.Encode(&node), "unable to encode the manifest at %s", build.LOCAL_MANIFEST)
}

// createUpdateCommand creates a yqlib.UpdateCommand using the supplied YAML
// path and replacement value
func createUpdateCommand(path, value string) yqlib.UpdateCommand {
	var valueParser = yqlib.NewValueParser()
	// valueParser.Parse takes: argument (string), custom tag (string),
	// custom style (string), anchor name (string), create alias (bool)
	var parsedValue = valueParser.Parse(value, "", "", "", false)
	return yqlib.UpdateCommand{Command: "update", Path: path, Value: parsedValue, Overwrite: true}
}

func (p *Porter) initYqLogger() {
	// The yq lib that we use makes frequent calls to a logger
	// Here we set up to log to Porter's Err stream at log level ERROR
	var backend = logging.AddModuleLevel(logging.NewLogBackend(p.Err, "", 0))
	backend.SetLevel(logging.ERROR, "yq")
	logging.SetBackend(backend)
}
