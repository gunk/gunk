package pot

import (
	"fmt"
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
	AddTranslations(msg []string)
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
func (b *builder) AddTranslations(msg []string) {
	for _, m := range msg {
		b.entries[m] = struct{}{}
	}
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
