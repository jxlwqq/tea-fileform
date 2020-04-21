package service

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/alibabacloud-go/tea/tea"
)

type FileField struct {
	Filename    *string   `json:"filename" xml:"filename" require:"true"`
	ContentType *string   `json:"content-type" xml:"content-type" require:"true"`
	Content     io.Reader `json:"content" xml:"content" require:"true"`
}

type FileFormReader struct {
	formFiles []*formFile
	formField io.Reader
	index     int
	streaming bool
	ifField   bool
}

type formFile struct {
	StartField io.Reader
	EndField   io.Reader
	File       io.Reader
	start      bool
	end        bool
}

func GetBoundary() *string {
	return tea.String(randStringBytes(14))
}

func ToFileForm(body map[string]interface{}, boundary *string) io.Reader {
	out := bytes.NewBuffer(nil)
	line := "--" + tea.StringValue(boundary) + "\r\n"
	forms := make(map[string]string)
	files := make(map[string]map[string]interface{})
	for key, value := range body {
		switch value.(type) {
		case *FileField:
			if val, ok := value.(*FileField); ok {
				out := make(map[string]interface{})
				out["filename"] = tea.StringValue(val.Filename)
				out["content-type"] = tea.StringValue(val.ContentType)
				out["content"] = val.Content
				files[key] = out
			}
		case map[string]interface{}:
			if val, ok := value.(map[string]interface{}); ok {
				files[key] = val
			}
		default:
			forms[key] = fmt.Sprintf("%v", value)
		}
	}
	for key, value := range forms {
		if value != "" {
			out.Write([]byte(line))
			out.Write([]byte("Content-Disposition: form-data; name=\"" + key + "\"" + "\r\n\r\n"))
			out.Write([]byte(value + "\r\n"))
		}
	}
	formFiles := make([]*formFile, 0)
	for key, value := range files {
		start := line
		start += "Content-Disposition: form-data; name=\"" + key + "\"; filename=\"" + value["filename"].(string) + "\"\r\n"
		start += "Content-Type: " + value["content-type"].(string) + "\r\n\r\n"
		formFile := &formFile{
			File:       value["content"].(io.Reader),
			start:      true,
			StartField: strings.NewReader(start),
		}
		if len(files) == len(formFiles)+1 {
			end := "\r\n\r\n--" + tea.StringValue(boundary) + "--\r\n"
			formFile.EndField = strings.NewReader(end)
		} else {
			formFile.EndField = strings.NewReader("\r\n\r\n")
		}
		formFiles = append(formFiles, formFile)
	}
	return &FileFormReader{
		formFiles: formFiles,
		formField: out,
		ifField:   true,
	}
}

func (f *FileFormReader) Read(p []byte) (n int, err error) {
	if f.ifField {
		n, err = f.formField.Read(p)
		if err != nil && err != io.EOF {
			return n, err
		} else if err == io.EOF {
			err = nil
			f.ifField = false
			f.streaming = true
		}
	} else if f.streaming {
		form := f.formFiles[f.index]
		if form.start {
			n, err = form.StartField.Read(p)
			if err != nil && err != io.EOF {
				return n, err
			} else if err == io.EOF {
				err = nil
				form.start = false
			}
		} else if form.end {
			n, err = form.EndField.Read(p)
			if err != nil && err != io.EOF {
				return n, err
			} else if err == io.EOF {
				f.index++
				form.end = false
				if f.index < len(f.formFiles) {
					err = nil
				}
			}
		} else {
			n, err = form.File.Read(p)
			if err != nil && err != io.EOF {
				return n, err
			} else if err == io.EOF {
				err = nil
				form.end = true
			}
		}
	}

	return n, err
}
