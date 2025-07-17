package kattis

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	// "slices"
	// "sort"
	// "strconv"
	"strings"
	"time"

	"github.com/eolymp/go-problems/connector"
	assetpb "github.com/eolymp/go-sdk/eolymp/asset"
	atlaspb "github.com/eolymp/go-sdk/eolymp/atlas"
	ecmpb "github.com/eolymp/go-sdk/eolymp/ecm"
	executorpb "github.com/eolymp/go-sdk/eolymp/executor"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	yaml "gopkg.in/yaml.v2"
)

var imageFinder = regexp.MustCompile("(\\\\includegraphics.*?{)(.+?)(})")

type ProblemLoader struct {
	upload *connector.MultipartUploader
	log    connector.Logger
}

func NewProblemLoader(upload connector.Uploader, log connector.Logger) *ProblemLoader {
	return &ProblemLoader{
		log:    log,
		upload: connector.NewMultipartUploader(upload, log),
	}
}

// helper to get past the root folder created after downloader the zip
func resolveRoot(path string) (string, error) {
	if fileExists(filepath.Join(path, "problem.yaml")) {
		return path, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	var sub string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			if sub != "" { // more than one folder
				return "", fmt.Errorf("no problem.yaml at archive root")
			}
			sub = e.Name()
		}
	}
	if sub == "" {
		return "", fmt.Errorf("problem.yaml not found")
	}
	if fileExists(filepath.Join(path, sub, "problem.yaml")) {
		return filepath.Join(path, sub), nil
	}
	return "", fmt.Errorf("problem.yaml not found")
}

func (p *ProblemLoader) Fetch(ctx context.Context, link string) (*atlaspb.Snapshot, error) {
	// create workspace
	path := filepath.Join(os.TempDir(), uuid.New().String())
	if err := os.Mkdir(path, 0777); err != nil {
		return nil, fmt.Errorf("unable to create workspace: %w", err)
	}

	defer p.cleanup(path)

	start := time.Now()

	p.log.Printf("Downloading problem archive")

	// download and unpack
	if err := p.download(ctx, path, link); err != nil {
		return nil, fmt.Errorf("unable to download problem archive: %w", err)
	}

	p.log.Printf("Downloaded in %v", time.Since(start))

	start = time.Now()

	if err := p.unpack(ctx, path); err != nil {
		return nil, fmt.Errorf("unable to unpack problem archive: %w", err)
	}

	p.log.Printf("Unpacked in %v!", time.Since(start))

	path, err := resolveRoot(path)
	if err != nil {
		return nil, err
	}

	return p.Snapshot(ctx, path)
}

// Snapshot reads problem specification from the unpacked problem archive and returns a Snapshot of the problem.
func (p *ProblemLoader) Snapshot(ctx context.Context, path string) (*atlaspb.Snapshot, error) {
	file, err := os.Open(filepath.Join(path, "problem.yaml"))
	if err != nil {
		return nil, fmt.Errorf("unable to open problem.yaml: %w", err)
	}
	defer file.Close()

	spec := &Specification{}

	dec := yaml.NewDecoder(file)
	if err := dec.Decode(spec); err != nil {
		return nil, fmt.Errorf("unable to decode problem.yaml: %w", err)
	}

	p.log.Printf("File problem.yaml successfully parsed")

	// import
	checker, err := p.checker(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("unable to read checker configuration: %w", err)
	}

	validator, err := p.validator(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("unable to read validator configuration: %w", err)
	}

	interactor := &atlaspb.Interactor{}

	statements, err := p.statements(ctx, path, spec)
	if err != nil {
		return nil, fmt.Errorf("unable to read statements: %w", err)
	}

	templates := []*atlaspb.Template{}

	attachments, err := p.attachments(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("unable to read attachments (materials): %w", err)
	}

	testsets, tests, err := p.testing(ctx, path, spec)
	if err != nil {
		return nil, fmt.Errorf("unable to read tests: %w", err)
	}

	editorials, err := p.editorials(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("unable to read tutorials: %w", err)
	}

	solutions, err := p.solutions(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("unable to read solutions: %w", err)
	}

	scripts, err := p.scripts(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("unable to read solutions: %w", err)
	}

	return &atlaspb.Snapshot{
		Problem:     &atlaspb.Problem{Topics: TopicsFromTags(spec.Keywords), Type: atlaspb.Problem_PROGRAM},
		Testing:     &atlaspb.TestingConfig{},
		Checker:     checker,
		Validator:   validator,
		Interactor:  interactor,
		Statements:  statements,
		Templates:   templates,
		Attachments: attachments,
		Testsets:    testsets,
		Tests:       tests,
		Editorials:  editorials,
		Solutions:   solutions,
		Scripts:     scripts,
	}, nil
}

func (p *ProblemLoader) download(ctx context.Context, path string, link string) error {
	origin, err := url.Parse(link)
	if err != nil {
		return fmt.Errorf("invalid problem origin: %w", err)
	}
	return p.download_by_link(ctx, path, origin)
}

// fetches ANY public .zip URL and stores it as <path>/problem.zip.
func (p *ProblemLoader) download_by_link(ctx context.Context, path string, link *url.URL) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link.String(), nil)
	if err != nil {
		return fmt.Errorf("compose GET request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK: // fine
	case http.StatusNotFound:
		return fmt.Errorf("link %q returned 404 (file not found)", link.String())
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("link %q requires credentials", link.String())
	default:
		return fmt.Errorf("link %q: unexpected HTTP %d", link.String(), resp.StatusCode)
	}

	// validate that we're downloading a ZIp
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		kind, _, _ := mime.ParseMediaType(ct)
		switch strings.ToLower(kind) {
		case "application/zip", "application/octet-stream", "application/x-zip-compressed":
			// ok
		default:
			if !strings.HasSuffix(strings.ToLower(link.Path), ".zip") {
				return fmt.Errorf("link %q does not appear to be a ZIP (Content-Type %q)", link.String(), ct)
			}
		}
	}

	// <path>/problem.zip.
	dstPath := filepath.Join(path, "problem.zip")
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create local archive: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("write local archive: %w", err)
	}

	return nil
}

// unpack problem archive
func (p *ProblemLoader) unpack(ctx context.Context, path string) error {
	reader, err := zip.OpenReader(filepath.Join(path, "problem.zip"))
	if err != nil {
		return err
	}

	defer reader.Close()

	for _, file := range reader.File {
		file := file

		err := func() error {
			// clean up file path
			name := strings.TrimPrefix(filepath.Clean(filepath.Join("/", file.Name)), string([]rune{filepath.Separator}))
			fpath := filepath.Join(path, name)

			if file.FileInfo().IsDir() {
				if err := os.MkdirAll(fpath, 0777); err != nil && !os.IsExist(err) {
					return fmt.Errorf("unable to create folder %#v: %w", name, err)
				}

				return nil
			}

			if err := os.MkdirAll(filepath.Dir(fpath), 0777); err != nil && !os.IsExist(err) {
				return fmt.Errorf("unable to create folder %#v: %w", filepath.Dir(name), err)
			}

			sf, err := file.Open()
			if err != nil {
				return fmt.Errorf("unable to open %#v for reading: %w", name, err)
			}

			defer sf.Close()

			df, err := os.Create(fpath)
			if err != nil {
				return fmt.Errorf("unable to open %#v for writing: %w", name, err)
			}

			defer df.Close()

			if _, err = io.Copy(df, sf); err != nil {
				return fmt.Errorf("unable to write %#v: %w", name, err)
			}

			return nil
		}()

		if err != nil {
			return err
		}
	}

	return nil
}

// cleanup after import
func (p *ProblemLoader) cleanup(path string) {
	if err := os.RemoveAll(path); err != nil {
		p.log.Errorf("Unable to cleanup workspace path: %v", err)
	}
}

// Output Validator
//  1. if output_validator is missing - return the defautl checker (precision 0, case-insensitive).
//  2. otherwise scan every file in the directory
//  3. if no mappable program file is found fall back to the default checker
func (p *ProblemLoader) checker(ctx context.Context, path string) (*atlaspb.Checker, error) {
	valDir := filepath.Join(path, "output_validator")

	entries, err := os.ReadDir(valDir)
	if err != nil {
		// default checker
		if os.IsNotExist(err) {
			p.log.Printf("No output_validator/ provided – using default token checker")
			return &atlaspb.Checker{Type: executorpb.Checker_TOKENS, Precision: 0, CaseSensitive: false}, nil
		}
		return nil, err
	}

	// ext to lang
	extToLang := map[string]string{}
	for lang, ext := range LanguageExtensions {
		extToLang[ext] = lang
	}

	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}

		name := ent.Name()
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")

		lang, ok := extToLang[ext]
		if !ok {
			p.log.Printf("Unknown validator extension .%s – skipping %q", ext, name)
			continue
		}

		runtime, ok := RuntimeMapping[lang]
		if !ok {
			p.log.Printf("No runtime mapping for language %s – skipping %q", lang, name)
			continue
		}

		// read main source
		mainPath := filepath.Join(valDir, name)
		code, rErr := os.ReadFile(mainPath)
		if rErr != nil {
			return nil, rErr
		}

		// get helper files
		var files []*executorpb.File
		for _, extra := range entries {
			if extra.IsDir() || extra.Name() == name {
				continue
			}
			// add .h files for c/c++
			if (lang == "c" || lang == "cpp") && filepath.Ext(extra.Name()) == ".h" {
				url, upErr := p.upload.UploadFile(ctx, filepath.Join(valDir, extra.Name()))
				if upErr != nil {
					p.log.Errorf("Unable to upload validator helper %q: %v", extra.Name(), upErr)
					continue
				}
				files = append(files, &executorpb.File{Path: extra.Name(), SourceUrl: url})
			}
		}

		p.log.Printf("Adding program checker %q (%s)", name, runtime)

		return &atlaspb.Checker{Type: executorpb.Checker_PROGRAM, Runtime: runtime, Source: string(code), Files: files}, nil
	}

	p.log.Printf("No program-style validator found – using default token checker")
	return &atlaspb.Checker{Type: executorpb.Checker_TOKENS, Precision: 0, CaseSensitive: false}, nil
}

// Input validator
func (p *ProblemLoader) validator(ctx context.Context, path string) (*atlaspb.Validator, error) {
	valDir := filepath.Join(path, "input_validators")

	entries, err := os.ReadDir(valDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	extToLang := map[string]string{}
	for lang, ext := range LanguageExtensions {
		extToLang[ext] = lang
	}

	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}

		name := ent.Name()
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		lang, ok := extToLang[ext]
		if !ok {
			continue
		}

		runtime, ok := RuntimeMapping[lang]
		if !ok {
			p.log.Printf("Skipping validator %q: no runtime for %s", name, lang)
			continue
		}

		mainPath := filepath.Join(valDir, name)
		code, rErr := os.ReadFile(mainPath)
		if rErr != nil {
			return nil, rErr
		}

		var files []*executorpb.File
		for _, extra := range entries {
			if extra.IsDir() || extra.Name() == name {
				continue
			}
			// add .h files for c/c++
			if (lang == "c" || lang == "cpp") && filepath.Ext(extra.Name()) == ".h" {
				url, upErr := p.upload.UploadFile(ctx, filepath.Join(valDir, extra.Name()))
				if upErr != nil {
					p.log.Errorf("Unable to upload validator helper %q: %v", extra.Name(), upErr)
					continue
				}
				files = append(files, &executorpb.File{
					Path:      extra.Name(),
					SourceUrl: url,
				})
			}
		}

		p.log.Printf("Adding program validator %q (%s)", name, runtime)

		return &atlaspb.Validator{Runtime: runtime, Source: string(code), Files: files}, nil
	}

	return nil, nil
}

// scans <path>/statement/ and converts every file it finds
func (p *ProblemLoader) statements(ctx context.Context, path string, spec *Specification) (stmts []*atlaspb.Statement, err error) {
	dir := filepath.Join(path, "statement")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("statement/: %w", err)
	}

	title := ""
	switch {
	case spec.Name.One != "":
		title = spec.Name.One
	case spec.Name.Map != nil:
		if v, ok := spec.Name.Map["en"]; ok {
			title = v
		} else {
			for _, v := range spec.Name.Map {
				title = v
				break
			}
		}
	default:
		title = "Problem"
	}

	author := ""

	pick := func(pl PersonList) string { // helper to pick first
		if len(pl) == 0 {
			return ""
		}
		if pl[0].Name != "" {
			return pl[0].Name
		}
		return pl[0].Source
	}

	switch {
	case len(spec.Credits.Authors) > 0:
		author = pick(spec.Credits.Authors)
	case len(spec.Credits.Contributors) > 0:
		author = pick(spec.Credits.Contributors)
	case len(spec.Credits.Packagers) > 0:
		author = pick(spec.Credits.Packagers)
	case len(spec.Credits.Testers) > 0:
		author = pick(spec.Credits.Testers)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))

		// lang detection
		locale := "en"
		if dot := strings.Index(name, "."); dot == 2 {
			locale = strings.ToLower(name[:2])
		}

		// read file
		path := filepath.Join(dir, name)
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil, rerr
		}

		var content *ecmpb.Content
		switch ext {
		case ".md", ".markdown":
			content = &ecmpb.Content{Value: &ecmpb.Content_Markdown{Markdown: string(raw)}}
		case ".html", ".htm":
			content = &ecmpb.Content{Value: &ecmpb.Content_Html{Html: string(raw)}}
		case ".tex":
			content = &ecmpb.Content{Value: &ecmpb.Content_Latex{Latex: string(raw)}}
		case ".pdf":
			return nil, errors.New("pdf statements are not supported/")
		default:
			// unknown extension
			continue
		}

		stmts = append(stmts, &atlaspb.Statement{
			Locale:  locale,
			Title:   title,
			Author:  author,
			Content: content,
		})
	}

	if len(stmts) == 0 {
		return nil, errors.New("no usable statement files found in statement/")
	}
	return stmts, nil
}

func (p *ProblemLoader) attachments(ctx context.Context, path string) (attachments []*atlaspb.Attachment, err error) {
	root := filepath.Join(path, "attachments")

	// directory does not exist
	if _, statErr := os.Stat(root); os.IsNotExist(statErr) {
		return nil, nil
	}

	walkErr := filepath.WalkDir(root, func(fp string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		// read file
		data, rErr := os.ReadFile(fp)
		if rErr != nil {
			p.log.Errorf("unable to read attachment %q: %v", fp, rErr)
			return nil
		}

		name := filepath.Base(fp) // original filename

		// upload
		asset, upErr := p.upload.UploadAsset(ctx, &assetpb.UploadAssetInput{Name: name, Data: data})
		if upErr != nil {
			p.log.Errorf("unable to upload attachment %q: %v", fp, upErr)
			return nil
		}

		attachments = append(attachments, &atlaspb.Attachment{Name: name, Link: asset.GetAssetUrl()})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return attachments, nil
}

type groupYAML struct {
	FullFeedback bool                   `yaml:"full_feedback"`
	Scoring      map[string]interface{} `yaml:"scoring"` // "mode" or "aggregate"
}

// data/sample/               - sample cases
// data/secret/*.in           - main tests
func (p *ProblemLoader) testing(ctx context.Context, path string, spec *Specification) (testsets []*atlaspb.Testset, tests []*atlaspb.Test, err error) {
	dataDir := filepath.Join(path, "data")
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(5)

	// samples
	sampleDir := filepath.Join(dataDir, "sample")
	if dirExists(sampleDir) {
		set := newSet(0, "sample", true, spec.Limits)
		testsets = append(testsets, set)
	}

	// secret groups
	secretDir := filepath.Join(dataDir, "secret")
	if !dirExists(secretDir) {
		return nil, nil, fmt.Errorf("data/secret directory is required")
	}

	entries, _ := os.ReadDir(secretDir)
	subDirs := make([]os.DirEntry, 0)
	for _, e := range entries {
		if e.IsDir() {
			subDirs = append(subDirs, e)
		}
	}

	nextSetIdx := 1
	processDir := func(dirPath, groupName string) error {
		// read test_group.yaml if present
		cfg := groupYAML{}
		cfgPath := filepath.Join(dirPath, "test_group.yaml")
		if fileExists(cfgPath) {
			raw, rErr := os.ReadFile(cfgPath)
			if rErr != nil {
				return rErr
			}
			if err := yaml.Unmarshal(raw, &cfg); err != nil {
				return err
			}
		}

		// build testset and apply yaml overrides
		set := newSet(nextSetIdx, groupName, false, spec.Limits)
		nextSetIdx++

		if cfg.FullFeedback {
			set.FeedbackPolicy = atlaspb.FeedbackPolicy_COMPLETE
		}
		if mode, ok := cfg.Scoring["mode"]; ok && mode == "min" {
			set.ScoringMode = atlaspb.ScoringMode_WORST
		}
		if agg, ok := cfg.Scoring["aggregate"]; ok && agg == "min" {
			set.ScoringMode = atlaspb.ScoringMode_WORST
		}

		testsets = append(testsets, set)

		// .in files
		idx := 1
		return filepath.WalkDir(dirPath, func(fp string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() || filepath.Ext(d.Name()) != ".in" {
				return walkErr
			}
			base := strings.TrimSuffix(fp, ".in")
			inPath, ansPath := fp, base+".ans"

			test := &atlaspb.Test{
				TestsetId: set.GetId(),
				Index:     int32(idx),
			}
			idx++

			eg.Go(func() error {
				url, err := p.upload.UploadFile(ctx, inPath)
				test.Input = &atlaspb.Test_InputUrl{InputUrl: url}
				return err
			})

			if fileExists(ansPath) {
				eg.Go(func() error {
					url, err := p.upload.UploadFile(ctx, ansPath)
					test.Answer = &atlaspb.Test_AnswerUrl{AnswerUrl: url}
					return err
				})
			} else {
				test.Answer = &atlaspb.Test_AnswerGenerator{
					AnswerGenerator: &atlaspb.Test_Generator{ScriptName: "solution"},
				}
			}

			tests = append(tests, test)
			return nil
		})
	}

	if len(subDirs) == 0 {
		if err := processDir(secretDir, "secret"); err != nil {
			return nil, nil, err
		}
	} else {
		for _, dir := range subDirs {
			if err := processDir(filepath.Join(secretDir, dir.Name()), dir.Name()); err != nil {
				return nil, nil, err
			}
		}
	}

	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}

	var total float32
	for _, t := range tests {
		total += t.GetScore()
	}
	if total == 0 {
		credit := 100.0
		for i := range tests {
			score := float32(math.Floor(credit / float64(len(tests)-i)))
			tests[i].Score = score
			credit -= float64(score)
		}
	}
	return testsets, tests, nil
}

// does dir exist
func dirExists(p string) bool {
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return true
	}
	return false
}

// helper to make a new set
func newSet(idx int, name string, sample bool, lim Limits) *atlaspb.Testset {
	ms := uint32(lim.TimeLimit * 1000)
	mem := uint64(lim.Memory) << 20
	fs := uint64(lim.OutputLimit) << 20
	if fs == 0 {
		fs = 512 << 20
	}

	ts := &atlaspb.Testset{
		Id:            uuid.New().String(),
		Index:         uint32(idx),
		CpuLimit:      ms,
		MemoryLimit:   mem,
		FileSizeLimit: fs,
	}
	if sample {
		ts.ScoringMode = atlaspb.ScoringMode_EACH
		ts.FeedbackPolicy = atlaspb.FeedbackPolicy_COMPLETE
	} else {
		ts.ScoringMode = atlaspb.ScoringMode_ALL
		ts.FeedbackPolicy = atlaspb.FeedbackPolicy_ICPC_EXPANDED
	}
	return ts
}

func (p *ProblemLoader) editorials(ctx context.Context, path string) (editorials []*atlaspb.Editorial, err error) {
	solDir := filepath.Join(path, "solution")
	entries, err := os.ReadDir(solDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("solution/: %w", err)
	}

	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}

		name := ent.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".tex" {
			p.log.Printf("Skipping solution %q (unsupported extension)", name)
			continue // we only consider LaTeX
		}

		// solution.<lang>.tex = lang = <lang>
		// solution.tex        = lang = "en"
		locale := "en"
		if parts := strings.SplitN(name, ".", 3); len(parts) == 3 {
			lang := parts[1]
			var convErr error
			locale, convErr = LocaleFromLanguage(lang)
			if convErr != nil {
				p.log.Printf("Skipping solution %q: %v", name, convErr)
				continue
			}
		}

		fp := filepath.Join(solDir, name)
		data, rerr := os.ReadFile(fp)
		if rerr != nil {
			p.log.Errorf("Unable to read solution %q: %v", name, rerr)
			continue
		}

		latex := p.uploadImagesFromLatex(ctx, solDir, string(data))

		editorials = append(editorials, &atlaspb.Editorial{
			Locale:  locale,
			Content: &ecmpb.Content{Value: &ecmpb.Content_Latex{Latex: latex}},
		})
	}

	return editorials, nil
}

func (p *ProblemLoader) scripts(ctx context.Context, path string) (scripts []*atlaspb.Script, err error) {
	genDir := filepath.Join(path, "generators")

	if _, statErr := os.Stat(genDir); os.IsNotExist(statErr) {
		return nil, nil
	}

	extToLang := map[string]string{}
	for lang, ext := range LanguageExtensions {
		extToLang[ext] = lang
	}

	walkErr := filepath.WalkDir(genDir, func(fp string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		switch strings.ToLower(filepath.Ext(d.Name())) {
		case ".txt", ".md":
			return nil
		}

		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(d.Name())), ".")
		lang, ok := extToLang[ext]
		if !ok {
			p.log.Printf("Skipping generator %q – unknown extension .%s", d.Name(), ext)
			return nil
		}

		runtime, ok := RuntimeMapping[lang]
		if !ok {
			p.log.Printf("Skipping generator %q – no runtime for lang %s", d.Name(), lang)
			return nil
		}

		// main source
		mainSrc, rErr := os.ReadFile(fp)
		if rErr != nil {
			p.log.Errorf("Unable to read generator %q: %v", fp, rErr)
			return nil
		}

		// helper header
		var files []*executorpb.File
		if lang == "c" || lang == "cpp" {
			dirEntries, _ := os.ReadDir(genDir)
			for _, e := range dirEntries {
				if e.IsDir() || filepath.Ext(e.Name()) != ".h" {
					continue
				}
				hPath := filepath.Join(genDir, e.Name())
				url, upErr := p.upload.UploadFile(ctx, hPath)
				if upErr != nil {
					p.log.Errorf("Unable to upload generator helper %q: %v", hPath, upErr)
					continue
				}
				files = append(files, &executorpb.File{
					Path:      e.Name(),
					SourceUrl: url,
				})
			}
		}

		scripts = append(scripts, &atlaspb.Script{
			Name:    strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())),
			Runtime: runtime,
			Source:  string(mainSrc),
			Files:   files,
		})
		return nil
	})

	if walkErr != nil {
		return nil, walkErr
	}
	return scripts, nil
}

func (p *ProblemLoader) solutions(ctx context.Context, path string) (solutions []*atlaspb.Solution, err error) {
	subDir := filepath.Join(path, "submissions")
	if _, statErr := os.Stat(subDir); os.IsNotExist(statErr) {
		return nil, nil
	}

	extToLang := map[string]string{}
	for lang, ext := range LanguageExtensions {
		extToLang[ext] = lang
	}

	verdictMap := map[string]atlaspb.Solution_Type{
		"accepted":                        atlaspb.Solution_CORRECT,
		"reference":                       atlaspb.Solution_CORRECT,
		"wrong_answer":                    atlaspb.Solution_WRONG_ANSWER,
		"rejected":                        atlaspb.Solution_INCORRECT,
		"time_limit_exceeded":             atlaspb.Solution_TIMEOUT,
		"time_limit_exceeded_or_accepted": atlaspb.Solution_TIMEOUT_OR_ACCEPTED,
		"memory_limit_exceeded":           atlaspb.Solution_OVERFLOW,
		"runtime_error":                   atlaspb.Solution_FAILURE,
		"failed":                          atlaspb.Solution_FAILURE,
	}

	walkErr := filepath.WalkDir(subDir, func(fp string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		verdict := filepath.Base(filepath.Dir(fp))
		kind, ok := verdictMap[verdict]
		if !ok {
			kind = atlaspb.Solution_DONT_RUN
		}

		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fp)), ".")
		lang, ok := extToLang[ext]
		if !ok {
			p.log.Printf("Skipping submission %q: unmapped extension .%s", fp, ext)
			return nil
		}

		runtime, ok := RuntimeMapping[lang]
		if !ok {
			p.log.Printf("Skipping submission %q: no runtime for language %s", fp, lang)
			return nil
		}

		data, rErr := os.ReadFile(fp)
		if rErr != nil {
			p.log.Errorf("Unable to read submission %q: %v", fp, rErr)
			return nil
		}

		solutions = append(solutions, &atlaspb.Solution{
			Name:    filepath.Base(fp),
			Runtime: runtime,
			Source:  string(data),
			Type:    kind,
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return solutions, nil
}

// uploadImagesFromLatex finds images in text, uploads them and replaces original names with links.
// e.g. \includegraphics[width=12cm]{myimage.png} -> \includegraphics[width=12cm]{https://...}
func (p *ProblemLoader) uploadImagesFromLatex(ctx context.Context, path, text string) string {
	images := imageFinder.FindAllStringSubmatch(text, -1)

	replaced := map[string]bool{}
	for _, image := range images {
		if want, got := 4, len(image); want != got {
			p.log.Errorf("Unable to parse \\includegraphics parameters")
			continue
		}

		full := image[0]
		prefix := image[1]
		name := image[2]
		suffix := image[3]

		if _, ok := replaced[full]; ok {
			continue
		}

		data, err := os.ReadFile(filepath.Join(path, name))
		if err != nil {
			p.log.Errorf("Unable to read image %#v: %v", name, err)
			continue
		}

		asset, err := p.upload.UploadAsset(ctx, &assetpb.UploadAssetInput{Name: name, Data: data})
		if err != nil {
			p.log.Errorf("Unable to upload image %#v: %v", name, err)
			continue
		}

		p.log.Printf("Image %#v is uploaded to %#v", name, asset.GetAssetUrl())

		text = strings.Replace(text, full, prefix+asset.GetAssetUrl()+suffix, -1)
	}

	return text
}
