package versionutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	fixtureVersion730    = "7.3.0"
	fixtureVersion751    = "7.5.1"
	fixturePC2024101     = "2024.1.0.1"
	fixtureVersion651    = "6.5.1"
	fixtureVersion76     = "7.6"
	fixtureVersion7605   = "7.6.0.5"
	fixtureMasterVersion = "master"
	fixtureFull751       = "el8.5-release-ganges-7.5.1-stable-b2a0591e15d312a5888e2eee14e1c4bffc3e3b3d"
	fixtureFull76        = "el9-release-ganges-7.6-stable-abc123"
	fixtureFull7512      = "el8.5-release-ganges-7.5.1.2-stable-abc123"
	fixtureVersion7512   = "7.5.1.2"
	fixtureFull681       = "el7-release-fraser-6.8.1-stable-abc123"
	fixtureVersion681    = "6.8.1"
	fixtureFull75        = "el8.5-release-ganges-7.5-stable-abc123"
	fixtureVersion75     = "7.5"
	fixtureFullMaster    = "el9-opt-master-56c4af6e0fb9c6a2aed0e517cbfb3768cde74311"
	fixtureBuild751      = "build-ganges-7.5.1-abc123"
	fixtureVersion3521   = "3.5.2.1"
	fixtureVersion760    = "7.6.0"
	fixtureVersion761    = "7.6.1"
	fixtureVersion750    = "7.5.0"
	fixtureUpperMaster   = "MASTER"
	fixturePC20243       = "2024.3"
	fixtureVersion99     = "9.9"
	fixturePC7605        = "pc.7.6.0.5"
	fixtureReuseCount    = 3
)

func TestParseNormalizes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercases and trims", "  PC.7.3.0  ", fixtureVersion730},
		{"handles missing prefix", fixtureVersion751, fixtureVersion751},
		{"handles uppercase PC prefix", "Pc.2024.1.0.1", fixturePC2024101},
		{"strips only a leading prefix", "pc.pc.7.3", "pc.7.3"},
		{"leaves an empty version empty", "   ", ""},
		{"leaves an AOS version alone", fixtureVersion651, fixtureVersion651},
		{"strips a v prefix", "v7.6", fixtureVersion76},
		{"strips an uppercase V prefix", "V7.6", fixtureVersion76},
		{"strips a v prefix after pc.", "pc.v7.6.0.5", fixtureVersion7605},
		{"leaves master alone", fixtureMasterVersion, fixtureMasterVersion},
		{"extracts AOS from Prism fullVersion", fixtureFull751, fixtureVersion751},
		{"extracts AOS from a non-el OS prefix", "ol8.5-release-ganges-7.5.1-stable-b2a0591e15d312a5888e2eee14e1c4bffc3e3b3d", fixtureVersion751},
		{"extracts AOS 7.6 from Prism fullVersion", fixtureFull76, fixtureVersion76},
		{"extracts AOS when there is no OS prefix", "release-ganges-7.5.1-stable-abc123", fixtureVersion751},
		{"extracts a four-component AOS token", fixtureFull7512, fixtureVersion7512},
		{"extracts AOS 6.x", fixtureFull681, fixtureVersion681},
		{"fullVersion 7.5 stays 7.5", fixtureFull75, fixtureVersion75},
		{"trims whitespace around fullVersion", "  el8.5-release-ganges-7.5.1-stable-abc  ", fixtureVersion751},
		{"internal opt-master fullVersion stays unknown", fixtureFullMaster, fixtureFullMaster},
		{
			"dotted OS prefix without AOS stays unknown",
			"el8.5-opt-master-56c4af6e0fb9c6a2aed0e517cbfb3768cde74311", "el8.5-opt-master-56c4af6e0fb9c6a2aed0e517cbfb3768cde74311",
		},
		{"uppercased fullVersion still extracts AOS", "EL8.5-RELEASE-GANGES-7.5.1-STABLE-ABC123", fixtureVersion751},
		{"generic OS letters are not limited to el/ol", "centos8.5-release-ganges-7.5.1-stable-abc123", fixtureVersion751},
		{"an extra kernel-style token is not preferred over AOS", "el8.5-5.10.0-release-ganges-7.5.1-stable-abc123", fixtureVersion751},
		{"a dotted token without OS prefix or -stable- is left unknown", fixtureBuild751, fixtureBuild751},
		{"a single-component AOS token is not extracted", "el9-release-ganges-7-stable-abc123", "el9-release-ganges-7-stable-abc123"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.expected, Parse(test.input).String())
		})
	}
}

func TestCompare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{"equal versions", fixtureVersion3521, fixtureVersion3521, 0},
		{"greater patch", "3.5.2.2", fixtureVersion3521, 1},
		{"lesser patch", fixtureVersion3521, "3.5.2.2", -1},
		{"greater major", "10.0", "9.9.9", 1},
		{"missing components are zero", fixtureVersion76, fixtureVersion760, 0},
		{"missing components are zero, reversed", fixtureVersion760, fixtureVersion76, 0},
		{"trailing component breaks the tie", fixtureVersion761, fixtureVersion76, 1},
		{"master sorts newest", fixtureMasterVersion, fixtureVersion750, 1},
		{"master sorts newest, reversed", fixtureVersion750, fixtureMasterVersion, -1},
		{"master is case insensitive", fixtureUpperMaster, fixtureVersion750, 1},
		{"unparsable sorts newest", "not-a-version", fixtureVersion750, 1},
		{"empty sorts newest", "", fixtureVersion750, 1},
		{"two unparsable versions are equal", fixtureMasterVersion, "", 0},
		{"7.x beats 202x", fixtureVersion730, fixturePC2024101, 1},
		{"202x loses to 7.x", fixturePC2024101, fixtureVersion730, -1},
		{"pc prefix is ignored", fixturePC7605, fixtureVersion7605, 0},
		{"pc prefix is ignored across lines", "PC.7.3", "pc.2024.3.1", 1},
		{"two calendar versions compare numerically", "2024.3.1", "2023.4", 1},
		{"AOS versions compare numerically", fixtureVersion681, "6.5.3.1", 1},
		{"AOS 7.x compares numerically", "7.0", "6.8", 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.expected, Parse(test.v1).compare(Parse(test.v2)))
		})
	}
}

func TestAtLeast(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		version    string
		minVersion string
		expected   bool
	}{
		{"equal", fixtureVersion76, fixtureVersion76, true},
		{"greater", fixtureVersion7605, fixtureVersion76, true},
		{"lesser", "7.5.0.5", fixtureVersion76, false},
		{"pc prefix is ignored", fixturePC7605, fixtureVersion76, true},
		{"calendar version does not satisfy a 7.x minimum", fixturePC20243, fixtureVersion76, false},
		{"7.x satisfies a calendar minimum", "7.0", fixturePC20243, true},
		{"missing components are zero", fixtureVersion76, fixtureVersion760, true},
		{"AOS minimum", fixtureVersion681, fixtureVersion651, true},
		{"AOS below minimum", "6.5.0", fixtureVersion651, false},
		{"truncated-looking fullVersion satisfies the patch floor", fixtureFull751, fixtureVersion751, true},
		{"fullVersion 7.5 does not satisfy 7.5.1", fixtureFull75, fixtureVersion751, false},
		{"fullVersion 7.5.1 does not satisfy 7.6", fixtureFull751, fixtureVersion76, false},
		{"fullVersion 7.5.1 satisfies a 7.5 floor", "el8.5-release-ganges-7.5.1-stable-abc123", fixtureVersion75, true},
		{"fullVersion 7.6 satisfies the 7.5.1 floor", fixtureFull76, fixtureVersion751, true},
		{"four-component fullVersion satisfies the patch floor", fixtureFull7512, fixtureVersion751, true},
		{"ol-prefixed fullVersion satisfies the patch floor", "ol8.5-release-ganges-7.5.1-stable-abc123", fixtureVersion751, true},
		{"fullVersion without an OS prefix still satisfies the patch floor", "release-ganges-7.5.1-stable-abc123", fixtureVersion751, true},
		{"AOS 6.x fullVersion does not satisfy 7.5.1", fixtureFull681, fixtureVersion751, false},
		{"AOS 6.x fullVersion satisfies a 6.5.1 floor", fixtureFull681, fixtureVersion651, true},
		{"an extra kernel-style token still satisfies 7.5.1", "el8.5-5.10.0-release-ganges-7.5.1-stable-abc123", fixtureVersion751, true},
		{"internal opt-master fullVersion satisfies every gate", fixtureFullMaster, fixtureVersion751, true},
		{"dotted OS-only fullVersion is unknown and satisfies every gate", "ol8.5-opt-master-56c4af6e0fb9c6a2aed0e517cbfb3768cde74311", fixtureVersion99, true},
		{"a non-fullVersion dotted token stays unknown and satisfies every gate", fixtureBuild751, fixtureVersion99, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.expected, Parse(test.version).AtLeast(Parse(test.minVersion)))
		})
	}
}

// TestVPrefixIsNotUnknown guards the distinction between a version we cannot
// read and one wearing a conventional prefix. "master" is genuinely unknown
// and gates open by design; "v7.6" is 7.6 and must gate like it, rather than
// collapsing to the unparsable sentinel and satisfying every minimum.
func TestVPrefixIsNotUnknown(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"v7.6", "V7.6", "pc.v7.6", "  v7.6  "} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			assert.True(t, Parse(version).AtLeast(Parse(fixtureVersion76)))
			assert.True(t, Parse(version).AtLeast(Parse(fixtureVersion651)))
			assert.False(t, Parse(version).AtLeast(Parse(fixtureVersion99)),
				"a v-prefixed version must not satisfy every minimum")
			assert.False(t, Parse(version).AtLeast(Parse("7.7")))
			assert.Equal(t, 0, Parse(version).compare(Parse(fixtureVersion76)))
		})
	}

	// Stripping the prefix off something still unreadable leaves it
	// unreadable, and so still the newest version.
	assert.True(t, Parse("v-some-branch").AtLeast(Parse(fixtureVersion99)))
	assert.True(t, Parse(fixtureMasterVersion).AtLeast(Parse(fixtureVersion99)))
}

func TestParseReported(t *testing.T) {
	t.Parallel()

	full751 := fixtureFull751
	full75 := fixtureFull75
	full76 := fixtureFull76
	fullMaster := fixtureFullMaster
	osOnlyFull := "ol8.5-opt-master-56c4af6e0fb9c6a2aed0e517cbfb3768cde74311"

	tests := []struct {
		name     string
		short    string
		full     string
		want     string
		atLeast  string
		wantGate bool
	}{
		{"truncated short is refined by fullVersion patch", fixtureVersion75, full751, fixtureVersion751, fixtureVersion751, true},
		{"truncated short with matching 7.5 fullVersion stays below patch floor", fixtureVersion75, full75, fixtureVersion75, fixtureVersion751, false},
		{"short 7.6 with 7.6 fullVersion passes metro floor", fixtureVersion76, full76, fixtureVersion76, fixtureVersion751, true},
		{"short 7.6.1 with no fullVersion passes", fixtureVersion761, "", fixtureVersion761, fixtureVersion751, true},
		{"short 7.5 with no fullVersion stays 7.5", fixtureVersion75, "", fixtureVersion75, fixtureVersion751, false},
		{"empty short uses AOS token from fullVersion", "", full751, fixtureVersion751, fixtureVersion751, true},
		{"empty short and OS-only fullVersion stays unknown", "", osOnlyFull, "", fixtureVersion751, true},
		{"master short is not replaced by a GA fullVersion", fixtureMasterVersion, full751, fixtureMasterVersion, fixtureVersion751, true},
		{"master short with opt-master fullVersion stays master", fixtureMasterVersion, fullMaster, fixtureMasterVersion, fixtureVersion751, true},
		{"OS token 8.5 does not refine short 7.5", fixtureVersion75, osOnlyFull, fixtureVersion75, fixtureVersion751, false},
		{"OS token 8.5 does not refine short 8.6", "8.6", osOnlyFull, "8.6", fixtureVersion751, true},
		{"OS token 8.5 does not refine short 9.5", "9.5", osOnlyFull, "9.5", fixtureVersion751, true},
		{"short already at patch is not downgraded by a 7.5 fullVersion", fixtureVersion751, full75, fixtureVersion751, fixtureVersion751, true},
		{"whitespace around short and fullVersion is trimmed", "  7.5  ", "  " + full751 + "  ", fixtureVersion751, fixtureVersion751, true},
		{"uppercase MASTER is left as master", fixtureUpperMaster, full751, fixtureMasterVersion, fixtureVersion751, true},
		{
			"ol prefix still yields AOS patch from fullVersion", fixtureVersion75, "ol8.5-release-ganges-7.5.1-stable-abc",
			fixtureVersion751, fixtureVersion751, true,
		},
		{"both fields empty stays empty", "", "", "", fixtureVersion751, true},
		{"short 7.5 is not replaced by a mismatched 7.6 fullVersion", fixtureVersion75, full76, fixtureVersion75, fixtureVersion751, false},
		{"short 7.5 is refined by a four-component fullVersion", fixtureVersion75, fixtureFull7512, fixtureVersion7512, fixtureVersion751, true},
		{"short 7.5.1 is refined by a more specific patch", fixtureVersion751, fixtureFull7512, fixtureVersion7512, fixtureVersion751, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := ParseReported(test.short, test.full)
			assert.Equal(t, test.want, got.String())
			assert.Equal(t, test.wantGate, got.AtLeast(Parse(test.atLeast)))
		})
	}
}

func TestParseFullVersionDoesNotTreatOSPrefixAsAOS(t *testing.T) {
	t.Parallel()

	// An internal build's fullVersion has el8.5/el9 and no AOS token. It must
	// stay unknown (newest), not parse as 8.5 and fail a 7.5.1 gate.
	internal := Parse(fixtureFullMaster)
	assert.True(t, internal.AtLeast(Parse(fixtureVersion751)))
	assert.True(t, internal.AtLeast(Parse(fixtureVersion99)))

	olInternal := Parse("ol8.5-opt-master-56c4af6e0fb9c6a2aed0e517cbfb3768cde74311")
	assert.True(t, olInternal.AtLeast(Parse(fixtureVersion751)))
	assert.True(t, olInternal.AtLeast(Parse(fixtureVersion99)))
}

// TestMasterIsLatest pins the policy that a development build reporting
// "master" satisfies every minimum-version gate. Consumers rely on this to
// avoid blocking test and debug clusters.
func TestMasterIsLatest(t *testing.T) {
	t.Parallel()

	for _, version := range []string{fixtureMasterVersion, fixtureUpperMaster, "Master", "pc.master", "  master  "} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			for _, minVersion := range []string{fixtureVersion651, fixtureVersion76, "7.10.2", fixturePC20243, "2025.1.0.1"} {
				assert.True(t, Parse(version).AtLeast(Parse(minVersion)),
					"Parse(%q).AtLeast(%q) must treat master as latest", version, minVersion)
			}
		})
	}
}

// TestUnreportedVersionIsTheCallersQuestion documents the pattern for a
// caller that has no version at all. That is a question about whether the
// lookup succeeded, not about version ordering, so it is answered before
// parsing -- unlike "master", which must still gate as the newest version.
func TestUnreportedVersionIsTheCallersQuestion(t *testing.T) {
	t.Parallel()

	minPC := Parse(fixtureVersion76)
	supported := func(version string) bool {
		if version == "" {
			return false
		}

		return Parse(version).AtLeast(minPC)
	}

	assert.True(t, supported(fixturePC7605))
	assert.False(t, supported("pc.7.5.0.5"))
	assert.False(t, supported(fixturePC20243))
	assert.False(t, supported(""))
	// A development build is newer than any release, so it gates open.
	assert.True(t, supported(fixtureMasterVersion))
}

// TestParsedVersionIsReusable guards the reason for parsing separately from
// comparing: consumers parse once and gate many times.
func TestParsedVersionIsReusable(t *testing.T) {
	t.Parallel()

	v := Parse(fixturePC7605)
	for range fixtureReuseCount {
		assert.True(t, v.AtLeast(Parse(fixtureVersion76)))
		assert.False(t, v.AtLeast(Parse("7.7")))
		assert.Equal(t, fixtureVersion7605, v.String())
	}
}
