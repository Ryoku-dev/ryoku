package m3u8

/*
 This file defines functions related to playlist parsing.
*/

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

var ErrExtM3UAbsent = errors.New("#EXTM3U absent")
var ErrNotYesOrNo = errors.New("value must be YES or NO")
var ErrCannotDetectPlaylistType = errors.New("cannot detect playlist type")
var ErrDanglingSCTE35DateRange = errors.New("dangling SCTE-35 DateRange tag after last segment not supported")

// TimeParse allows globally apply and/or override Time Parser function.
// Available variants:
//   - FullTimeParse - implements full featured ISO/IEC 8601:2004
//   - StrictTimeParse - implements only RFC3339 Nanoseconds format
var TimeParse func(value string) (time.Time, error) = FullTimeParse

// Decode parses a master playlist passed from the buffer. If `strict`
// parameter is true then it returns first syntax error.
func (p *MasterPlaylist) Decode(data bytes.Buffer, strict bool) error {
	return p.decode(&data, strict)
}

// DecodeFrom parses a master playlist passed from an io.Reader.
// If strict parameter is true then it returns first syntax error.
func (p *MasterPlaylist) DecodeFrom(reader io.Reader, strict bool) error {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(reader)
	if err != nil {
		return err
	}
	return p.decode(buf, strict)
}

// WithCustomDecoders adds custom tag decoders to the master playlist for decoding
func (p *MasterPlaylist) WithCustomDecoders(customDecoders []CustomDecoder) Playlist {
	// Create the map if it doesn't already exist
	if p.Custom == nil {
		p.Custom = make(CustomMap)
	}

	p.customDecoders = customDecoders

	return p
}

// Parse master playlist. Internal function.
func (p *MasterPlaylist) decode(buf *bytes.Buffer, strict bool) error {
	var eof bool

	state := new(decodingState)

	for !eof {
		line, err := buf.ReadString('\n')
		if err == io.EOF {
			eof = true
		} else if err != nil {
			break
		}
		line = trimLineEnd(line)
		if line == "" {
			continue
		}
		err = decodeLineOfMasterPlaylist(p, state, line, strict)
		if strict && err != nil {
			return err
		}
	}

	p.attachRenditionsToVariants(state.alternatives)

	if strict && !state.m3u {
		return ErrExtM3UAbsent
	}
	return nil
}

func (p *MasterPlaylist) attachRenditionsToVariants(alternatives []*Alternative) {
	for _, variant := range p.Variants {
		if variant.Iframe {
			continue
		}
		for _, alt := range alternatives {
			if alt == nil {
				continue
			}
			if variant.Video != "" && alt.Type == "VIDEO" && variant.Video == alt.GroupId {
				variant.Alternatives = append(variant.Alternatives, alt)
			}
			if variant.Audio != "" && alt.Type == "AUDIO" && variant.Audio == alt.GroupId {
				variant.Alternatives = append(variant.Alternatives, alt)
			}
			if variant.Captions != "" && alt.Type == "CLOSED-CAPTIONS" && variant.Captions == alt.GroupId {
				variant.Alternatives = append(variant.Alternatives, alt)
			}
			if variant.Subtitles != "" && alt.Type == "SUBTITLES" && variant.Subtitles == alt.GroupId {
				variant.Alternatives = append(variant.Alternatives, alt)
			}
		}
	}
}

// Version returns the HLS protocol version as signaled by EXT-X-VERSION
func (p *MasterPlaylist) Version() uint8 {
	return p.ver
}

// SetVersion sets the HLS protocol version as signaled by EXT-X-VERSION
func (p *MasterPlaylist) SetVersion(ver uint8) {
	p.ver = ver
}

// Decode parses a media playlist passed from the buffer. If strict
// parameter is true then return first syntax error.
func (p *MediaPlaylist) Decode(data bytes.Buffer, strict bool) error {
	return p.decode(&data, strict)
}

// DecodeFrom parses a media playlist passed from the io.Reader stream.
// If strict parameter is true then it returns first syntax error.
func (p *MediaPlaylist) DecodeFrom(reader io.Reader, strict bool) error {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(reader)
	if err != nil {
		return err
	}
	return p.decode(buf, strict)
}

// WithCustomDecoders adds custom tag decoders to the media playlist for decoding.
func (p *MediaPlaylist) WithCustomDecoders(customDecoders []CustomDecoder) Playlist {
	// Create the map if it doesn't already exist
	if p.Custom == nil {
		p.Custom = make(CustomMap)
	}

	p.customDecoders = customDecoders

	return p
}

// Version returns playlist's version number.
func (p *MediaPlaylist) Version() uint8 {
	return p.ver
}

// SetVersion sets the playlist version number, note the version
// have increased automatically by other Set methods.
func (p *MediaPlaylist) SetVersion(ver uint8) {
	p.ver = ver
}

// LastSegIndex returns the index of the last segment in the media playlist.
// It calculates the index based on the next media sequence number
// and the number of segments already skipped.
// If the NextPartIndex is 0, indicating that it has just rolled over to the next segment,
// it returns the previous sequence number. Otherwise, it returns the current sequence number.
func (p *MediaPlaylist) LastSegIndex() uint64 {
	nextSeqNo := p.SegmentIndexing.NextMSNIndex + p.SkippedSegments()
	if p.SegmentIndexing.NextPartIndex == 0 {
		// Just rolled over to the next segment
		return nextSeqNo - 1
	}
	return nextSeqNo
}

func (p *MediaPlaylist) LastPartSegIndex() uint64 {
	if p.SegmentIndexing.NextPartIndex == 0 {
		// Just rolled over to the next segment
		return p.SegmentIndexing.MaxPartIndex
	}
	return p.SegmentIndexing.NextPartIndex - 1
}

func (p *MediaPlaylist) GetNextSequenceAndPart() (seq uint64, part uint64) {
	seq = p.LastSegIndex()
	part = p.LastPartSegIndex()
	if part == p.SegmentIndexing.MaxPartIndex {
		// Roll over to the next segment
		part = 0
		seq++
	} else {
		part++
	}
	return seq, part
}

func (p *MediaPlaylist) IsSegmentReady(uri string) bool {
	for _, seg := range p.Segments {
		if seg != nil && strings.HasSuffix(uri, seg.URI) {
			return true
		}
	}

	for _, partial := range p.PartialSegments {
		if strings.HasSuffix(uri, partial.URI) {
			return true
		}
	}

	return false
}

// SkippedSegments returns the value of SKIPPED-SEGMENTS tag in the media playlist.
func (p *MediaPlaylist) SkippedSegments() uint64 {
	return p.skippedSegments
}

// SCTE35Syntax returns the SCTE35 syntax version detected as used in the playlist.
func (p *MediaPlaylist) SCTE35Syntax() SCTE35Syntax {
	return p.scte35Syntax
}

func (p *MediaPlaylist) decode(buf *bytes.Buffer, strict bool) error {
	var eof bool
	var line string
	var err error

	state := new(decodingState)
	for !eof {
		if line, err = buf.ReadString('\n'); err == io.EOF {
			eof = true
		} else if err != nil {
			break
		}
		line = trimLineEnd(line)
		if line == "" {
			continue
		}
		err = decodeLineOfMediaPlaylist(p, state, line, strict)
		if strict && err != nil {
			return err
		}

	}
	if strict && !state.m3u {
		return ErrExtM3UAbsent
	}
	// SCTE-35 DATERANGE tags after last segment are not allowed
	// since we associate each SCTE-35 tag with the next segment.
	if len(state.scte35DateRanges) > 0 {
		return ErrDanglingSCTE35DateRange
	}
	return nil
}

// Decode detects type of playlist and decodes it.
func Decode(data bytes.Buffer, strict bool) (Playlist, ListType, error) {
	return decode(&data, strict, nil)
}

// DecodeFrom detects type of playlist and decodes it.
func DecodeFrom(reader io.Reader, strict bool) (Playlist, ListType, error) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(reader)
	if err != nil {
		return nil, 0, err
	}
	return decode(buf, strict, nil)
}

// DecodeWith detects the type of playlist and decodes it. It accepts either bytes.Buffer
// or io.Reader as input. Any custom decoders provided will be used during decoding.
func DecodeWith(input interface{}, strict bool, customDecoders []CustomDecoder) (Playlist, ListType, error) {
	switch v := input.(type) {
	case bytes.Buffer:
		return decode(&v, strict, customDecoders)
	case io.Reader:
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(v)
		if err != nil {
			return nil, 0, err
		}
		return decode(buf, strict, customDecoders)
	default:
		return nil, 0, fmt.Errorf("input must be bytes.Buffer or io.Reader type, got %T", input)
	}
}

// Detect playlist type and decode it. May be used as decoder for both
// master and media playlists.
func decode(buf *bytes.Buffer, strict bool, customDecoders []CustomDecoder) (Playlist, ListType, error) {
	var eof bool
	var line string
	var master *MasterPlaylist
	var media *MediaPlaylist
	var listType ListType
	var err error

	state := new(decodingState)

	master = NewMasterPlaylist()
	media, err = NewMediaPlaylist(8, 1024) // Winsize for VoD will become 0, capacity auto extends
	if err != nil {
		return nil, 0, fmt.Errorf("create media playlist failed: %w", err)
	}

	// If we have custom tags to parse
	if customDecoders != nil {
		media = media.WithCustomDecoders(customDecoders).(*MediaPlaylist)
		master = master.WithCustomDecoders(customDecoders).(*MasterPlaylist)
		state.custom = make(CustomMap)
	}

	for !eof {
		if line, err = buf.ReadString('\n'); err == io.EOF {
			eof = true
		} else if err != nil {
			break
		}
		line = trimLineEnd(line)
		if line == "" {
			continue
		}

		if state.listType != MEDIA {
			err = decodeLineOfMasterPlaylist(master, state, line, strict)
			if strict && err != nil {
				return master, state.listType, err
			}
		}

		if state.listType != MASTER {
			err = decodeLineOfMediaPlaylist(media, state, line, strict)
			if strict && err != nil {
				return media, state.listType, err
			}
		}

	}

	if strict && !state.m3u {
		return nil, listType, ErrExtM3UAbsent
	}

	switch state.listType {
	case MASTER:
		master.attachRenditionsToVariants(state.alternatives)
		return master, MASTER, nil
	case MEDIA:
		if media.Closed || media.MediaType == EVENT {
			// VoD and Event's should show the entire playlist
			_ = media.SetWinSize(0)
		}
		// SCTE-35 DATERANGE tags after last segment are not allowed
		// since we associate each SCTE-35 tag with the next segment.
		if len(state.scte35DateRanges) > 0 {
			return nil, MEDIA, ErrDanglingSCTE35DateRange
		}
		return media, MEDIA, nil
	}
	return nil, state.listType, ErrCannotDetectPlaylistType
}

// decodeAndTrimAttributes decodes a line of attributes into a map.
// It removes any quotes and spaces around the values.
func decodeAndTrimAttributes(line string) map[string]string {
	out := make(map[string]string)
	for _, attr := range decodeAttributes(line) {
		out[attr.Key] = strings.Trim(attr.Val, ` "`)
	}
	return out
}

// Parse one line of master playlist.
func decodeLineOfMasterPlaylist(p *MasterPlaylist, state *decodingState, line string, strict bool) error {
	var err error

	// check for custom tags first to allow custom parsing of existing tags
	if p.Custom != nil {
		for _, v := range p.customDecoders {
			if strings.HasPrefix(line, v.TagName()) {
				t, err := v.Decode(line)

				if strict && err != nil {
					return err
				}
				p.Custom[t.TagName()] = t
			}
		}
	}

	switch {
	case line == "#EXTM3U": // start tag first
		state.m3u = true
	case strings.HasPrefix(line, "#EXT-X-VERSION:"): // version tag
		_, err = fmt.Sscanf(line, "#EXT-X-VERSION:%d", &p.ver)
		if strict && err != nil {
			return err
		}
	case strings.HasPrefix(line, "#EXT-X-START:"):
		p.StartTime, p.StartTimePrecise, err = parseExtXStartParams(line[len("#EXT-X-START:"):])
		if err != nil {
			return fmt.Errorf("error parsing EXT-X-START: %w", err)
		}
	case line == "#EXT-X-INDEPENDENT-SEGMENTS":
		p.SetIndependentSegments(true)
	case strings.HasPrefix(line, "#EXT-X-MEDIA:"):
		state.listType = MASTER
		alt, err := parseExtXMedia(line, strict)
		if err != nil {
			return fmt.Errorf("error parsing EXT-X-MEDIA: %w", err)
		}
		state.alternatives = append(state.alternatives, &alt)
	case !state.tagStreamInf && strings.HasPrefix(line, "#EXT-X-STREAM-INF:"):
		state.tagStreamInf = true
		state.listType = MASTER
		variant, err := parseExtXStreamInf(line, strict)
		if err != nil {
			return fmt.Errorf("error parsing EXT-X-STREAM-INF: %w", err)
		}
		state.variant = variant
		p.Variants = append(p.Variants, variant)
	case state.tagStreamInf && !strings.HasPrefix(line, "#"):
		state.tagStreamInf = false
		state.variant.URI = line
	case strings.HasPrefix(line, "#EXT-X-I-FRAME-STREAM-INF:"):
		state.listType = MASTER
		variant, err := parseExtXStreamInf(line, strict)
		if err != nil {
			return fmt.Errorf("error parsing EXT-X-I-FRAME-STREAM-INF: %w", err)
		}
		state.variant = variant
		state.variant.Iframe = true
		p.Variants = append(p.Variants, state.variant)
	case strings.HasPrefix(line, "#EXT-X-DEFINE:"): // Define tag
		define, err := parseDefine(line)
		if err != nil {
			return err
		}
		err = p.AppendDefine(define)
		if err != nil {
			return err
		}
	case strings.HasPrefix(line, "#EXT-X-SESSION-DATA:"):
		sd, err := parseSessionData(line)
		if err != nil {
			return err
		}
		p.SessionDatas = append(p.SessionDatas, sd)
	case strings.HasPrefix(line, "#EXT-X-SESSION-KEY:"):
		p.SessionKeys = append(p.SessionKeys, parseKeyParams(line[19:]))
	case strings.HasPrefix(line, "#EXT-X-CONTENT-STEERING:"):
		p.ContentSteering = parseContentSteering(line[len("#EXT-X-CONTENT-STEERING:"):])
	}

	return err
}

func parseExtXMedia(line string, strict bool) (Alternative, error) {
	var alt Alternative
	if !strings.HasPrefix(line, "#EXT-X-MEDIA:") {
		return alt, fmt.Errorf("invalid line: %q", line)
	}
	var err error
	for k, v := range decodeAndTrimAttributes(line[len("#EXT-X-MEDIA:"):]) {
		switch k {
		case "TYPE":
			alt.Type = v
		case "URI":
			alt.URI = v
		case "GROUP-ID":
			alt.GroupId = v
		case "LANGUAGE":
			alt.Language = v
		case "ASSOC-LANGUAGE":
			alt.AssocLanguage = v
		case "NAME":
			alt.Name = v
		case "STABLE-RENDITION-ID":
			alt.StableRenditionId = v
		case "DEFAULT":
			alt.Default, err = yesOrNo(v, strict)
			if err != nil {
				return alt, fmt.Errorf("%s:%s %w", k, v, ErrNotYesOrNo)
			}
		case "AUTOSELECT":
			alt.Autoselect, err = yesOrNo(v, strict)
			if err != nil {
				return alt, fmt.Errorf("%s:%s %w", k, v, ErrNotYesOrNo)
			}
		case "FORCED":
			alt.Forced, err = yesOrNo(v, strict)
			if err != nil {
				return alt, fmt.Errorf("%s:%s %w", k, v, ErrNotYesOrNo)
			}
		case "INSTREAM-ID":
			alt.InstreamId = v
		case "BIT-DEPTH":
			bitDepth, err := strconv.Atoi(v)
			if err != nil {
				return alt, fmt.Errorf("invalid BIT-DEPTH: %w", err)
			}
			alt.BitDepth = byte(bitDepth)
		case "SAMPLE-RATE":
			sampleRate, err := strconv.Atoi(v)
			if err != nil {
				return alt, fmt.Errorf("invalid SAMPLE-RATE: %w", err)
			}
			alt.SampleRate = uint32(sampleRate)
		case "CHARACTERISTICS":
			alt.Characteristics = v
		case "CHANNELS":
			alt.Channels, err = parseChannels(v)
			if err != nil {
				return alt, fmt.Errorf("invalid CHANNELS: %w", err)
			}
		}
	}
	return alt, nil
}

func parseExtXStreamInf(line string, strict bool) (*Variant, error) {
	variant := Variant{}
	var tagLen int
	switch {
	case strings.HasPrefix(line, "#EXT-X-STREAM-INF:"):
		tagLen = len("#EXT-X-STREAM-INF:")
	case strings.HasPrefix(line, "#EXT-X-I-FRAME-STREAM-INF:"):
		tagLen = len("#EXT-X-I-FRAME-STREAM-INF:")
	default:
		return nil, fmt.Errorf("invalid line: %q", line)
	}
	attrs := decodeAttributes(line[tagLen:])
	for _, a := range attrs {
		switch a.Key {
		case "BANDWIDTH":
			val, err := strconv.Atoi(a.Val)
			if strict && err != nil {
				return nil, err
			}
			variant.Bandwidth = uint32(val)
		case "AVERAGE-BANDWIDTH":
			val, err := strconv.Atoi(a.Val)
			if strict && err != nil {
				return nil, err
			}
			variant.AverageBandwidth = uint32(val)
		case "SCORE":
			val, err := strconv.ParseFloat(a.Val, 64)
			if strict && err != nil {
				return nil, err
			}
			variant.Score = val
		case "CODECS":
			variant.Codecs = deQuote(a.Val)
		case "SUPPLEMENTAL-CODECS":
			variant.SupplementalCodecs = deQuote(a.Val)
		case "RESOLUTION": // decimal-resolution WxH
			variant.Resolution = a.Val
		case "FRAME-RATE":
			val, err := strconv.ParseFloat(a.Val, 64)
			if strict && err != nil {
				return nil, err
			}
			variant.FrameRate = val
		case "HDCP-LEVEL": // NONE, TYPE-0, TYPE-1
			variant.HDCPLevel = a.Val
		case "ALLOWED-CPC":
			variant.AllowedCPC = deQuote(a.Val)
		case "VIDEO-RANGE": // SDR, HLG, PQ
			variant.VideoRange = a.Val
		case "REQ-VIDEO-LAYOUT":
			variant.ReqVideoLayout = deQuote(a.Val)
		case "STABLE-VARIANT-ID":
			variant.StableVariantId = deQuote(a.Val)
		case "AUDIO": // Alternative renditions group ID
			variant.Audio = deQuote(a.Val)
		case "VIDEO": // Alternative renditions group ID
			variant.Video = deQuote(a.Val)
		case "SUBTITLES": // Alternative renditions group ID
			variant.Subtitles = deQuote(a.Val)
		case "CLOSED-CAPTIONS":
			if a.Val == "NONE" {
				variant.Captions = "NONE"
			} else {
				variant.Captions = deQuote(a.Val)
			}
		case "PATHWAY-ID": // Content steering pathway ID
			variant.PathwayId = deQuote(a.Val)
		case "URI":
			variant.URI = deQuote(a.Val)
		case "PROGRAM-ID": // Deprecated from version 6
			val, err := strconv.Atoi(a.Val)
			if strict && err != nil {
				return nil, err
			}
			variant.ProgramId = &val
		case "NAME":
			variant.Name = deQuote(a.Val)
		}
	}
	return &variant, nil
}

func parseDateRange(line string) (*DateRange, error) {
	var dr DateRange
	if !strings.HasPrefix(line, "#EXT-X-DATERANGE:") {
		return nil, fmt.Errorf("invalid date-range line: %q", line)
	}
	for _, attr := range decodeAttributes(line[17:]) {
		switch attr.Key {
		case "ID":
			dr.ID = deQuote(attr.Val)
		case "CLASS":
			dr.Class = deQuote(attr.Val)
		case "START-DATE":
			startDate, err := time.Parse(DATETIME, deQuote(attr.Val))
			if err != nil {
				return nil, fmt.Errorf("invalid START-DATE: %w", err)
			}
			dr.StartDate = startDate
		case "END-DATE":
			endDate, err := time.Parse(DATETIME, deQuote(attr.Val))
			if err != nil {
				return nil, fmt.Errorf("invalid END-DATE: %w", err)
			}
			dr.EndDate = &endDate
		case "CUE":
			dr.Cue = attr.Val
		case "DURATION":
			dur, err := strconv.ParseFloat(attr.Val, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid DURATION: %w", err)
			}
			dr.Duration = &dur
		case "PLANNED-DURATION":
			plannedDur, err := strconv.ParseFloat(attr.Val, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid PLANNED-DURATION: %w", err)
			}
			dr.PlannedDuration = &plannedDur
		case "SCTE35-CMD":
			if len(attr.Val) <= 4 || attr.Val[:2] != "0x" {
				return nil, fmt.Errorf("invalid SCTE35-CMD: %s", attr.Val)
			}
			dr.SCTE35Cmd = attr.Val
		case "SCTE35-OUT":
			if len(attr.Val) <= 4 || attr.Val[:2] != "0x" {
				return nil, fmt.Errorf("invalid SCTE35-OUT: %s", attr.Val)
			}
			dr.SCTE35Out = attr.Val
		case "SCTE35-IN":
			if len(attr.Val) <= 4 || attr.Val[:2] != "0x" {
				return nil, fmt.Errorf("invalid SCTE35-IN: %s", attr.Val)
			}
			dr.SCTE35In = attr.Val
		case "END-ON-NEXT":
			dr.EndOnNext = attr.Val == "YES"
		default:
			if strings.HasPrefix(attr.Key, "X-") {
				dr.XAttrs = append(dr.XAttrs, attr)
			}
		}
	}
	return &dr, nil
}

func parseExtXStartParams(parameters string) (float64, bool, error) {
	var startTime float64
	var startTimePrecise bool
	var err error

	for _, attr := range decodeAttributes(parameters) {
		switch attr.Key {
		case "TIME-OFFSET":
			startTime, err = strconv.ParseFloat(attr.Val, 64)
			if err != nil {
				return startTime, startTimePrecise,
					fmt.Errorf("invalid TIME-OFFSET: %s: %w", attr.Val, err)
			}
		case "PRECISE":
			startTimePrecise = attr.Val == "YES"
		}
	}
	return startTime, startTimePrecise, nil
}

func parseDefine(line string) (Define, error) {
	var (
		d   Define
		err error
	)

	switch {
	case strings.HasPrefix(line, "#EXT-X-DEFINE:NAME="):
		d.Type = VALUE
		_, err = fmt.Sscanf(line, "#EXT-X-DEFINE:NAME=%q,VALUE=%q", &d.Name, &d.Value)
	case strings.HasPrefix(line, "#EXT-X-DEFINE:QUERYPARAM="):
		d.Type = QUERYPARAM
		_, err = fmt.Sscanf(line, "#EXT-X-DEFINE:QUERYPARAM=%q", &d.Name)
	case strings.HasPrefix(line, "#EXT-X-DEFINE:IMPORT="):
		d.Type = IMPORT
		_, err = fmt.Sscanf(line, "#EXT-X-DEFINE:IMPORT=%q", &d.Name)
	default:
		return d, fmt.Errorf("unknown EXT-X-DEFINE format: %s", line)
	}

	if err != nil {
		return d, fmt.Errorf("error parsing EXT-X-DEFINE: %w", err)
	}
	return d, nil
}

func parsePartialSegment(parameters string) (*PartialSegment, error) {
	ps := PartialSegment{}
	for _, attr := range decodeAttributes(parameters) {
		switch attr.Key {
		case "URI":
			ps.URI = deQuote(attr.Val)
		case "DURATION":
			duration, err := strconv.ParseFloat(attr.Val, 64)
			if err != nil {
				return nil, fmt.Errorf("duration parsing error: %w", err)
			}
			ps.Duration = duration
		case "INDEPENDENT":
			ps.Independent = attr.Val == "YES"
		case "BYTERANGE":
			if _, err := fmt.Sscanf(deQuote(attr.Val), "%d@%d", &ps.Limit, &ps.Offset); err != nil {
				return nil, fmt.Errorf("byterange sub-range length value parsing error: %w", err)
			}
		}
	}
	return &ps, nil
}

func parsePreloadHint(parameters string) (*PreloadHint, error) {
	ph := PreloadHint{}
	for _, attr := range decodeAttributes(parameters) {
		switch attr.Key {
		case "TYPE":
			ph.Type = attr.Val
		case "URI":
			ph.URI = deQuote(attr.Val)
		case "BYTERANGE-START":
			start, err := strconv.ParseInt(attr.Val, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("start parsing error: %w", err)
			}
			ph.Offset = start
		case "BYTERANGE-LENGTH":
			length, err := strconv.ParseInt(attr.Val, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("length parsing error: %w", err)
			}
			ph.Limit = length
		}
	}
	return &ph, nil
}

func parseSkipTag(parameters string) (uint64, error) {
	var skipped uint64
	var err error
	for _, attr := range decodeAttributes(parameters) {
		switch attr.Key {
		case "SKIPPED-SEGMENTS":
			if skipped, err = strconv.ParseUint(attr.Val, 10, 64); err != nil {
				return 0, fmt.Errorf("skipped-segments parsing error: %w", err)
			}
		}
	}
	return skipped, nil
}

func parseServerControl(parameters string) (*ServerControl, error) {
	sc := ServerControl{}
	var err error
	for _, attr := range decodeAttributes(parameters) {
		switch attr.Key {
		case "CAN-SKIP-UNTIL":
			if sc.CanSkipUntil, err = strconv.ParseFloat(attr.Val, 64); err != nil {
				return nil, fmt.Errorf("can-skip-until parsing error: %w", err)
			}
		case "CAN-SKIP-DATERANGES":
			sc.CanSkipDateRanges = attr.Val == "YES"
		case "HOLD-BACK":
			if sc.HoldBack, err = strconv.ParseFloat(attr.Val, 64); err != nil {
				return nil, fmt.Errorf("hold-back parsing error: %w", err)
			}
		case "PART-HOLD-BACK":
			if sc.PartHoldBack, err = strconv.ParseFloat(attr.Val, 64); err != nil {
				return nil, fmt.Errorf("part-hold-back parsing error: %w", err)
			}
		case "CAN-BLOCK-RELOAD":
			sc.CanBlockReload = attr.Val == "YES"
		}
	}
	return &sc, nil
}

func parseSessionData(line string) (*SessionData, error) {
	sd := SessionData{
		Format: "JSON",
	}
	if !strings.HasPrefix(line, "#EXT-X-SESSION-DATA:") {
		return nil, fmt.Errorf("invalid EXT-X-SESSION-DATA line: %q", line)
	}
	for _, attr := range decodeAttributes(line[len("EXT-X-SESSION_-DATA:"):]) {
		switch attr.Key {
		case "DATA-ID":
			sd.DataId = deQuote(attr.Val)
		case "VALUE":
			sd.Value = deQuote(attr.Val)
		case "URI":
			sd.URI = deQuote(attr.Val)
		case "FORMAT":
			switch attr.Val {
			case "JSON", "RAW":
				sd.Format = attr.Val
			default:
				return nil, fmt.Errorf("invalid FORMAT: %s", attr.Val)
			}
		case "LANGUAGE":
			sd.Language = deQuote(attr.Val)
		}
	}
	return &sd, nil
}

func parseExtXMapParameters(parameters string) (*Map, error) {
	m := Map{}
	for _, attr := range decodeAttributes(parameters) {
		switch attr.Key {
		case "URI":
			m.URI = deQuote(attr.Val)
		case "BYTERANGE":
			if _, err := fmt.Sscanf(deQuote(attr.Val), "%d@%d", &m.Limit, &m.Offset); err != nil {
				return nil, fmt.Errorf("byterange sub-range length value parsing error: %w", err)
			}
		}
	}
	return &m, nil
}

func parseKeyParams(parameters string) *Key {
	key := Key{}
	for _, attr := range decodeAttributes(parameters) {
		switch attr.Key {
		case "METHOD":
			key.Method = attr.Val // NONE, AES-128, SAMPLE-AES, SAMPLE-AES-CTR
		case "URI":
			key.URI = deQuote(attr.Val)
		case "IV":
			key.IV = attr.Val // Hex value
		case "KEYFORMAT":
			key.Keyformat = deQuote(attr.Val)
		case "KEYFORMATVERSIONS":
			key.Keyformatversions = deQuote(attr.Val)
		}
	}
	return &key
}

func parseContentSteering(params string) *ContentSteering {
	cs := ContentSteering{}
	for _, attr := range decodeAttributes(params) {
		switch attr.Key {
		case "SERVER-URI":
			cs.ServerURI = deQuote(attr.Val)
		case "PATHWAY-ID":
			cs.PathwayId = deQuote(attr.Val)
		}
	}
	return &cs
}

func parseChannels(line string) (*Channels, error) {
	c := &Channels{}
	// split the parameters
	params := strings.Split(line, "/")
	paramAmt := len(params)
	channels, err := strconv.Atoi(params[0])
	if err != nil {
		return nil, fmt.Errorf("invalid attribute value. first parameter of CHANNELS must be an integer: %s", params[0])
	}
	c.Amount = channels
	if paramAmt >= 2 {
		// if there is more than one parameter, the second one is a list of spatial audio identifiers
		c.SpatialAudioIdentifiers = params[1]
	}
	if paramAmt == 3 {
		// if there are three parameters, the third one is a list of channel usage indicators
		//This parameter is optional, however if it is present the second parameter MUST be non-empty.
		if c.SpatialAudioIdentifiers == "" {
			return nil, fmt.Errorf(
				"invalid attribute value. if CHANNELS contains 3 parameters,"+
					" the second MUST be non-empty: %s",
				line,
			)
		}
		c.ChannelUsageIndicators = params[2]
	}
	return c, nil
}

// deQuote removes quotes from a string.
func deQuote(s string) string {
	if len(s) < 2 {
		return s
	}
	if s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// Parse one line of a media playlist.
func decodeLineOfMediaPlaylist(p *MediaPlaylist, state *decodingState, line string, strict bool) error {
	var err error

	// check for custom tags first to allow custom parsing of existing tags
	if p.Custom != nil {
		for _, v := range p.customDecoders {
			if strings.HasPrefix(line, v.TagName()) {
				t, err := v.Decode(line)

				if strict && err != nil {
					return err
				}

				if v.SegmentTag() {
					state.tagCustom = true
					state.custom[v.TagName()] = t
				} else {
					p.Custom[v.TagName()] = t
				}
			}
		}
	}

	switch {
	case line == "#EXT-X-INDEPENDENT-SEGMENTS":
		p.SetIndependentSegments(true)
	case !state.tagInf && strings.HasPrefix(line, "#EXTINF:"):
		state.tagInf = true
		state.listType = MEDIA
		sepIndex := strings.Index(line, ",")
		if sepIndex == -1 {
			if strict {
				return fmt.Errorf("could not parse: %q", line)
			}
			sepIndex = len(line)
		}
		duration := line[8:sepIndex]
		if len(duration) > 0 {
			if state.duration, err = strconv.ParseFloat(duration, 64); strict && err != nil {
				return fmt.Errorf("duration parsing error: %w", err)
			}
		}
		if len(line) > sepIndex {
			state.title = line[sepIndex+1:]
		}
	case !strings.HasPrefix(line, "#"):
		if state.tagInf {
			seg := GetSegment()
			seg.URI = line
			seg.Duration = state.duration
			seg.Title = state.title
			if state.lastReadMap != nil && !state.lastReadMap.Equal(state.lastStoredMap) {
				seg.Map = state.lastReadMap
				state.lastStoredMap = state.lastReadMap
			}
			err := p.AppendSegment(seg)
			if err == ErrPlaylistFull {
				// Extend playlist by doubling size, reset internal state, try again.
				// If the second Append fails, the if err block will handle it.
				// Retrying instead of being recursive was chosen as the state may be
				// modified non-idempotently.
				p.Segments = append(p.Segments, make([]*MediaSegment, p.Count())...)
				p.capacity = uint(len(p.Segments))
				p.tail = p.count
				err = p.AppendSegment(seg)
			}

			// Check err for first or subsequent Append()
			if err != nil {
				return err
			}

			state.tagInf = false
		}
		if state.tagRange {
			if err = p.SetRange(state.limit, state.offset); strict && err != nil {
				return err
			}
			state.tagRange = false
		}
		if state.tagSCTE35 {
			state.tagSCTE35 = false
			if err = p.SetSCTE35(state.scte); strict && err != nil {
				return err
			}
			p.scte35Syntax = state.scte.Syntax
			state.scte = nil

		}
		if len(state.scte35DateRanges) > 0 {
			p.Segments[p.last()].SCTE35DateRanges = state.scte35DateRanges
			state.scte35DateRanges = nil
			p.scte35Syntax = SCTE35_DATERANGE
		}
		if state.tagDiscontinuity {
			state.tagDiscontinuity = false
			if err = p.SetDiscontinuity(); strict && err != nil {
				return err
			}
		}
		if state.tagGap {
			state.tagGap = false
			if err = p.SetGap(); strict && err != nil {
				return err
			}
		}
		if state.tagProgramDateTime && p.Count() > 0 {
			state.tagProgramDateTime = false
			if err = p.SetProgramDateTime(state.programDateTime); strict && err != nil {
				return err
			}
		}
		// If EXT-X-KEY appeared before reference to segment (EXTINF) then it linked to this segment
		if state.tagKey {
			p.Segments[p.last()].Keys = state.xkeys
			// First EXT-X-KEY may appeared in the header of the playlist and linked to first segment
			// but for convenient playlist generation it also linked as default playlist key
			if len(p.Keys) == 0 {
				p.Keys = state.xkeys
			}
			// reset state
			state.xkeys = nil
			state.tagKey = false
		}
		// if segment custom tag appeared before EXTINF then it links to this segment
		if state.tagCustom {
			p.Segments[p.last()].Custom = state.custom
			state.custom = make(CustomMap)
			state.tagCustom = false
		}
		// all partial segment which appeared before the segment should be marked as completed
		if state.tagPartialSegment {
			// Mark all partial segments as completed
			state.tagPartialSegment = false
		}
	// start tag first
	case line == "#EXTM3U":
		state.m3u = true
	case line == "#EXT-X-ENDLIST":
		state.listType = MEDIA
		p.Closed = true
	case strings.HasPrefix(line, "#EXT-X-VERSION:"):
		if _, err = fmt.Sscanf(line, "#EXT-X-VERSION:%d", &p.ver); strict && err != nil {
			return err
		}
	case strings.HasPrefix(line, "#EXT-X-TARGETDURATION:"):
		state.listType = MEDIA
		if _, err = fmt.Sscanf(line, "#EXT-X-TARGETDURATION:%d", &p.TargetDuration); strict && err != nil {
			return err
		}
	case strings.HasPrefix(line, "#EXT-X-PART-INF:PART-TARGET="):
		state.listType = MEDIA
		if _, err = fmt.Sscanf(line, "#EXT-X-PART-INF:PART-TARGET=%f", &p.PartTargetDuration); strict && err != nil {
			return err
		}
	case strings.HasPrefix(line, "#EXT-X-SERVER-CONTROL:"):
		state.listType = MEDIA
		if p.ServerControl, err = parseServerControl(line[22:]); err != nil {
			return err
		}
	case strings.HasPrefix(line, "#EXT-X-SKIP:"):
		state.listType = MEDIA
		skipped, err := parseSkipTag(line[12:])
		if err != nil {
			return err
		}
		p.skippedSegments = skipped
	case strings.HasPrefix(line, "#EXT-X-PART:"):
		state.listType = MEDIA
		state.tagPartialSegment = true
		partialSegment, err := parsePartialSegment(line[12:])
		if err != nil {
			return err
		}
		// if the program date time tag is present, set it on this partial segment
		if state.tagProgramDateTime && p.HasPartialSegments() {
			partialSegment.ProgramDateTime = state.programDateTime
			state.tagProgramDateTime = false
		}
		if err = p.AppendPartialSegment(partialSegment); err != nil {
			return err
		}
	case strings.HasPrefix(line, "#EXT-X-PRELOAD-HINT:"):
		preloadHint, err := parsePreloadHint(line[20:])
		if err != nil {
			return fmt.Errorf("error parsing EXT-X-PRELOAD-HINT: %w", err)
		}
		p.PreloadHints = preloadHint
	case strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"):
		state.listType = MEDIA
		if _, err = fmt.Sscanf(line, "#EXT-X-MEDIA-SEQUENCE:%d", &p.SeqNo); strict && err != nil {
			return err
		}
		p.SegmentIndexing.NextMSNIndex = p.SeqNo
	case strings.HasPrefix(line, "#EXT-X-DEFINE:"): // Define tag
		define, err := parseDefine(line)
		if err != nil {
			return fmt.Errorf("error parsing EXT-X-DEFINE: %w", err)
		}
		p.AppendDefine(define)
	case strings.HasPrefix(line, "#EXT-X-PLAYLIST-TYPE:"):
		state.listType = MEDIA
		var playlistType string
		_, err = fmt.Sscanf(line, "#EXT-X-PLAYLIST-TYPE:%s", &playlistType)
		if err != nil {
			if strict {
				return err
			}
		} else {
			switch playlistType {
			case "EVENT":
				p.MediaType = EVENT
			case "VOD":
				p.MediaType = VOD
			}
		}
	case strings.HasPrefix(line, "#EXT-X-DISCONTINUITY-SEQUENCE:"):
		state.listType = MEDIA
		if _, err = fmt.Sscanf(line, "#EXT-X-DISCONTINUITY-SEQUENCE:%d", &p.DiscontinuitySeq); strict && err != nil {
			return err
		}
	case strings.HasPrefix(line, "#EXT-X-START:"):
		p.StartTime, p.StartTimePrecise, err = parseExtXStartParams(line[len("#EXT-X-START:"):])
		if err != nil {
			return fmt.Errorf("error parsing EXT-X-START: %w", err)
		}
	case strings.HasPrefix(line, "#EXT-X-KEY:"):
		state.listType = MEDIA
		xkey := parseKeyParams(line[11:])
		state.xkeys = append(state.xkeys, *xkey)
		state.tagKey = true
	case strings.HasPrefix(line, "#EXT-X-MAP:"):
		state.listType = MEDIA
		xMap, err := parseExtXMapParameters(line[11:])
		if err != nil {
			return fmt.Errorf("error parsing EXT-X-MAP: %w", err)
		}
		if state.lastReadMap == nil && p.Count() == 0 {
			p.Map = xMap
			state.lastStoredMap = xMap
		}
		if state.lastReadMap == nil || !state.lastReadMap.Equal(xMap) {
			state.lastReadMap = xMap
		}
	case !state.tagProgramDateTime && strings.HasPrefix(line, "#EXT-X-PROGRAM-DATE-TIME:"):
		state.tagProgramDateTime = true
		state.listType = MEDIA
		if state.programDateTime, err = TimeParse(line[25:]); strict && err != nil {
			return err
		}
	case !state.tagRange && strings.HasPrefix(line, "#EXT-X-BYTERANGE:"):
		state.tagRange = true
		state.listType = MEDIA
		state.offset = 0
		params := strings.SplitN(line[17:], "@", 2)
		if state.limit, err = strconv.ParseInt(params[0], 10, 64); strict && err != nil {
			return fmt.Errorf("byterange sub-range length value parsing error: %w", err)
		}
		if len(params) > 1 {
			if state.offset, err = strconv.ParseInt(params[1], 10, 64); strict && err != nil {
				return fmt.Errorf("byterange sub-range offset value parsing error: %w ", err)
			}
		}
	case !state.tagSCTE35 && strings.HasPrefix(line, "#EXT-SCTE35:"):
		state.tagSCTE35 = true
		state.listType = MEDIA
		state.scte = new(SCTE)
		state.scte.Syntax = SCTE35_67_2014
		for attribute, value := range decodeAndTrimAttributes(line[12:]) {
			switch attribute {
			case "CUE":
				state.scte.Cue = value
			case "ID":
				state.scte.ID = value
			case "TIME":
				state.scte.Time, _ = strconv.ParseFloat(value, 64)
			}
		}
	case !state.tagSCTE35 && strings.HasPrefix(line, "#EXT-OATCLS-SCTE35:"):
		// EXT-OATCLS-SCTE35 contains the SCTE35 tag, EXT-X-CUE-OUT contains duration
		state.tagSCTE35 = true
		state.scte = new(SCTE)
		state.scte.Syntax = SCTE35_OATCLS
		state.scte.Cue = line[19:]
		// on the line below, state.scte.Syntax is a nil pointer
	case state.tagSCTE35 && state.scte != nil &&
		state.scte.Syntax == SCTE35_OATCLS && strings.HasPrefix(line, "#EXT-X-CUE-OUT:"):
		// EXT-OATCLS-SCTE35 contains the SCTE35 tag, EXT-X-CUE-OUT contains duration
		state.scte.Time, _ = strconv.ParseFloat(line[15:], 64)
		state.scte.CueType = SCTE35Cue_Start
	case !state.tagSCTE35 && strings.HasPrefix(line, "#EXT-X-CUE-OUT-CONT:"):
		state.tagSCTE35 = true
		state.scte = new(SCTE)
		state.scte.Syntax = SCTE35_OATCLS
		state.scte.CueType = SCTE35Cue_Mid
		for attribute, value := range decodeAndTrimAttributes(line[20:]) {
			switch attribute {
			case "SCTE35":
				state.scte.Cue = value
			case "Duration":
				state.scte.Time, _ = strconv.ParseFloat(value, 64)
			case "ElapsedTime":
				state.scte.Elapsed, _ = strconv.ParseFloat(value, 64)
			}
		}
	case !state.tagSCTE35 && strings.HasPrefix(line, "#EXT-X-CUE-OUT"):
		state.tagSCTE35 = true
		state.scte = new(SCTE)
		state.scte.Syntax = SCTE35_OATCLS
		state.scte.CueType = SCTE35Cue_Start
		lenLine := len(line)
		// EXT-X-CUE-OUT may have DURATION attribute or just a duration value.
		// It's not part of the standard, and different vendors use different formats.
		// We support decoding both, but only encode the version with just the value.
		hasDurationAttr := strings.Contains(line, "DURATION=")
		if hasDurationAttr {
			for attribute, value := range decodeAndTrimAttributes(line[15:]) {
				switch attribute {
				case "DURATION":
					state.scte.Time, _ = strconv.ParseFloat(value, 64)
				}
			}
		} else if lenLine > 14 {
			state.scte.Time, _ = strconv.ParseFloat(line[15:], 64)
		}
	case !state.tagSCTE35 && line == "#EXT-X-CUE-IN":
		state.tagSCTE35 = true
		state.scte = new(SCTE)
		state.scte.Syntax = SCTE35_OATCLS
		state.scte.CueType = SCTE35Cue_End
	case strings.HasPrefix(line, "#EXT-X-DATERANGE:"):
		dr, err := parseDateRange(line)
		if err != nil {
			return fmt.Errorf("error parsing EXT-X-DATERANGE: %w", err)
		}
		isSCTE35 := dr.SCTE35Cmd != "" || dr.SCTE35Out != "" || dr.SCTE35In != ""
		if isSCTE35 {
			state.scte35DateRanges = append(state.scte35DateRanges, dr)
		} else { // Other EXT-X-DATERANGE
			p.DateRanges = append(p.DateRanges, dr)
		}
	case !state.tagDiscontinuity && strings.HasPrefix(line, "#EXT-X-DISCONTINUITY"):
		state.tagDiscontinuity = true
		state.listType = MEDIA
	case !state.tagGap && strings.HasPrefix(line, "#EXT-X-GAP"):
		state.tagGap = true
		state.listType = MEDIA
	case strings.HasPrefix(line, "#EXT-X-I-FRAMES-ONLY"):
		state.listType = MEDIA
		p.Iframe = true
	case strings.HasPrefix(line, "#EXT-X-ALLOW-CACHE:"):
		val := strings.TrimPrefix(line, "#EXT-X-ALLOW-CACHE:") == "YES"
		p.AllowCache = &val
	}
	return err
}

// StrictTimeParse implements RFC3339 with Nanoseconds accuracy.
func StrictTimeParse(value string) (time.Time, error) {
	return time.Parse(DATETIME, value)
}

// FullTimeParse implements ISO/IEC 8601:2004.
func FullTimeParse(value string) (time.Time, error) {
	layouts := []string{
		"2006-01-02T15:04:05.999999999Z0700",
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05.999999999Z07",
	}
	var (
		err error
		t   time.Time
	)
	for _, layout := range layouts {
		if t, err = time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	return t, err
}

func yesOrNo(v string, strict bool) (bool, error) {
	if strict {
		switch v {
		case "YES":
			return true, nil
		case "NO":
			return false, nil
		default:
			return false, fmt.Errorf("value %q: %w", v, ErrNotYesOrNo)
		}
	}
	switch {
	case strings.ToUpper(v) == "YES":
		return true, nil
	default:
		return false, nil
	}
}

// trimLineEnd removes a trailing `\n` or `\r\n` from a string.
func trimLineEnd(line string) string {
	l := len(line)
	nrRemove := 0
	if l > 0 && line[l-1] == '\n' {
		nrRemove++
		if l > 1 && line[l-2] == '\r' {
			nrRemove++
		}
		return line[:l-nrRemove]
	}
	return line
}
