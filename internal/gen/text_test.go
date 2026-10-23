package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPascal(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Video Resolution":          "VideoResolution",
		"2.7K 4:3":                  "N2_7K4x3",
		"Anti-Flicker":              "AntiFlicker",
		"-2.0":                      "Minus2_0",
		"Linear + Horizon Leveling": "LinearPlusHorizonLeveling",
		"5GHZ Available":            "N5GHZAvailable",
		"":                          "Unnamed",
		"50%":                       "N50Pct",
	}
	for in, want := range cases {
		assert.Equal(t, want, pascal(in), in)
	}
	assert.Equal(t, "2_7K4x3", optionIdent("2.7K 4:3"))
	assert.Equal(t, "30", optionIdent("30.0"))
	assert.Equal(t, "Wide", optionIdent("Wide"))
	assert.Equal(t, "NeverStarted", optionIdent("Never Started"))
	assert.Equal(t, "CameraIdle", optionIdent("Camera Idle: No one is attempting to change camera settings"))
	assert.Equal(t, "AccessoryConnectedAndAMicrophoneIs", optionIdent("Accessory connected and a microphone is plugged into the accessory"))
	assert.Equal(t, "N000", pascal("000"))
	assert.Equal(t, "000", optionIdent("000: Display Mod: 0"))
}

func TestKebab(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "video-resolution", kebab("Video Resolution"))
	assert.Equal(t, "2.7k-4-3", kebab("2.7K 4:3"))
	assert.Equal(t, "anti-flicker", kebab(" Anti-Flicker "))
	assert.Equal(t, "lcd-brightness", kebab("LCD Brightness"))
}

func TestMarkdownHelpers(t *testing.T) {
	t.Parallel()
	desc := "**Internal Battery Bars**\n\n\n| Value | Meaning |\n| ----- | ------- |\n| 0 | Zero |\n| 4 | Charging |\n\nRough approximation.\nSecond line.\n"
	assert.Equal(t, "Internal Battery Bars", boldTitle(desc))
	assert.Empty(t, boldTitle("plain"))
	rows := parseValueTable(desc)
	assert.Equal(t, []tableRow{{0, "Zero"}, {4, "Charging"}}, rows)
	assert.Equal(t, "Rough approximation. Second line.", firstParagraph(stripBold(desc)))
	assert.Empty(t, parseValueTable("no table"))
	lo, hi := parseRange("percentage value from 10-100")
	assert.Equal(t, uint32(10), lo)
	assert.Equal(t, uint32(100), hi)
	lo, hi = parseRange("| 2-8    | Minute |")
	assert.Zero(t, lo, "bit tables are not ranges")
	assert.Zero(t, hi)
	lo, hi = parseRange("nothing here")
	assert.Zero(t, lo)
	assert.Zero(t, hi)
	assert.Equal(t, "OpOGPGetState", opConst("OGP_GET_STATE"))
	assert.Equal(t, "OpGPInfo", opConst("GP_INFO"))
	assert.Equal(t, "OpGPCAMERAWebcamStartOgp", opConst("GPCAMERA_WEBCAM_START_OGP"))
	assert.True(t, supportsHERO9([]string{"HERO13 Black", hero9}))
	assert.False(t, supportsHERO9(nil))
	assert.Equal(t, "Zero.", firstSentence("Zero. Then more", "x"))
	assert.Equal(t, "x", firstSentence("", "x"))
}

func TestGenerateAgainstSpecIsDeterministic(t *testing.T) {
	t.Parallel()
	spec, err := loadSpec("../../spec/opengopro/openapi.json")
	if err != nil {
		t.Skip("spec not available:", err)
	}
	ov, err := loadOverrides("../../spec/hero9_overrides.yaml")
	if err != nil {
		t.Skip("overrides not available:", err)
	}
	m1, err := buildModel(spec, ov)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := buildModel(spec, ov)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, emitSettings(m1), emitSettings(m2))
	assert.Equal(t, emitStatus(m1), emitStatus(m2))
	assert.Equal(t, emitCapability(m1), emitCapability(m2))
	assert.Equal(t, emitCapabilityDoc(m1), emitCapabilityDoc(m2))
	assert.NotEmpty(t, m1.Settings)
	assert.NotEmpty(t, m1.Statuses)
	assert.NotEmpty(t, m1.Ops)
}
