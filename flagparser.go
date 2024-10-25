package main

import (
	"sort"
	"strings"
)

const (
	FlagSeen    = 'S'
	FlagReplied = 'R'
	FlagFlagged = 'F'
)

type FlagParser struct {
	path  string
	flags []rune
}

func NewFlagParser(path string) *FlagParser {
	fp := &FlagParser{path: path}
	fp.parseFlags()
	return fp
}

func (fp *FlagParser) parseFlags() {
	parts := strings.Split(fp.path, ",")
	flagsStr := parts[len(parts)-1]
	fp.flags = []rune(flagsStr)
}

func (fp *FlagParser) GetFlags() []rune {
	return fp.flags
}

func (fp *FlagParser) ToggleFlag(flag rune) {
	for i, f := range fp.flags {
		if f == flag {
			fp.removeFlag(i)
			return
		}
	}
	fp.SetFlag(flag)
}

func (fp *FlagParser) SetFlag(flag rune) {
	if len([]rune{flag}) > 1 {
		panic("Flag must be one character long")
	}

	for _, f := range fp.flags {
		if f == flag {
			return
		}
	}

	fp.flags = append(fp.flags, flag)

	// sort values in alphabetical order as required by spec
	sort.Slice(fp.flags, func(i, j int) bool {
		return fp.flags[i] < fp.flags[j]
	})

	fp.buildPath()
}

func (fp *FlagParser) removeFlag(index int) {
	fp.flags = append(fp.flags[:index], fp.flags[index+1:]...)
	fp.buildPath()
}

func (fp *FlagParser) GetPath() string {
	return fp.path
}

func (fp *FlagParser) buildPath() {
	flagsStr := string(fp.flags)

	// update path with new flag string
	pathParts := strings.Split(fp.path, ",")
	pathParts[len(pathParts)-1] = flagsStr
	fp.path = strings.Join(pathParts, ",")
}
func (fp *FlagParser) HasFlag(flag rune) bool {
	for _, f := range fp.flags {
		if flag == f {
			return true
		}
	}
	return false
}
