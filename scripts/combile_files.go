package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: git_combine <username/repository/?optional_branch> output.path")
		os.Exit(1)
	}

	repoInfo := os.Args[1]
	outputPath := os.Args[2]

	tmpDir, err := ioutil.TempDir("", "git_combine")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cloneRepo(repoInfo, tmpDir)
	combinedContent := getCombinedContent(tmpDir)
	writeCombinedContentToFile(combinedContent, outputPath)
}

func cloneRepo(repoInfo, tmpDir string) {
	cmd := exec.Command("git", "clone", "https://github.com/"+repoInfo, tmpDir)
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func getCombinedContent(root string) string {
	var combinedContent bytes.Buffer
	var treeContent bytes.Buffer
	printTree(root, "", &treeContent)

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == filepath.Join(root, ".git") {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(root, path)
		if !isTextBased(relPath) {
			content, _ := ioutil.ReadFile(path)
			combinedContent.WriteString("\n" + relPath + "\n")
			combinedContent.Write(content)
			combinedContent.WriteString("\n\n" + strings.Repeat("=", 80) + "\n")
		}
		return nil
	})

	return treeContent.String() + "\n" + combinedContent.String()
}

func printTree(root, prefix string, buffer *bytes.Buffer) {
	files, err := ioutil.ReadDir(root)
	if err != nil {
		log.Fatal(err)
	}

	for i, file := range files {
		if file.Name() == ".git" {
			continue
		}
		last := i == len(files)-1
		var newPrefix string
		if last {
			buffer.WriteString(prefix + "└── " + file.Name() + "\n")
			newPrefix = prefix + "    "
		} else {
			buffer.WriteString(prefix + "├── " + file.Name() + "\n")
			newPrefix = prefix + "│   "
		}
		if file.IsDir() {
			printTree(filepath.Join(root, file.Name()), newPrefix, buffer)
		}
	}
}

func isTextBased(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Check the file header
	header := make([]byte, 512)
	_, err = file.Read(header)
	if err != nil && err != io.EOF {
		return false
	}

	// Check the contents
	var buf [1024]byte
	n, err := file.Read(buf[:])
	if err != nil && err != io.EOF {
		return false
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return false
		}
	}

	// Check for UTF-8 BOM
	if bytes.HasPrefix(header, []byte("\xef\xbb\xbf")) {
		return true
	}

	return true
}

func writeCombinedContentToFile(content, outputPath string) {
	err := ioutil.WriteFile(outputPath, []byte(content), 0644)
	if err != nil {
		log.Fatal(err)
	}
}
