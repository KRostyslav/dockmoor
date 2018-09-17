package dockerfile

import (
	"bytes"
	"github.com/MeneDev/dockmoor/dockref"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
)

var log = logrus.New()

func init() {
	log.SetOutput(bytes.NewBuffer(nil))
}

func TestDockerfileName(t *testing.T) {
	format := DockerfileFormatNew()
	name := format.Name()
	assert.Equal(t, "Dockerfile", name)
}

func TestDockerfileFormatEmptyIsInvalid(t *testing.T) {
	file := ``
	format := DockerfileFormatNew()
	valid := format.ValidateInput(log, strings.NewReader(file), "anything")

	assert.Error(t, valid)
}

func TestDockerfileFormatMissingFromIsInvalid(t *testing.T) {
	file := `RUN command`
	format := DockerfileFormatNew()
	valid := format.ValidateInput(log, strings.NewReader(file), "anything")

	assert.Error(t, valid)
}

func TestDockerfileFormatOtherIsInvalid(t *testing.T) {
	file := `other stuff`
	format := DockerfileFormatNew()
	valid := format.ValidateInput(log, strings.NewReader(file), "anything")

	assert.Error(t, valid)
}

func TestDockerfileFromScratchIsValid(t *testing.T) {
	file := `FROM scratch`
	format := DockerfileFormatNew()
	valid := format.ValidateInput(log, strings.NewReader(file), "anything")

	assert.Nil(t, valid)
}

func TestDockerfileFromScratchPlusInvalidIsInvalid(t *testing.T) {
	file := `FROM scratch
Invalid thing`
	format := DockerfileFormatNew()
	valid := format.ValidateInput(log, strings.NewReader(file), "anything")

	assert.Error(t, valid)
}

func TestDockerfileFromNginxIsValid(t *testing.T) {
	file := `FROM nginx`
	format := DockerfileFormatNew()
	valid := format.ValidateInput(log, strings.NewReader(file), "anything")

	assert.Nil(t, valid)
}

func TestDockerfileFromNginxWithTagIsValid(t *testing.T) {
	file := `FROM nginx:tag`
	format := DockerfileFormatNew()
	valid := format.ValidateInput(log, strings.NewReader(file), "anything")

	assert.Nil(t, valid)
}

func TestDockerfileMultiFromIsValid(t *testing.T) {
	file := `FROM nginx:tag
FROM something:tag`
	format := DockerfileFormatNew()
	valid := format.ValidateInput(log, strings.NewReader(file), "anything")

	assert.Nil(t, valid)
}
func TestMultilineCommandIsValid(t *testing.T) {
	file := `FROM nginx:tag
RUN some \
	command`

	format := DockerfileFormatNew()
	valid := format.ValidateInput(log, strings.NewReader(file), "anything")

	assert.Nil(t, valid)
}

func TestDockerfileMultiFromIsCalls(t *testing.T) {
	file := `FROM nginx:tag
RUN some \
	command

FROM something:tag`
	format := DockerfileFormatNew()
	calls := 0
	format.ValidateInput(log, strings.NewReader(file), "anything")

	err := format.Process(log, strings.NewReader(file), bytes.NewBuffer(nil), func(r dockref.Reference) (string, error) {
		calls++
		return "", nil
	})

	assert.Nil(t, err)
	assert.Equal(t, 2, calls)
}

func TestDockerfilePassProcessorErrors(t *testing.T) {
	file := `FROM valid`
	format := DockerfileFormatNew()
	format.ValidateInput(log, strings.NewReader(file), "anything")

	expected := errors.New("Expected")
	err := format.Process(log, strings.NewReader(file), bytes.NewBuffer(nil), func(r dockref.Reference) (string, error) {
		return "", expected
	})

	assert.Equal(t, expected, err)
}

func TestDockerfilePassMultiLineAndMultistage(t *testing.T) {
	file := `FROM nginx:tag
RUN some \
	command

FROM something:tag
RUN something \
	in the end

# And a comment`
	format := DockerfileFormatNew()
	format.ValidateInput(log, strings.NewReader(file), "anything")

	calls := 0
	err := format.Process(log, strings.NewReader(file), bytes.NewBuffer(nil), func(r dockref.Reference) (string, error) {
		calls++
		return "", nil
	})

	assert.Nil(t, err)
	assert.Equal(t, 2, calls)
}

func TestDockerfileInvalidFromReported(t *testing.T) {
	file := `FROM nginx:a:b`
	format := DockerfileFormatNew()
	format.ValidateInput(log, strings.NewReader(file), "anything")

	processErr := format.Process(log, strings.NewReader(file), bytes.NewBuffer(nil), func(r dockref.Reference) (string, error) {
		return "", nil
	})

	assert.Error(t, processErr)
}

func TestParserErrorsAreReported(t *testing.T) {
	file := `FROM nginx:a:b`
	format := DockerfileFormatNew()

	expected := errors.New("expected")
	format.parseFunction = func(rwc io.Reader) (*parser.Result, error) {
		return nil, expected
	}

	err := format.ValidateInput(log, strings.NewReader(file), "anything")

	assert.Equal(t, expected, err)
}

func TestParserSha256(t *testing.T) {
	file := `FROM nginx@sha256:db5acc22920799fe387a903437eb89387607e5b3f63cf0f4472ac182d7bad644`
	format := DockerfileFormatNew()

	err := format.ValidateInput(log, strings.NewReader(file), "anything")

	processErr := format.Process(log, strings.NewReader(file), bytes.NewBuffer(nil), func(r dockref.Reference) (string, error) {
		return "", nil
	})

	assert.Nil(t, err)
	assert.Nil(t, processErr)
}

func TestProcessLogsReplacingReferences(t *testing.T) {

	var log = logrus.New()
	buffer := bytes.NewBuffer(nil)
	log.SetOutput(buffer)
	log.SetLevel(logrus.InfoLevel)

	file := `FROM nginx`
	format := DockerfileFormatNew()

	err := format.ValidateInput(log, strings.NewReader(file), "anything")

	processErr := format.Process(log, strings.NewReader(file), bytes.NewBuffer(nil), func(r dockref.Reference) (string, error) {
		return "nginx@pinned", nil
	})

	assert.Contains(t, buffer.String(), `nginx@pinned`)
	assert.Nil(t, err)
	assert.Nil(t, processErr)
}
