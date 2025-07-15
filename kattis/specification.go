package kattis

import (
	"fmt"
	"time"
)

// string or []string
type StringSeq struct {
	One string
	Seq []string
}

func (s *StringSeq) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// plain string?
	var str string
	if err := unmarshal(&str); err == nil {
		s.One = str
		return nil
	}
	// sequence?
	var list []string
	if err := unmarshal(&list); err == nil {
		s.Seq = list
		return nil
	}
	return fmt.Errorf("must be string or sequence of strings")
}

func (s StringSeq) AsSlice() []string {
	if s.Seq != nil {
		return s.Seq
	}
	if s.One != "" {
		return []string{s.One}
	}
	return nil
}

// string or []string or map[string]string
type StringSeqMap struct {
	String string
	Seq    []string
	Map    map[string]string
}

func (m *StringSeqMap) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err == nil {
		m.String = s
		return nil
	}
	var list []string
	if err := unmarshal(&list); err == nil {
		m.Seq = list
		return nil
	}
	var mp map[string]string
	if err := unmarshal(&mp); err == nil {
		m.Map = mp
		return nil
	}
	return fmt.Errorf("must be string, sequence, or map")
}

// string OR map(locale→string)
type StringOrMap struct {
	One string
	Map map[string]string
}

func (n *StringOrMap) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err == nil {
		n.One = s
		return nil
	}
	var mp map[string]string
	if err := unmarshal(&mp); err == nil {
		n.Map = mp
		return nil
	}
	return fmt.Errorf("name must be string or map(locale→string)")
}

// "Full Name" or mapping {name:,email:,orcid:,kattis:}
type Person struct {
	Name   string `yaml:"name"`
	Email  string `yaml:"email,omitempty"`
	ORCID  string `yaml:"orcid,omitempty"`
	Kattis string `yaml:"kattis,omitempty"`
	// when short form: keep raw string here
	Source string `yaml:"-"`
}

func (p *Person) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var str string
	if err := unmarshal(&str); err == nil {
		p.Source = str
		p.Name = str
		return nil
	}
	type _alias Person
	return unmarshal((*_alias)(p))
}

// single person or sequence
type PersonList []Person

func (pl *PersonList) UnmarshalYAML(um func(interface{}) error) error {
	// single
	var one Person
	if err := um(&one); err == nil && (one.Source != "" || one.Name != "") {
		*pl = PersonList{one}
		return nil
	}
	// list
	var list []Person
	if err := um(&list); err == nil {
		*pl = list
		return nil
	}
	return fmt.Errorf("person list must be person or sequence of persons")
}

// map lang->persons or person list
type Translators struct {
	Default PersonList
	LangMap map[string]PersonList
}

func (tr *Translators) UnmarshalYAML(um func(interface{}) error) error {
	// map?
	var mp map[string]PersonList
	if err := um(&mp); err == nil {
		tr.LangMap = mp
		return nil
	}
	// list?
	var list PersonList
	if err := um(&list); err == nil {
		tr.Default = list
		return nil
	}
	return fmt.Errorf("translators must be map(lang→persons) or person list")
}

type Credits struct {
	Authors         PersonList  `yaml:"authors,omitempty"`
	Contributors    PersonList  `yaml:"contributors,omitempty"`
	Testers         PersonList  `yaml:"testers,omitempty"`
	Translators     Translators `yaml:"translators,omitempty"`
	Packagers       PersonList  `yaml:"packagers,omitempty"`
	Acknowledgments PersonList  `yaml:"acknowledgements,omitempty"`

	Extra map[string]interface{} `yaml:"-,inline"`
}

type Limits struct {
	TimeLimit       int                `yaml:"time_limit,omitempty"`   // seconds
	Memory          int                `yaml:"memory,omitempty"`       // MiB
	OutputLimit     int                `yaml:"output_limit,omitempty"` // MiB
	TimeMultipliers map[string]float64 `yaml:"time_multipliers,omitempty"`
	RunGroups       []RunGroup         `yaml:"run_groups,omitempty"`

	Extra map[string]interface{} `yaml:"-,inline"`
}

type RunGroup struct {
	Count     int `yaml:"count"`
	TimeLimit int `yaml:"time_limit,omitempty"`
	Memory    int `yaml:"memory,omitempty"`
	Output    int `yaml:"output_limit,omitempty"`
}

type Specification struct {
	// required
	ProblemFormatVersion string      `yaml:"problem_format_version"`
	Name                 StringOrMap `yaml:"name"` // string or map
	UUID                 string      `yaml:"uuid"`

	Type StringSeq `yaml:"type,omitempty"`

	// optional common
	Version      string                 `yaml:"version,omitempty"`
	Credits      Credits                `yaml:"credits,omitempty"`
	Source       StringSeqMap           `yaml:"source,omitempty"`
	License      string                 `yaml:"license,omitempty"`
	RightsOwner  string                 `yaml:"rights_owner,omitempty"`
	EmbargoUntil *time.Time             `yaml:"embargo_until,omitempty"`
	Limits       Limits                 `yaml:"limits,omitempty"`
	Keywords     []string               `yaml:"keywords,omitempty"`
	Languages    StringSeq              `yaml:"languages,omitempty"` // default "all"
	AllowWrite   *bool                  `yaml:"allow_file_writing,omitempty"`
	Constants    map[string]interface{} `yaml:"constants,omitempty"`

	// extra
	Extra map[string]interface{} `yaml:"-,inline"`
}
