package github

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
)

// used to narrow down a string that contains env variables
// example:
//2022-02-24T11:10:24.8627684Z env:
//2022-02-24T11:10:24.8628366Z   foo: 1234
//2022-02-24T11:10:24.8629094Z   bar: xyz
//2022-02-24T11:10:24.8629861Z ##[endgroup]
var envGroupRegex = regexp.MustCompile(`(?s)Z env:.*?\[endgroup\]`)

// used to split a single environment variable new line and extract the key-value pair
// example:
//	2022-02-24T11:10:24.8628366Z   foo: 1234
var envLinePramRegex = regexp.MustCompile(`\s{2,}|\t`)

type inMemoryFile struct {
	name string
	body []byte
}

func parseWorkflowParams(logFiles []*inMemoryFile) ([]JobRunParams, error) {
	params := make([]JobRunParams, len(logFiles))
	for i, logFile := range logFiles {
		runParams, err := parseJobRunLog(logFile)
		if err != nil {
			return nil, err
		}
		params[i] = runParams
	}

	return params, nil
}

func parseJobRunLog(logFile *inMemoryFile) (JobRunParams, error) {
	result := JobRunParams{}

	envGroups := envGroupRegex.FindAllString(string(logFile.body), -1)
	if envGroups == nil {
		return result, nil
	}

	for _, envGroup := range envGroups {
		newLines := strings.Split(strings.ReplaceAll(envGroup, "\r\n", "\n"), "\n")

		for i := 0; i < len(newLines); i++ {
			envLine := newLines[i]
			if strings.HasPrefix(envLine, "Z env:") || strings.HasSuffix(envLine, "[endgroup]") {
				continue
			}

			parts := envLinePramRegex.Split(envLine, 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("can't extract env variable from log with name %s and log line '%s' since it containts %d parts", logFile.name, envLine, len(parts))
			}

			keyAndValue := strings.SplitN(parts[1], ": ", 2)
			if len(keyAndValue) != 2 {
				return nil, fmt.Errorf("can't extract key-value pair from log with name %s and key-value string '%s' since it contains %d parts", logFile.name, parts[1], len(keyAndValue))
			}
			result[keyAndValue[0]] = keyAndValue[1]
		}
	}
	return result, nil
}

func readZip(buff bytes.Buffer) ([]*inMemoryFile, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(buff.Bytes()), int64(buff.Len()))

	if err != nil {
		return nil, err
	}

	files := make([]*inMemoryFile, len(zipReader.File))

	for i, zipFile := range zipReader.File {
		zipBytes, err := readZipFile(zipFile)
		if err != nil {
			return nil, fmt.Errorf("failed reading zip file %s, err: %s", zipFile.Name, err)
		}

		files[i] = &inMemoryFile{name: zipFile.Name, body: zipBytes}
	}

	return files, nil
}

func readZipFile(zf *zip.File) ([]byte, error) {
	f, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}
