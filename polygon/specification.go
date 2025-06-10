package polygon

import "strings"

type Specification struct {
	Names       []SpecificationName       `xml:"names>name"`
	Statements  []SpecificationStatement  `xml:"statements>statement"`
	Tutorials   []SpecificationTutorial   `xml:"tutorials>tutorial"`
	Executables []SpecificationExecutable `xml:"files>executables>executable"`
	Resources   []SpecificationResource   `xml:"files>resources>file"`
	Materials   []SpecificationMaterial   `xml:"materials>material"`
	Judging     SpecificationJudging      `xml:"judging"`
	Checker     SpecificationChecker      `xml:"assets>checker"`
	Interactor  SpecificationInteractor   `xml:"assets>interactor"`
	Validator   []SpecificationValidator  `xml:"assets>validators>validator"`
	Solutions   []SpecificationSolution   `xml:"assets>solutions>solution"`
	Tags        []SpecificationTag        `xml:"tags>tag"`
}

func (s *Specification) Tagged(tag string) bool {
	for _, t := range s.Tags {
		if t.Value == tag {
			return true
		}
	}
	return false
}

type SpecificationName struct {
	Language string `xml:"language,attr"`
	Value    string `xml:"value,attr"`
}

type SpecificationStatement struct {
	Charset  string `xml:"charset,attr"`
	Language string `xml:"language,attr"`
	MathJAX  bool   `xml:"mathjax,attr"`
	Path     string `xml:"path,attr"`
	Type     string `xml:"type,attr"`
}

type SpecificationTutorial struct {
	Charset  string `xml:"charset,attr"`
	Language string `xml:"language,attr"`
	MathJAX  bool   `xml:"mathjax,attr"`
	Path     string `xml:"path,attr"`
	Type     string `xml:"type,attr"`
}

type SpecificationExecutable struct {
	Source SpecificationSource `xml:"source"`
	Binary SpecificationBinary `xml:"binary"`
}

type SpecificationResource struct {
	Path     string                     `xml:"path,attr"`
	Type     string                     `xml:"type,attr"`
	ForTypes string                     `xml:"for-types,attr"`
	Assets   []SpecificationGraderAsset `xml:"assets>asset"`
}

func (r *SpecificationResource) Asset(name string) bool {
	for _, a := range r.Assets {
		if strings.ToLower(a.Name) == strings.ToLower(name) {
			return true
		}
	}

	return false
}

type SpecificationGraderAsset struct {
	Name string `xml:"name,attr"`
}

type SpecificationMaterial struct {
	Path    string `xml:"path,attr"`
	Publish string `xml:"publish,attr"`
}

type SpecificationSolution struct {
	Tag    string              `xml:"tag,attr"`
	Source SpecificationSource `xml:"source"`
	Binary SpecificationBinary `xml:"binary"`
}

type SpecificationJudging struct {
	Testsets []SpecificationTestset `xml:"testset"`
	RunCount int                    `xml:"run-count,attr"`
}

type SpecificationTestset struct {
	Name              string               `xml:"name,attr"`
	TimeLimit         int                  `xml:"time-limit"`
	MemoryLimit       int                  `xml:"memory-limit"`
	TestCount         int                  `xml:"test-count"`
	InputPathPattern  string               `xml:"input-path-pattern"`
	AnswerPathPattern string               `xml:"answer-path-pattern"`
	Tests             []SpecificationTest  `xml:"tests>test"`
	Groups            []SpecificationGroup `xml:"groups>group"`
}

type SpecificationTest struct {
	Method  string  `xml:"method,attr"`
	Group   string  `xml:"group,attr"`
	Command string  `xml:"cmd,attr"`
	Sample  bool    `xml:"sample,attr"`
	Points  float32 `xml:"points,attr"`
}

type SpecificationGroup struct {
	FeedbackPolicy string                    `xml:"feedback-policy,attr"`
	Name           string                    `xml:"name,attr"`
	Points         float32                   `xml:"points,attr"`
	PointsPolicy   string                    `xml:"points-policy,attr"`
	Dependencies   []SpecificationDependency `xml:"dependencies>dependency"`
}

type SpecificationDependency struct {
	Group string `xml:"group,attr"`
}

type SpecificationChecker struct {
	Name     string                `xml:"name,attr"`
	Type     string                `xml:"type,attr"`
	Sources  []SpecificationSource `xml:"source"`
	Binaries []SpecificationBinary `xml:"binary"`
}

type SpecificationValidator struct {
	Name     string                `xml:"name,attr"`
	Type     string                `xml:"type,attr"`
	Sources  []SpecificationSource `xml:"source"`
	Binaries []SpecificationBinary `xml:"binary"`
}

type SpecificationInteractor struct {
	Name     string                `xml:"name,attr"`
	Sources  []SpecificationSource `xml:"source"`
	Binaries []SpecificationBinary `xml:"binary"`
	Runs     []string              `xml:"runs>run"`
}

type SpecificationBinary struct {
	Path string `xml:"path,attr"`
	Type string `xml:"type,attr"`
}

type SpecificationSource struct {
	Path string `xml:"path,attr"`
	Type string `xml:"type,attr"`
}

type SpecificationTag struct {
	Value string `xml:"value,attr"`
}

type ProblemProperties struct {
	Language    string `json:"language"`
	Name        string `json:"name"`
	Legend      string `json:"legend"`
	Input       string `json:"input"`
	Interaction string `json:"interaction"`
	Output      string `json:"output"`
	Notes       string `json:"notes"`
	Scoring     string `json:"scoring"`
	AuthorLogin string `json:"authorLogin"`
	AuthorName  string `json:"authorName"`
	Solution    string `json:"tutorial"`
}
