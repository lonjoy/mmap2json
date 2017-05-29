package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

type JobGroup struct {
	num int
	ch  chan int
}

func (this *JobGroup) Add(f interface{}, params ...interface{}) {
	g := reflect.ValueOf(f)
	in := make([]reflect.Value, len(params))
	for k, param := range params {
		in[k] = reflect.ValueOf(param)
	}
	go func() {
		g.Call(in)
		this.ch <- 1
	}()
	this.num++
}

func (this *JobGroup) Wait() {
	for i := 0; i < this.num; i++ {
		<-this.ch
	}
}

func NewJobGroup() *JobGroup {
	job_group := JobGroup{}
	job_group.ch = make(chan int)
	return &job_group
}

type Topic struct {
	Text struct {
		Name string `xml:"PlainText,attr"`
	} `xml:"Text"`
	SubTopics []Topic `xml:"SubTopics>Topic"`
}

type Mmap struct {
	Topic Topic `xml:"OneTopic>Topic"`
}

type Node struct {
	Name     string `json:"name"`
	Children []Node `json:"children,omitempty"`
}

func to_json(node *Node) []byte {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "\t")
	_ = encoder.Encode(node)
	return buf.Bytes()
}

func convert_json(topic Topic) Node {
	node := Node{}
	node.Name = topic.Text.Name
	for _, v := range topic.SubTopics {
		node.Children = append(node.Children, convert_json(v))
	}
	return node
}

func xml_to_json(buf *bytes.Buffer) []byte {

	result := Mmap{}
	xml.Unmarshal(buf.Bytes(), &result)
	// fmt.Printf("%v\n", buf.Bytes())

	node := convert_json(result.Topic)
	return to_json(&node)
}

func save_to_file(input string, output string, buf []byte) {
	fmt.Println(input, "to", output)
	os.MkdirAll(filepath.Dir(output), os.ModePerm)
	ioutil.WriteFile(output, []byte(buf), os.ModePerm)
}

func convert_file(path string, input string, output string) {

	name, _ := filepath.Rel(input, path)
	name = filepath.Join(output, name)
	ext := filepath.Ext(name)
	name = name[0:len(name)-len(ext)] + ".json"

	r, err := zip.OpenReader(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "Document.xml" {
			// fmt.Printf("文件名 %s:\n", f.Name)
			rc, err := f.Open()
			if err != nil {
				fmt.Println(err)
				return
			}
			defer rc.Close()

			buf := new(bytes.Buffer)
			buf.ReadFrom(rc)
			str := xml_to_json(buf)
			save_to_file(path, name, str)
		}
	}
}

func usage() {
	fmt.Println("usage: mmap2json -i=<input path> -o=<output path>")
}

func scan_dir(input string) []string {
	list := make([]string, 0, 100)

	filepath.Walk(input, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".mmap" {
			list = append(list, path)
		}
		return nil
	})
	return list
}

func main() {
	if len(os.Args) < 3 {
		usage()
		return
	}
	input := flag.String("i", "", "input path")
	output := flag.String("o", "", "output path")
	flag.Parse()

	if *input == "" || *output == "" {
		usage()
		return
	}
	// fmt.Println(*input)
	// fmt.Println(*output)

	start := time.Now()
	fmt.Println("start convert\n")

	list := scan_dir(*input)

	os.RemoveAll(*output)
	// fmt.Println(list)

	job_group := NewJobGroup()
	for _, path := range list {
		job_group.Add(convert_file, path, *input, *output)
	}
	job_group.Wait()

	fmt.Println("\ndone", time.Since(start))
}
