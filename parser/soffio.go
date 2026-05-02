// Package parser defines the soffio extension and rules.
package parser

import (
	"bufio"
	"io"
	"strings"
)

type state int

const (
	stateHeader state = iota
	stateBody
)

type BlockType int

const (
	BlockTypeText BlockType = iota
	BlockTypeMedia
)

type Section struct {
	ID     string
	Title  string
	Blocks []Block
}

// Document represents a parsed soffio file.
type Document struct {
	ID        string
	Language  string
	Title     string
	Author    string
	Year      string
	Technique string

	Sections []Section
}

type Block interface {
	Type() BlockType
}

type TextBlock struct {
	Content string
}

type MediaBlock struct {
	Path    string
	Caption string
}

func (t TextBlock) Type() BlockType {
	return BlockTypeText
}

func (t MediaBlock) Type() BlockType {
	return BlockTypeMedia
}

func parseHeaderField(d *Document, key, val string) {
	val = strings.TrimSpace(val)

	switch key {
	case "ID":
		d.ID = val
	case "Title", "Titolo":
		d.Title = val
	case "Author", "Autore":
		d.Author = val
	case "Year", "Anno":
		d.Year = val
	case "Technique", "Tecnica":
		d.Technique = val
	case "Language", "Lingua":
		d.Language = val
	}
}

// Parse reads a soffio formatted document from r.
func Parse(r io.Reader) (Document, error) {
	var doc Document
	scanner := bufio.NewScanner(r)
	currentState := stateHeader
	var b strings.Builder

	flushText := func() {
		if b.Len() > 0 && len(doc.Sections) > 0 {
			lastIdx := len(doc.Sections) - 1
			doc.Sections[lastIdx].Blocks = append(doc.Sections[lastIdx].Blocks, TextBlock{Content: b.String()})
			b.Reset()
		}
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch currentState {
		case stateHeader:
			if line == "" {
				currentState = stateBody
				continue
			}
			if key, val, ok := strings.Cut(line, ":"); ok {
				parseHeaderField(&doc, strings.TrimSpace(key), val)
			}

		case stateBody:
			if line == "" {
				flushText()
				continue
			}

			if cmd, isCmd := strings.CutPrefix(line, ":: "); isCmd {
				flushText()

				if strings.HasPrefix(strings.ToLower(cmd), "media: ") {
					_, payload, _ := strings.Cut(cmd, ": ")
					if path, caption, ok := strings.Cut(payload, "|"); ok && len(doc.Sections) > 0 {
						lastIdx := len(doc.Sections) - 1
						doc.Sections[lastIdx].Blocks = append(doc.Sections[lastIdx].Blocks, MediaBlock{
							Path:    strings.TrimSpace(path),
							Caption: strings.TrimSpace(caption),
						})
					}
					continue
				}

				id, title, hasPipe := strings.Cut(cmd, "|")
				if !hasPipe {
					id = cmd
					title = cmd
				}

				doc.Sections = append(doc.Sections, Section{
					ID:    strings.TrimSpace(id),
					Title: strings.TrimSpace(title),
				})
				continue
			}

			if len(doc.Sections) > 0 {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return Document{}, err
	}

	flushText()
	return doc, nil
}
