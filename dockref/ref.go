package dockref

import (
	_ "crypto/sha256" // side effect: register sha256
	"fmt"
	"github.com/blang/semver"
	"github.com/docker/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sort"
	"strings"
)

type Reference interface {
	Name() string
	Tag() string
	DigestString() string
	Digest() digest.Digest
	Original() string
	Domain() string
	Path() string
	Named() reference.Named
	Format() Format
	Formatted() string
	String() string
	WithRequestedFormat(format Format) (Reference, error)
	WithDigest(dig string) Reference
	WithTag(tag string) Reference
}

func ParseAlgoDigest(algoDigest string) (ref Reference, e error) {
	dig := digest.Digest(algoDigest)
	err := dig.Validate()
	if err != nil {
		return nil, err
	}

	hex := dig.Hex()
	return MustParse(hex), nil
}

func MustParseAlgoDigest(algoDigest string) Reference {
	ref, e := ParseAlgoDigest(algoDigest)
	deliberatelyUnsued(e)
	return ref
}

func Parse(original string) (ref Reference, e error) {
	r, e := reference.ParseAnyReference(original)
	if e != nil {
		return
	}

	var name string
	var domain string
	var path string
	var named reference.Named
	var ok bool
	if named, ok = r.(reference.Named); ok {
		name = named.Name()
		domain = reference.Domain(named)
		path = reference.Path(named)
	}

	var tag string
	if tagged, ok := r.(reference.Tagged); ok {
		tag = tagged.Tag()
	}

	var dig string
	if digested, ok := r.(reference.Digested); ok {
		dig = string(digested.Digest())
	}

	ref = dockref{
		original: original,
		domain:   domain,
		name:     name,
		tag:      tag,
		digest:   dig,
		path:     path,
		named:    named,
		format:   findDockrefFormat(named, original, name, tag, dig),
	}

	return
}

// MustParse same functionallity as Parse, but hides errors.
// Use this function only when you know that the input cannot have an error
func MustParse(original string) Reference {
	ref, e := Parse(original)
	deliberatelyUnsued(e)

	return ref
}

type Format uint

const (
	FormatHasName   Format = 1 << iota
	FormatHasTag    Format = 1 << iota
	FormatHasDomain Format = 1 << iota
	FormatHasDigest Format = 1 << iota
)

func (format Format) hasName() bool {
	return format&FormatHasName != 0
}

func (format Format) hasTag() bool {
	return format&FormatHasTag != 0
}
func (format Format) hasDomain() bool {
	return format&FormatHasDomain != 0
}
func (format Format) hasDigest() bool {
	return format&FormatHasDigest != 0
}
func (format Format) Valid() (bool, error) {
	f := format
	f &= ^(FormatHasName | FormatHasTag | FormatHasDomain | FormatHasDigest)
	valid := f == 0
	var err error
	if !valid {
		err = fmt.Errorf("Invalid format, %d", format)
	}
	return valid, err
}

var _ Reference = (*dockref)(nil)

type dockref struct {
	name     string
	original string
	tag      string
	digest   string
	domain   string
	path     string
	named    reference.Named
	format   Format
}

func findDockrefFormat(named reference.Reference, original, name, tag, digestString string) Format {
	var format Format

	if named != nil {
		fn := reference.FamiliarString(named)
		if fn != original {
			format |= FormatHasDomain
		}
	}

	if name != "" {
		format |= FormatHasName
	}
	if tag != "" {
		format |= FormatHasTag
	}
	if digestString != "" {
		format |= FormatHasDigest
	}

	return format
}

func (r dockref) Format() Format {
	return r.format
}

func (r dockref) Formatted() string {

	s := ""

	var name string

	format := r.Format()

	if format.hasName() {
		if format.hasDomain() {
			name = r.name
		} else {
			name = reference.FamiliarName(r.named)
		}
		s += name
	}

	if format.hasTag() {
		s += ":" + r.tag
	}

	if format.hasDigest() {
		if format.hasName() {
			s += "@" + r.DigestString()
		} else {
			s += r.Digest().Hex()
		}
	}

	return s
}

func (r dockref) Named() reference.Named {
	return r.named
}

func (r dockref) Name() string {
	return r.name
}

func (r dockref) Tag() string {
	return r.tag
}

func (r dockref) DigestString() string {
	return r.digest
}

func (r dockref) Digest() digest.Digest {
	return digest.Digest(r.digest)
}

func (r dockref) Original() string {
	return r.original
}

func (r dockref) Domain() string {
	return r.domain
}

func (r dockref) Path() string {
	return r.path
}

func (r dockref) String() string {
	return r.Formatted()
}

func (r dockref) WithRequestedFormat(format Format) (Reference, error) {
	if ok, err := format.Valid(); !ok {
		return nil, err
	}
	var required Format

	if r.Domain() != "" && r.Domain() != "docker.io" {
		required |= FormatHasDomain | FormatHasName
	}

	if format.hasTag() {
		required |= FormatHasName
	}

	cpy := r
	cpy.format = format | required

	if r.Format() == FormatHasDigest { // digest-only
		cpy.format = FormatHasDigest
	}

	return cpy, nil
}

func (r dockref) WithDigest(dig string) Reference {
	cpy := r
	cpy.digest = dig
	return cpy
}

func (r dockref) WithTag(tag string) Reference {
	cpy := r
	cpy.tag = tag
	return cpy
}

func FindRelevantTagsForReference(ref Reference, refs []Reference, log *logrus.Logger) ([]Reference, error) {
	// name and domain must match
	sameName := make([]Reference, 0)
	for _, r := range refs {
		if ref.Name() == r.Name() {
			sameName = append(sameName, r)
		}
	}

	_, refVariant := splitVersionAndVariant(ref.Tag())
	sameVariant := make([]Reference, 0)
	for _, r := range sameName {
		_, rVariant := splitVersionAndVariant(r.Tag())

		if refVariant == rVariant {
			sameVariant = append(sameVariant, r)
		}
	}

	return sameVariant, nil
}

func MostPreciseTag(refs []Reference, log *logrus.Logger) (Reference, error) {
	if refs == nil {
		return nil, errors.New("refs is nil")
	}
	seen := make(map[string]struct{}, len(refs))
	unique := make([]Reference, 0)
	for _, r := range refs {
		if r == nil {
			return nil, errors.New("refs contains nil element")
		}

		tag := r.Tag()
		if _, ok := seen[tag]; !ok {
			seen[tag] = struct{}{}
			unique = append(unique, r)
		}
	}
	refs = unique

	if len(refs) == 1 {
		return refs[0], nil
	}

	nonSemVer, best := bestSemVer(refs)

	if best != nil {
		return best, nil
	}

	// look for best non semver

	// remove empty tag
	nonEmpty := removeEmpty(nonSemVer)
	if len(nonEmpty) == 1 {
		return nonEmpty[0], nil
	}

	nonLatest := removeLatest(nonEmpty)
	if len(nonLatest) == 1 {
		return nonLatest[0], nil
	}

	if log != nil {
		log.Warnf("Didn't find semantic versioning tags, still trying to choose best tag but your mileage might vary")
	}

	// take longest

	longest := filterNonLongest(nonLatest)

	// alphabetic
	sort.Slice(longest, func(i, j int) bool {
		a := longest[i]
		b := longest[j]
		return strings.Compare(a.Tag(), b.Tag()) > 0
	})

	best = longest[0]
	return best, nil
}

func filterNonLongest(nonLatest []Reference) []Reference {
	sort.Slice(nonLatest, func(i, j int) bool {
		a := nonLatest[i]
		b := nonLatest[j]

		return len(a.Tag()) > len(b.Tag())
	})
	maxLen := len(nonLatest[0].Tag())
	longest := make([]Reference, 0)
	for _, r := range nonLatest {
		if len(r.Tag()) == maxLen {
			longest = append(longest, r)
		}
	}
	return longest
}

func removeLatest(refs []Reference) []Reference {
	nonLatest := make([]Reference, 0)
	for _, r := range refs {
		if r.Tag() == "latest" {
			continue
		}
		nonLatest = append(nonLatest, r)
	}
	return nonLatest
}

func removeEmpty(refs []Reference) []Reference {
	nonEmpty := make([]Reference, 0)
	for _, r := range refs {
		if r.Tag() == "" {
			continue
		}
		nonEmpty = append(nonEmpty, r)
	}
	return nonEmpty
}

func bestSemVer(refs []Reference) ([]Reference, Reference) {
	var best Reference
	var bestSemver semver.Version
	nonSemVer := make([]Reference, 0)
	// look for semver first, semver wins
	for _, r := range refs {
		tag := r.Tag()
		version, e := parseVeryTolerant(tag)
		if e != nil {
			nonSemVer = append(nonSemVer, r)
			continue
		}
		if version.Compare(bestSemver) >= 0 {
			bestSemver = version
			best = r
		}
	}
	return nonSemVer, best
}

func splitVersionAndVariant(tag string) (version string, variant string) {
	lastIndex := strings.Index(tag, "-")
	if lastIndex >= 0 {
		version = tag[0:lastIndex]
		variant = tag[lastIndex+1:]
		_, e := semver.ParseTolerant(version)
		if e != nil {
			version = ""
			variant = tag
			return
		}
	} else {
		// no variant
		_, e := semver.ParseTolerant(tag)
		if e != nil {
			version = ""
			variant = tag
			return
		}
		version = tag
		variant = ""
		return
	}

	return
}

func parseVeryTolerant(tag string) (semver.Version, error) {
	version, variant := splitVersionAndVariant(tag)

	parsed, e := semver.ParseTolerant(version)
	if e != nil {
		return parsed, e
	}
	if variant != "" {
		str := parsed.String()
		version = str + "-" + variant
	}

	return semver.ParseTolerant(version)
}

func deliberatelyUnsued(err error) {
	// noop
}
