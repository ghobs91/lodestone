package parsers

import (
	"strconv"
	"strings"

	"github.com/ghobs91/lodestone/internal/classifier/classification"
	"github.com/ghobs91/lodestone/internal/keywords"
	"github.com/ghobs91/lodestone/internal/model"
	"github.com/ghobs91/lodestone/internal/regex"
	"github.com/hedhyw/rex/pkg/dialect"
	"github.com/hedhyw/rex/pkg/rex"
)

var titleTokens = []dialect.Token{
	rex.Group.Define(
		rex.Group.Composite(
			rex.Group.NonCaptured(
				regex.AnyWordChar().Repeat().OneOrMore(),
				rex.Group.NonCaptured(
					rex.Chars.Single('-'), regex.AnyWordChar().Repeat().OneOrMore(),
				).Repeat().ZeroOrMore(),
			),
			regex.AnyNonWordChar().Repeat().OneOrMore(),
		).NonCaptured().Repeat().OneOrMore(),
		rex.Group.Composite(
			regex.AnyNonWordChar().Repeat().OneOrMore(),
			rex.Chars.End(),
		).NonCaptured(),
	),
}

var titleRegex = rex.New(
	rex.Chars.Begin(),
	rex.Group.NonCaptured(titleTokens...),
).MustCompile()

var yearTokens = []dialect.Token{
	rex.Group.NonCaptured(rex.Common.NotClass(rex.Chars.WordCharacter()).Repeat().ZeroOrMore()),
	rex.Group.Define(
		rex.Group.Composite(
			rex.Common.Text("18"), rex.Common.Text("19"), rex.Common.Text("20"),
		).NonCaptured(),
		rex.Chars.Digits().Repeat().Exactly(2),
	),
	rex.Group.Composite(
		rex.Common.NotClass(rex.Chars.WordCharacter()),
		rex.Chars.End(),
	).NonCaptured(),
}

var titleYearRegex = rex.New(
	rex.Chars.Begin(),
	rex.Group.NonCaptured(rex.Group.NonCaptured(titleTokens...), rex.Group.NonCaptured(yearTokens...)),
).MustCompile()

var titleEpisodesRegex = rex.New(
	rex.Chars.Begin(),
	rex.Group.NonCaptured(
		rex.Group.NonCaptured(titleTokens...),
		model.EpisodesToken,
	),
).MustCompile()

var multiRegex = keywords.MustNewRegexFromKeywords("multi", "dual")

var separatorToken = rex.Chars.Runes(" ._")

var titlePartRegex = rex.New(
	separatorToken.Repeat().ZeroOrOne(),
	rex.Group.Define(regex.WordToken()),
	separatorToken.Repeat().ZeroOrOne(),
).MustCompile()

var trimTitleRegex = rex.New(
	rex.Chars.Begin(),
	rex.Group.Composite(
		rex.Group.NonCaptured(
			rex.Chars.Single('['),
			rex.Common.NotClass(rex.Chars.Single(']')).Repeat().OneOrMore(),
			rex.Chars.Single(']'),
		),
		rex.Group.NonCaptured(
			rex.Chars.Single('【'),
			rex.Common.NotClass(rex.Chars.Single('】')).Repeat().OneOrMore(),
			rex.Chars.Single('】'),
		),
	).NonCaptured().Repeat().ZeroOrOne(),
	regex.AnyNonWordChar().Repeat().ZeroOrMore(),
	rex.Group.Define(
		regex.WordToken(),
		rex.Group.NonCaptured(
			rex.Chars.Any(),
			regex.WordToken(),
		).Repeat().ZeroOrMore(),
	),
	regex.AnyNonWordChar().Repeat().ZeroOrMore(),
	rex.Chars.End(),
).MustCompile()

func cleanTitle(title string) string {
	title = titlePartRegex.ReplaceAllStringFunc(title, func(s string) string {
		partMatch := titlePartRegex.FindStringSubmatch(s)
		if partMatch == nil {
			return ""
		}

		return partMatch[1] + " "
	})
	title = trimTitleRegex.ReplaceAllString(title, "$1")

	return title
}

func parseTitleYear(input string) (string, model.Year, string, error) {
	if match := titleYearRegex.FindStringSubmatch(input); match != nil {
		yearMatch, _ := strconv.ParseUint(match[2], 10, 16)
		title := cleanTitle(match[1])

		if title != "" {
			return title, model.Year(yearMatch), input[len(match[0]):], nil
		}
	}

	return "", 0, "", classification.ErrUnmatched
}

func parseTitle(input string) (title string, rest string, err error) {
	if match := titleRegex.FindStringSubmatch(input); match != nil {
		title = cleanTitle(match[1])
		if title != "" {
			return title, input[len(match[0]):], nil
		}
	}

	return "", "", classification.ErrUnmatched
}

func parseTitleYearEpisodes(input string) (string, model.Year, model.Episodes, string, error) {
	if match := titleEpisodesRegex.FindStringSubmatch(input); match != nil {
		title := match[1]
		year := model.Year(0)

		if t, y, _, err := parseTitleYear(title); err == nil {
			title = t
			year = y
		} else {
			title = cleanTitle(title)
		}

		episodes := model.EpisodesMatchToEpisodes(match[2:])

		return title, year, episodes, input[len(match[0]):], nil
	}

	return "", 0, nil, "", classification.ErrUnmatched
}

func ParseTitleYearEpisodes(
	contentType model.NullContentType,
	input string,
) (string, model.Year, model.Episodes, string, error) {
	if !contentType.Valid || contentType.ContentType == model.ContentTypeTvShow {
		if title, year, episodes, rest, err := parseTitleYearEpisodes(input); err == nil {
			return title, year, episodes, rest, nil
		}
	}

	if title, year, rest, err := parseTitleYear(input); err == nil {
		return title, year, nil, rest, nil
	}

	if title, rest, err := parseTitle(input); err == nil {
		return title, 0, nil, rest, nil
	}

	return "", 0, nil, "", classification.ErrUnmatched
}

// videoTitleTags are release/quality tags that bound a title when the name has
// no year or episode pattern to delimit it. Keep this list to tags that are
// very unlikely to be part of a real title (e.g. don't include "web", "tv",
// "hd" or "sd").
var videoTitleTags = map[string]struct{}{
	"1080p": {}, "720p": {}, "2160p": {}, "1440p": {}, "480p": {},
	"360p": {}, "540p": {}, "576p": {}, "4320p": {}, "4k": {}, "8k": {},
	"uhd": {}, "fhd": {},
	"web-dl": {}, "webrip": {}, "web-rip": {}, "bluray": {}, "blu-ray": {},
	"brrip": {}, "bdrip": {}, "bdremux": {}, "dvdrip": {}, "dvd5": {},
	"dvd9": {}, "hdtv": {}, "hdrip": {}, "remux": {},
	"x264": {}, "x265": {}, "h264": {}, "h265": {}, "hevc": {}, "avc": {},
	"xvid": {}, "divx": {}, "mpeg2": {}, "mpeg4": {},
	"hdr": {}, "hdr10": {}, "10bit": {}, "8bit": {},
	"dts": {}, "ac3": {}, "truehd": {}, "atmos": {}, "ddp5": {}, "ddp7": {},
	"imax": {},
}

// trimVideoExtension removes a trailing video container extension from a
// single-file torrent name, so "Some Movie 1080p.mkv" parses the same as the
// folder name "Some Movie 1080p".
func trimVideoExtension(name string) string {
	ext := model.FileExtensionFromPath(name)
	if !ext.Valid {
		return name
	}

	fileType := model.FileTypeFromExtension(ext.String)
	if !fileType.Valid || fileType.FileType != model.FileTypeVideo {
		return name
	}

	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[:i]
	}

	return name
}

// cleanTitleWithoutYear removes release/quality tags from a title that has no
// year or episode pattern to bound it. It deliberately keeps the title so it
// can still be searched on TMDB.
func cleanTitleWithoutYear(title string) string {
	parts := strings.Fields(title)

	for i, part := range parts {
		lower := strings.ToLower(strings.Trim(part, "[](){}.,;:!?\"'"))
		if lower == "" {
			continue
		}

		if _, ok := videoTitleTags[lower]; ok {
			parts = parts[:i]
			break
		}

		matched := false
		for tag := range videoTitleTags {
			if strings.HasPrefix(lower, tag+"-") {
				parts = parts[:i]
				matched = true
				break
			}
		}
		if matched {
			break
		}
	}

	return strings.TrimRight(strings.Join(parts, " "), " -._")
}

func ParseVideoContent(torrent model.Torrent, result classification.Result) (classification.ContentAttributes, error) {
	input := trimVideoExtension(torrent.Name)

	title, year, episodes, rest, err := ParseTitleYearEpisodes(result.ContentType, input)
	if err != nil {
		if !result.ContentType.Valid {
			return classification.ContentAttributes{}, err
		}

		rest = input
	}

	ct := model.NullContentType{}

	switch {
	case result.ContentType.Valid:
		ct = model.NullContentType{Valid: true, ContentType: result.ContentType.ContentType}
	case len(episodes) > 0 || result.Date.IsValid():
		ct = model.NullContentType{Valid: true, ContentType: model.ContentTypeTvShow}
	case !year.IsNil():
		ct = model.NullContentType{Valid: true, ContentType: model.ContentTypeMovie}
	}

	if ct.ContentType != model.ContentTypeTvShow {
		episodes = nil
	}

	switch {
	case year.IsNil() && len(episodes) == 0 &&
		(!ct.Valid || ct.ContentType == model.ContentTypeMovie || ct.ContentType == model.ContentTypeTvShow):
		// Keep the title for a TMDB search even without a year, but strip
		// release/quality tags and the file extension.
		title = cleanTitleWithoutYear(title)
		rest = input
	case year.IsNil() && ct.ContentType != model.ContentTypeTvShow:
		// Preserve the old behavior for non-movie/TV types such as xxx:
		// don't use their title for a by-name search.
		title = ""
		rest = input
	}

	attrs := classification.ContentAttributes{
		ContentType:   ct,
		BaseTitle:     model.NullString{Valid: title != "", String: title},
		Date:          model.Date{Year: year},
		Episodes:      episodes,
		Languages:     model.InferLanguages(rest),
		LanguageMulti: multiRegex.MatchString(rest),
	}
	attrs.InferVideoAttributes(rest)

	return attrs, nil
}
