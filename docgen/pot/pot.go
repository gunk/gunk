package pot

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/gunk/gunk/reflectutil"
)

const (
	pot = `# Messages.pot - Contains all msgid extracted from swagger definitions.
# Copyright (C) YEAR THE PACKAGE'S COPYRIGHT HOLDER
# This file is distributed under the same license as the PACKAGE package.
# FIRST AUTHOR <EMAIL@ADDRESS>, YEAR.
#
#, fuzzy
msgid   ""
msgstr  "Project-Id-Version: %s\n"
		"Report-Msgid-Bugs-To: %s\n"
		"POT-Creation-Date: %s\n"
		"PO-Revision-Date: YEAR-MO-DA HO:MI+ZONE\n"
		"Last-Translator: FULL NAME <EMAIL@ADDRESS>\n"
		"Language-Team: LANGUAGE <LL@li.org>\n"
		"Language: \n"
		"MIME-Version: 1.0\n"
		"Content-Type: text/plain; charset=CHARSET\n"
		"Content-Transfer-Encoding: 8bit\n"`
)

// Builder provides pot building methods.
type Builder interface {
	AddTranslation(msg string)
	AddFromFile(name string) error
	String() string
}

type builder struct {
	entries map[string]struct{}
}

// NewBuilder creates a new pot builder.
func NewBuilder() Builder {
	return &builder{
		entries: map[string]struct{}{},
	}
}

// AddTranslations add a list of translations..
func (b *builder) AddTranslation(msg string) {
	b.entries[msg] = struct{}{}
}

// String returns a pot-formatted string representation of
// all the saved translations.
func (b *builder) String() string {
	v := reflect.ValueOf(b.entries)
	k := v.MapKeys()
	reflectutil.SortValues(k)

	s := &strings.Builder{}
	fmt.Fprintf(s, "%s\n", pot)
	for _, e := range k {
		fmt.Fprintf(s, "\nmsgid %q\n", e.String())
		fmt.Fprintf(s, "msgstr \"\"\n")
	}
	return s.String()
}

// AddFromFile reads translations from the POT file and
// add its entries to the receiver.
func (b *builder) AddFromFile(name string) error {
	f, err := os.Open(name)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		l := s.Text()
		if strings.HasPrefix(l, "msgid \"") {
			e := strings.TrimSuffix(strings.TrimPrefix(l, "msgid \""), "\"")
			if e != "" {
				b.AddTranslation(e)
			}
		}
	}
	return nil
}
