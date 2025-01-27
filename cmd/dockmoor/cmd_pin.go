package main

import (
	"bytes"
	"errors"
	"github.com/MeneDev/dockmoor/dockfmt"
	"github.com/MeneDev/dockmoor/dockproc"
	"github.com/MeneDev/dockmoor/dockref"
	"github.com/jessevdk/go-flags"
	"io"
	"io/ioutil"
	"os"
)

type pinOptions struct {
	MatchingOptions

	ReferenceFormat struct {
		ForceDomain bool `required:"no" long:"force-domain" description:"Includes domain even in well-known references"`
		NoName      bool `required:"no" long:"no-name" description:"Formats well-known references as digest only"`
		NoTag       bool `required:"no" long:"no-tag" description:"Don't include the tag in the reference"`
		NoDigest    bool `required:"no" long:"no-digest" description:"Don't include the digest in the reference"`
	} `group:"Reference format" description:"Control the format of references, defaults are sensible, changes are not recommended"`

	Output struct {
		OutputFile flags.Filename `required:"no" short:"o" long:"output" description:"Output file to write to. If empty, input file will be used."`
	} `group:"Output parameters" description:"Output parameters"`

	repoFactory func() dockref.Resolver
	matches     bool
}

func (po *pinOptions) Execute(args []string) error {
	return errors.New("Use ExecuteWithExitCode instead")
}

func (po *pinOptions) ExecuteWithExitCode(args []string) (exitCode ExitCode, err error) {
	// TODO code is redundant to other commands
	mopts := po.MatchingOptions

	exitCode, err = mopts.Verify()
	if err != nil {
		return
	}

	predicate, err := mopts.getPredicate()
	if err != nil {
		return ExitPredicateInvalid, err
	}

	buffer := bytes.NewBuffer(nil)

	err = mopts.WithInputDo(func(inputPath string, inputReader io.Reader) error {

		errFormat := mopts.WithFormatProcessorDo(inputReader, func(processor dockfmt.FormatProcessor) error {
			processor = processor.WithWriter(buffer)
			return po.applyFormatProcessor(predicate, processor)
		})

		if errFormat != nil {
			exitCode = ExitInvalidFormat
			return errFormat
		}
		return nil
	})

	if exitCode, ok := exitCodeFromError(err); ok {
		return exitCode, err
	}

	err = po.WithOutputDo(func(outputPath string) error {

		mode := os.FileMode(0660)

		info, e := os.Stat(outputPath)
		if e == nil {
			mode = info.Mode()
		}

		errWriteFile := ioutil.WriteFile(outputPath, buffer.Bytes(), mode)
		return errWriteFile
	})

	if po.matches {
		exitCode = ExitSuccess
	} else {
		exitCode = ExitNotFound
	}

	return exitCode, err
}

func (po *pinOptions) applyFormatProcessor(predicate dockproc.Predicate, processor dockfmt.FormatProcessor) error {

	return processor.Process(func(original dockref.Reference) (dockref.Reference, error) {
		if predicate.Matches(original) {
			repo := po.Repo()
			rs, err := repo.Resolve(original)
			if err != nil {
				po.Log().WithField("error", err.Error()).Errorf("Could not resolve %s", original.Original())
				return nil, err
			}

			format, err := po.RefFormat()
			if err != nil {
				return nil, err
			}

			mostPrecise, err := dockref.MostPreciseTag(rs, po.Log())

			if err == nil {
				po.matches = true
				reference, e := mostPrecise.WithRequestedFormat(format)
				if e != nil {
					return nil, e
				}
				mostPrecise = reference
				return mostPrecise, err
			}
			return mostPrecise, err
		}
		return original, nil
	})
}

func (po *pinOptions) Repo() dockref.Resolver {
	return po.repoFactory()
}

func pinOptionsNew(mainOptions *mainOptions, resolverFactory func() dockref.Resolver) *pinOptions {
	po := pinOptions{
		MatchingOptions: MatchingOptions{
			mainOpts: mainOptions,
		},
		repoFactory: resolverFactory,
		matches:     false,
	}

	return &po
}

func addPinCommand(mainOptions *mainOptions, adder func(opts *mainOptions, command string, shortDescription string, longDescription string, data interface{}) (*flags.Command, error)) (*flags.Command, error) {
	repoFactory := mainOptions.resolverFactory()
	pinOptions := pinOptionsNew(mainOptions, repoFactory)

	command, e := adder(mainOptions, "pin",
		"Change image references to a more reproducible format",
		"Change image references to a more reproducible format by adding version tags or digest",
		pinOptions)
	if e != nil {
		return nil, e

	}
	return command, e
}

func (po *pinOptions) RefFormat() (dockref.Format, error) {
	format := dockref.FormatHasName | dockref.FormatHasTag | dockref.FormatHasDigest

	rf := po.ReferenceFormat

	if rf.NoDigest && rf.NoName {
		return 0, errors.New("invalid Reference Format: --no-name and --no-digest are mutually exclusive")
	}

	if rf.ForceDomain && rf.NoName {
		return 0, errors.New("invalid Reference Format: --force-domain and --no-name are mutually exclusive")
	}

	if rf.ForceDomain {
		format |= dockref.FormatHasDomain
	}
	if rf.NoName {
		format &= ^dockref.FormatHasName
	}
	if rf.NoTag {
		format = format & ^dockref.FormatHasTag
	}
	if rf.NoDigest {
		format &= ^dockref.FormatHasDigest
	}

	return format, nil
}

func (po *pinOptions) WithOutputDo(action func(outputPath string) error) error {
	filename := string(po.Output.OutputFile)
	if filename != "" {
		return action(filename)
	}

	return po.MatchingOptions.WithOutputDo(action)
}
