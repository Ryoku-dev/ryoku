package m3u8

/*
 This file defines functions related to playlist generation.
*/

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrPlaylistFull = errors.New("playlist is full")
var ErrPlaylistEmpty = errors.New("playlist is empty")
var ErrWinSizeTooSmall = errors.New("window size must be >= capacity")
var ErrAlreadySkipped = errors.New("can not change the existing skip tag in a playlist")

var segmentSlices = sync.Pool{}
var segments = sync.Pool{}
var buffers = sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

func getBuffer() bytes.Buffer {
	b := buffers.Get().(*bytes.Buffer)
	b.Reset()
	return *b
}

func putBuffer(buf *bytes.Buffer) {
	buffers.Put(buf)
}

func getSegmentSlice(size uint) []*MediaSegment {
	s, ok := segmentSlices.Get().(*[]*MediaSegment)
	if ok && s != nil && cap(*s) >= int(size) {
		return (*s)[:size]
	}
	// Nothing in the pool or too small, make a new one
	return make([]*MediaSegment, size)
}

func putSegmentSlice(s *[]*MediaSegment) {
	// Set all elements to nil before returning it to pool
	// Lets GC clean up any referenced segments
	capLen := cap(*s)
	*s = (*s)[:capLen]
	for i := range *s {
		(*s)[i] = nil
	}
	segmentSlices.Put(s)
}

// GetSegment returns a segment using a sync.Pool.
// A new segment is created if the pool is empty.
// Segments used in a playlisy get recycled back to the pool when ReleasePlaylist is called
func GetSegment() *MediaSegment {
	s, ok := segments.Get().(*MediaSegment)
	if ok && s != nil {
		return s
	}
	// Nothing in the pool make a new one
	return &MediaSegment{}
}

// releaseSegment returns the segment to the pool for reuse.
func releaseSegment(s *MediaSegment) {
	// Clean the segment
	// Make sure we don't put nil pointers in there
	if s != nil {
		*s = MediaSegment{}
		segments.Put(s)
	}
}

// updateVersion updates the version if it is higher than before.
func updateVersion(ver *uint8, newVer uint8) {
	if *ver < newVer {
		*ver = newVer
	}
}

func strVer(ver uint8) string {
	return strconv.FormatUint(uint64(ver), 10)
}

// NewMasterPlaylist creates a new empty master playlist.
func NewMasterPlaylist() *MasterPlaylist {
	p := &MasterPlaylist{
		buf: getBuffer(),
	}
	p.ver = minVer
	p.writePrecision = DefaultFloatPrecision
	return p
}

// ReleasePlaylist returns buffer to pool for reuse. Do not use the playlist after this
func (p *MasterPlaylist) ReleasePlaylist() {
	putBuffer(&p.buf)
}

// Append appends a variant to master playlist. This operation resets the cache.
func (p *MasterPlaylist) Append(uri string, chunklist *MediaPlaylist, params VariantParams) {
	v := new(Variant)
	v.URI = uri
	v.Chunklist = chunklist
	v.VariantParams = params
	p.Variants = append(p.Variants, v)
	if len(v.Alternatives) > 0 {
		// This is not needed according to [Protocol Version Compatibility]
		// but remains for backwards compatibility reasons. Set the version
		// manually by using SerVersion.
		updateVersion(&p.ver, 4)
	}
	p.buf.Reset()
}

func (p *MasterPlaylist) AppendDefine(d Define) error {
	if d.Type == IMPORT {
		return errors.New("IMPORT not allowed in master playlist")
	}
	p.Defines = append(p.Defines, d)
	return nil
}

// ResetCache resets the playlist's cache (its buffer).
func (p *MasterPlaylist) ResetCache() {
	p.buf.Reset()
}

// Encode generates the output in M3U8 format and provides a pointer to its buffer.
func (p *MasterPlaylist) Encode() *bytes.Buffer {
	if p.buf.Len() > 0 {
		return &p.buf
	}

	p.buf.WriteString("#EXTM3U\n#EXT-X-VERSION:")
	p.buf.WriteString(strVer(p.ver))
	p.buf.WriteRune('\n')
	if p.ContentSteering != nil {
		writeContentSteering(&p.buf, p.ContentSteering)
	}

	if p.IndependentSegments() {
		p.buf.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	}

	if p.StartTime != 0.0 { // Both negative and positive values are allowed. Negative values are relative to the end.
		writeExtXStart(&p.buf, p.StartTime, p.StartTimePrecise, p.WritePrecision())
	}

	if len(p.Defines) > 0 {
		writeDefines(&p.buf, p.Defines)
	}

	for _, sd := range p.SessionDatas {
		writeSessionData(&p.buf, sd)
	}
	for _, key := range p.SessionKeys {
		writeKey("#EXT-X-SESSION-KEY:", &p.buf, key)
	}

	// Write any custom master tags
	if p.Custom != nil {
		for _, v := range p.Custom {
			if customBuf := v.Encode(); customBuf != nil {
				p.buf.WriteString(customBuf.String())
				p.buf.WriteRune('\n')
			}
		}
	}

	alts := p.GetAllAlternatives()
	for _, alt := range alts {
		writeExtXMedia(&p.buf, alt)
	}

	for _, vnt := range p.Variants {
		if vnt.Iframe {
			writeExtXIFrameStreamInf(&p.buf, vnt, p.WritePrecision())
		} else {
			writeExtXStreamInf(&p.buf, vnt, p.WritePrecision())
			p.buf.WriteString(vnt.URI)
			if p.Args != "" {
				if strings.Contains(vnt.URI, "?") {
					p.buf.WriteRune('&')
				} else {
					p.buf.WriteRune('?')
				}
				p.buf.WriteString(p.Args)
			}
			p.buf.WriteRune('\n')
		}
	}

	return &p.buf
}

// writeExtXMedia writes an EXT-X-MEDIA tag line including \n to the buffer.
// No checks are done that the date is valid.
func writeExtXMedia(buf *bytes.Buffer, alt *Alternative) {
	buf.WriteString("#EXT-X-MEDIA:")
	buf.WriteString("TYPE=")
	buf.WriteString(alt.Type)                 // Mandatory enumerated string
	writeQuoted(buf, "GROUP-ID", alt.GroupId) // Mandatory quoted-string
	writeQuoted(buf, "NAME", alt.Name)        // Mandatory quoted-string
	if alt.Language != "" {
		writeQuoted(buf, "LANGUAGE", alt.Language)
	}
	if alt.AssocLanguage != "" {
		writeQuoted(buf, "ASSOC-LANGUAGE", alt.AssocLanguage)
	}
	if alt.StableRenditionId != "" {
		writeQuoted(buf, "STABLE-RENDITION-ID", alt.StableRenditionId)
	}
	buf.WriteString(",DEFAULT=")
	writeYESorNO(buf, alt.Default)
	if alt.Autoselect {
		buf.WriteString(",AUTOSELECT=YES")
	}
	if alt.Forced {
		buf.WriteString(",FORCED=YES")
	}
	if alt.InstreamId != "" {
		writeQuoted(buf, "INSTREAM-ID", alt.InstreamId)
	}
	if alt.BitDepth != 0 {
		writeUint(buf, "BIT-DEPTH", uint(alt.BitDepth))
	}
	if alt.SampleRate != 0 {
		writeUint(buf, "SAMPLE-RATE", uint(alt.SampleRate))
	}
	if alt.Characteristics != "" {
		writeQuoted(buf, "CHARACTERISTICS", alt.Characteristics)
	}
	if alt.Channels != nil {
		writeChannels(buf, alt.Channels)
	}
	if alt.URI != "" {
		writeQuoted(buf, "URI", alt.URI)
	}
	buf.WriteRune('\n')
}

func writeExtXStreamInf(buf *bytes.Buffer, vnt *Variant, writePrecision int) {
	buf.WriteString("#EXT-X-STREAM-INF:BANDWIDTH=")
	buf.WriteString(strconv.FormatUint(uint64(vnt.Bandwidth), 10))
	if vnt.AverageBandwidth != 0 {
		buf.WriteString(",AVERAGE-BANDWIDTH=")
		buf.WriteString(strconv.FormatUint(uint64(vnt.AverageBandwidth), 10))
	}
	if vnt.Score > 0 {
		writeFloat(buf, "SCORE", vnt.Score, writePrecision)
	}
	if vnt.Codecs != "" {
		writeQuoted(buf, "CODECS", vnt.Codecs)
	}
	if vnt.SupplementalCodecs != "" {
		writeQuoted(buf, "SUPPLEMENTAL-CODECS", vnt.SupplementalCodecs)
	}
	if vnt.Resolution != "" {
		writeUnQuoted(buf, "RESOLUTION", vnt.Resolution)
	}
	if vnt.FrameRate != 0 {
		writeFloat(buf, "FRAME-RATE", vnt.FrameRate, writePrecision)
	}
	if vnt.HDCPLevel != "" {
		writeUnQuoted(buf, "HDCP-LEVEL", vnt.HDCPLevel)
	}
	if vnt.AllowedCPC != "" {
		writeQuoted(buf, "ALLOWED-CPC", vnt.AllowedCPC)
	}
	if vnt.VideoRange != "" {
		writeUnQuoted(buf, "VIDEO-RANGE", vnt.VideoRange)
	}
	if vnt.ReqVideoLayout != "" {
		writeQuoted(buf, "REQ-VIDEO-LAYOUT", vnt.ReqVideoLayout)
	}
	if vnt.StableVariantId != "" {
		writeQuoted(buf, "STABLE-VARIANT-ID", vnt.StableVariantId)
	}
	if vnt.Audio != "" {
		writeQuoted(buf, "AUDIO", vnt.Audio)
	}
	if vnt.Video != "" {
		writeQuoted(buf, "VIDEO", vnt.Video)
	}
	if vnt.Subtitles != "" {
		writeQuoted(buf, "SUBTITLES", vnt.Subtitles)
	}
	if vnt.Captions != "" {
		if vnt.Captions == "NONE" {
			writeUnQuoted(buf, "CLOSED-CAPTIONS", vnt.Captions)
		} else {
			writeQuoted(buf, "CLOSED-CAPTIONS", vnt.Captions)
		}
	}
	if vnt.PathwayId != "" {
		writeQuoted(buf, "PATHWAY-ID", vnt.PathwayId)
	}
	if vnt.ProgramId != nil { // Removed in version 6
		buf.WriteString(",PROGRAM-ID=")
		buf.WriteString(strconv.FormatUint(uint64(*vnt.ProgramId), 10))
	}
	if vnt.Name != "" {
		writeQuoted(buf, "NAME", vnt.Name)
	}
	buf.WriteRune('\n')
}

func writeExtXIFrameStreamInf(buf *bytes.Buffer, vnt *Variant, writePrecision int) {
	buf.WriteString("#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=")
	buf.WriteString(strconv.FormatUint(uint64(vnt.Bandwidth), 10))
	if vnt.AverageBandwidth != 0 {
		buf.WriteString(",AVERAGE-BANDWIDTH=")
		buf.WriteString(strconv.FormatUint(uint64(vnt.AverageBandwidth), 10))
	}
	if vnt.Score > 0 {
		writeFloat(buf, "SCORE", vnt.Score, writePrecision)
	}
	if vnt.Codecs != "" {
		writeQuoted(buf, "CODECS", vnt.Codecs)
	}
	if vnt.SupplementalCodecs != "" {
		writeQuoted(buf, "SUPPLEMENTAL-CODECS", vnt.SupplementalCodecs)
	}
	if vnt.Resolution != "" {
		writeUnQuoted(buf, "RESOLUTION", vnt.Resolution)
	}
	if vnt.HDCPLevel != "" {
		writeUnQuoted(buf, "HDCP-LEVEL", vnt.HDCPLevel)
	}
	if vnt.AllowedCPC != "" {
		writeQuoted(buf, "ALLOWED-CPC", vnt.AllowedCPC)
	}
	if vnt.VideoRange != "" {
		writeUnQuoted(buf, "VIDEO-RANGE", vnt.VideoRange)
	}
	if vnt.ReqVideoLayout != "" {
		writeQuoted(buf, "REQ-VIDEO-LAYOUT", vnt.ReqVideoLayout)
	}
	if vnt.StableVariantId != "" {
		writeQuoted(buf, "STABLE-VARIANT-ID", vnt.StableVariantId)
	}
	if vnt.Video != "" {
		writeQuoted(buf, "VIDEO", vnt.Video)
	}
	if vnt.PathwayId != "" {
		writeQuoted(buf, "PATHWAY-ID", vnt.PathwayId)
	}
	if vnt.ProgramId != nil { // Removed in version 6
		buf.WriteString(",PROGRAM-ID=")
		buf.WriteString(strconv.FormatInt(int64(*vnt.ProgramId), 10))
	}
	if vnt.Name != "" {
		writeQuoted(buf, "NAME", vnt.Name)
	}
	writeQuoted(buf, "URI", vnt.URI) // Mandatory
	buf.WriteRune('\n')
}

func writePartialSegment(buf *bytes.Buffer, ps *PartialSegment, writePrecision int) {
	if !ps.ProgramDateTime.IsZero() {
		buf.WriteString("#EXT-X-PROGRAM-DATE-TIME:")
		buf.WriteString(ps.ProgramDateTime.Format(DATETIME))
		buf.WriteRune('\n')
	}
	buf.WriteString("#EXT-X-PART:")
	buf.WriteString("DURATION=")
	buf.WriteString(strconv.FormatFloat(ps.Duration, 'f', writePrecision, 64))
	if ps.Independent {
		buf.WriteString(",INDEPENDENT=YES")
	}
	if ps.Gap {
		buf.WriteString(",GAP=")
		writeYESorNO(buf, ps.Gap)
	}
	if ps.Limit > 0 {
		writeRange(buf, ",BYTERANGE=", true, ps.Limit, ps.Offset)
	}
	buf.WriteString(",URI=\"")
	buf.WriteString(ps.URI)
	buf.WriteRune('"')
	buf.WriteRune('\n')
}

func writePreloadHint(buf *bytes.Buffer, ph *PreloadHint) {
	buf.WriteString("#EXT-X-PRELOAD-HINT:")
	buf.WriteString("TYPE=")
	buf.WriteString(ph.Type)
	buf.WriteString(",URI=\"")
	buf.WriteString(ph.URI)
	buf.WriteRune('"')
	if ph.Limit > 0 {
		buf.WriteString(",BYTERANGE-START=")
		buf.WriteString(strconv.FormatInt(ph.Offset, 10))
		buf.WriteString(",BYTERANGE-LENGTH=")
		buf.WriteString(strconv.FormatInt(ph.Limit, 10))
	}
	buf.WriteRune('\n')
}

func writeSkip(buf *bytes.Buffer, skippedSegments uint64) {
	buf.WriteString("#EXT-X-SKIP:")
	buf.WriteString("SKIPPED-SEGMENTS=")
	buf.WriteString(strconv.FormatUint(skippedSegments, 10))
	buf.WriteRune('\n')
}

func writeServerControl(buf *bytes.Buffer, sc *ServerControl, writePrecision int) {
	buf.WriteString("#EXT-X-SERVER-CONTROL:")
	stringsToWrite := []string{}
	if sc.CanSkipUntil > 0 {
		stringsToWrite = append(stringsToWrite, ("CAN-SKIP-UNTIL=" + strconv.FormatFloat(sc.CanSkipUntil,
			'f', writePrecision, 64)))
		if sc.CanSkipDateRanges {
			stringsToWrite = append(stringsToWrite, ("CAN-SKIP-DATERANGES=YES"))
		}
	}
	if sc.HoldBack > 0 {
		stringsToWrite = append(stringsToWrite, ("HOLD-BACK=" + strconv.FormatFloat(sc.HoldBack,
			'f', writePrecision, 64)))
	}
	if sc.PartHoldBack > 0 {
		stringsToWrite = append(stringsToWrite, ("PART-HOLD-BACK=" + strconv.FormatFloat(sc.PartHoldBack,
			'f', writePrecision, 64)))
	}
	if sc.CanBlockReload {
		stringsToWrite = append(stringsToWrite, ("CAN-BLOCK-RELOAD=YES"))
	}

	joinedString := strings.Join(stringsToWrite, ",")
	buf.WriteString(joinedString)
	buf.WriteRune('\n')
}

// writeDateRange writes an EXT-X-DATERANGE tag line including \n to the buffer.
func writeDateRange(buf *bytes.Buffer, dr *DateRange, writePrecision int) {
	buf.WriteString(`#EXT-X-DATERANGE:ID="`)
	buf.WriteString(dr.ID)
	buf.WriteRune('"')
	if dr.Class != "" {
		writeQuoted(buf, "CLASS", dr.Class)
	}
	str := dr.StartDate.Format(DATETIME)
	writeQuoted(buf, "START-DATE", str)
	if dr.Cue != "" {
		writeUnQuoted(buf, "CUE", dr.Cue)
	}
	if dr.EndDate != nil {
		str = dr.EndDate.Format(DATETIME)
		writeQuoted(buf, "END-DATE", str)
	}
	if dr.Duration != nil {
		writeFloat(buf, "DURATION", *dr.Duration, writePrecision)
	}
	if dr.PlannedDuration != nil {
		writeFloat(buf, "PLANNED-DURATION", *dr.PlannedDuration, writePrecision)
	}
	if dr.SCTE35Cmd != "" {
		writeUnQuoted(buf, "SCTE35-CMD", dr.SCTE35Cmd)
	}
	if dr.SCTE35Out != "" {
		writeUnQuoted(buf, "SCTE35-OUT", dr.SCTE35Out)
	}
	if dr.SCTE35In != "" {
		writeUnQuoted(buf, "SCTE35-IN", dr.SCTE35In)
	}
	if dr.EndOnNext {
		buf.WriteString(",END-ON-NEXT=YES")
	}
	for _, xa := range dr.XAttrs {
		writeUnQuoted(buf, xa.Key, xa.Val)
	}
	buf.WriteRune('\n')
}

func writeDefines(buf *bytes.Buffer, defines []Define) {
	for _, d := range defines {
		buf.WriteString("#EXT-X-DEFINE:")
		switch d.Type {
		case VALUE:
			buf.WriteString("NAME=\"")
			buf.WriteString(d.Name)
			buf.WriteString("\",VALUE=\"")
			buf.WriteString(d.Value)
			buf.WriteString("\"")
		case IMPORT:
			buf.WriteString("IMPORT=\"")
			buf.WriteString(d.Name)
			buf.WriteString("\"")
		case QUERYPARAM:
			buf.WriteString("QUERYPARAM=\"")
			buf.WriteString(d.Name)
			buf.WriteString("\"")
		}
		buf.WriteRune('\n')
	}
}

func writeSessionData(buf *bytes.Buffer, sd *SessionData) {
	buf.WriteString("#EXT-X-SESSION-DATA:DATA-ID=\"")
	buf.WriteString(sd.DataId)
	buf.WriteRune('"')
	if sd.Value != "" {
		writeQuoted(buf, "VALUE", sd.Value)
	}
	if sd.URI != "" {
		writeQuoted(buf, "URI", sd.URI)
	}
	if sd.Format != "JSON" {
		writeUnQuoted(buf, "FORMAT", sd.Format)
	}
	if sd.Language != "" {
		writeQuoted(buf, "LANGUAGE", sd.Language)
	}
	buf.WriteRune('\n')
}

func writeExtXStart(buf *bytes.Buffer, startTime float64, precise bool, writePrecision int) {
	buf.WriteString("#EXT-X-START:TIME-OFFSET=")
	writeFloatValue(buf, startTime, writePrecision)
	if precise {
		buf.WriteString(",PRECISE=YES")
	}
	buf.WriteRune('\n')
}

func writeExtXMap(buf *bytes.Buffer, m *Map) {
	buf.WriteString("#EXT-X-MAP:")
	buf.WriteString("URI=\"")
	buf.WriteString(m.URI)
	buf.WriteRune('"')
	if m.Limit > 0 {
		writeRange(buf, ",BYTERANGE=", true, m.Limit, m.Offset)
	}
	buf.WriteRune('\n')
}

func writeKey(tag string, buf *bytes.Buffer, key *Key) {
	buf.WriteString(tag)
	buf.WriteString("METHOD=")
	buf.WriteString(key.Method)
	if key.Method != "NONE" {
		writeQuoted(buf, "URI", key.URI)
		if key.IV != "" {
			writeUnQuoted(buf, "IV", key.IV)
		}
		if key.Keyformat != "" {
			writeQuoted(buf, "KEYFORMAT", key.Keyformat)
		}
		if key.Keyformatversions != "" {
			writeQuoted(buf, "KEYFORMATVERSIONS", key.Keyformatversions)
		}
	}
	buf.WriteRune('\n')
}

func writeContentSteering(buf *bytes.Buffer, cs *ContentSteering) {
	buf.WriteString(`#EXT-X-CONTENT-STEERING:SERVER-URI="`)
	buf.WriteString(cs.ServerURI)
	buf.WriteRune('"')
	if cs.PathwayId != "" {
		writeQuoted(buf, "PATHWAY-ID", cs.PathwayId)
	}
	buf.WriteRune('\n')
}

func writeRange(buf *bytes.Buffer, tag string, quoted bool, limit, offset int64) {
	buf.WriteString(tag)
	if quoted {
		buf.WriteRune('"')
	}
	buf.WriteString(strconv.FormatInt(limit, 10))
	buf.WriteRune('@')
	buf.WriteString(strconv.FormatInt(offset, 10))
	if quoted {
		buf.WriteRune('"')
	}
}

// writeQuoted writes a quoted key-value pair to the buffer preceded by a comma.
func writeQuoted(buf *bytes.Buffer, key, value string) {
	buf.WriteRune(',')
	buf.WriteString(key)
	buf.WriteString(`="`)
	buf.WriteString(value)
	buf.WriteRune('"')
}

func writeUnQuoted(buf *bytes.Buffer, key, value string) {
	buf.WriteRune(',')
	buf.WriteString(key)
	buf.WriteRune('=')
	buf.WriteString(value)
}

// writeUint writes a key-value pair to the buffer preceded by a comma.
func writeUint(buf *bytes.Buffer, key string, value uint) {
	buf.WriteRune(',')
	buf.WriteString(key)
	buf.WriteRune('=')
	buf.WriteString(strconv.FormatUint(uint64(value), 10))
}

func writeFloatValue(buf *bytes.Buffer, value float64, writePrecision int) {
	buf.WriteString(strconv.FormatFloat(value, 'f', writePrecision, 64))
}

func writeFloat(buf *bytes.Buffer, key string, value float64, writePrecision int) {
	if key != "" {
		buf.WriteRune(',')
		buf.WriteString(key)
		buf.WriteRune('=')
	}
	writeFloatValue(buf, value, writePrecision)
}

func writeYESorNO(buf *bytes.Buffer, b bool) {
	if b {
		buf.WriteString("YES")
	} else {
		buf.WriteString("NO")
	}
}

func writeChannels(buf *bytes.Buffer, channels *Channels) {
	buf.WriteRune(',')
	buf.WriteString("CHANNELS=\"")
	buf.WriteString(strconv.Itoa(channels.Amount))
	if channels.SpatialAudioIdentifiers != "" {
		buf.WriteRune('/')
		buf.WriteString(channels.SpatialAudioIdentifiers)
	}
	if channels.ChannelUsageIndicators != "" {
		buf.WriteRune('/')
		buf.WriteString(channels.ChannelUsageIndicators)
	}
	buf.WriteRune('"')
}

// SetCustomTag sets the provided tag on the master playlist for its TagName
func (p *MasterPlaylist) SetCustomTag(tag CustomTag) {
	if p.Custom == nil {
		p.Custom = make(CustomMap)
	}

	p.Custom[tag.TagName()] = tag
}

// IndependentSegments returns true if all media samples in a segment can be
// decoded without information from other buf.
func (p *MasterPlaylist) IndependentSegments() bool {
	return p.independentSegments
}

// SetIndependentSegments sets the master playlist #EXT-X--INDEPENDENT-SEGMENTS tag.
func (p *MasterPlaylist) SetIndependentSegments(b bool) {
	p.independentSegments = b
}

// String provides the playlist fulfilling the Stringer interface.
func (p *MasterPlaylist) String() string {
	return p.Encode().String()
}

// GetAllAlternatives returns all alternative renditions sorted by
// groupID, type, name, and language.
func (p *MasterPlaylist) GetAllAlternatives() []*Alternative {
	added := make(map[string]*Alternative)

	for _, v := range p.Variants {
		for _, alt := range v.Alternatives {
			key := fmt.Sprintf("%s-%s-%s-%s", alt.GroupId, alt.Type, alt.Name, alt.Language)
			if _, ok := added[key]; !ok {
				added[key] = alt
			}
		}
	}
	alts := make([]*Alternative, 0, len(added))
	keys := make([]string, 0, len(added))
	for k := range added {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		alts = append(alts, added[key])
	}
	return alts
}

// NewMediaPlaylist creates a new media playlist structure.
// Winsize defines live window for playlist generation. Set to zero for VOD or EVENT
// playlists.  Capacity is the total size of the backing segment list..
// For VOD playlists, call Close() after the last segment is added.
func NewMediaPlaylist(winsize uint, capacity uint) (*MediaPlaylist, error) {
	if capacity < winsize {
		return nil, ErrWinSizeTooSmall
	}
	p := &MediaPlaylist{
		Segments: getSegmentSlice(capacity),
		buf:      getBuffer(),
	}
	p.ver = minVer
	p.winsize = winsize
	p.capacity = capacity
	p.writePrecision = DefaultFloatPrecision
	return p, nil
}

// ReleasePlaylist returns ,buffer, segment slice, and all segments to pool for reuse.
// Do not use the playlist or segments after this
func (p *MediaPlaylist) ReleasePlaylist() {
	for _, s := range p.Segments {
		releaseSegment(s)
	}
	putBuffer(&p.buf)
	putSegmentSlice(&p.Segments)
}

// last returns the previously written segment's index
func (p *MediaPlaylist) last() uint {
	if p.tail == 0 {
		return p.capacity - 1
	}
	return p.tail - 1
}

func (p *MediaPlaylist) SetIndependentSegments(b bool) {
	p.independentSegments = b
}

func (p *MediaPlaylist) IndependentSegments() bool {
	return p.independentSegments
}

// Remove removes the first/oldest segment which is at the head of chunk slice form a media playlist.
// Useful for sliding playlists.
// This operation resets playlist cache.
func (p *MediaPlaylist) Remove() (err error) {
	if p.count == 0 {
		return ErrPlaylistEmpty
	}
	p.head = (p.head + 1) % p.capacity
	p.count--
	if !p.Closed {
		p.SeqNo++
	}
	p.buf.Reset()
	return nil
}

// Append general chunk to the tail of chunk slice for a media playlist.
// This operation resets playlist cache.
func (p *MediaPlaylist) Append(uri string, duration float64, title string) error {
	seg := GetSegment()
	seg.URI = uri
	seg.Duration = duration
	seg.Title = title
	return p.AppendSegment(seg)
}

// AppendSegment appends a MediaSegment to the tail of chunk slice for
// a media playlist.  This operation resets playlist cache.
func (p *MediaPlaylist) AppendSegment(seg *MediaSegment) error {
	if p.head == p.tail && p.count > 0 {
		return ErrPlaylistFull
	}
	seg.SeqId = p.SeqNo
	if p.count > 0 {
		seg.SeqId = p.Segments[(p.capacity+p.tail-1)%p.capacity].SeqId + 1
	}
	p.Segments[p.tail] = seg
	p.tail = (p.tail + 1) % p.capacity
	p.count++
	p.SegmentIndexing.NextMSNIndex++
	p.SegmentIndexing.NextPartIndex = 0
	if !p.targetDurLocked {
		p.TargetDuration = calcNewTargetDuration(seg.Duration, p.ver, p.TargetDuration)
	}
	if seg.SCTE != nil {
		p.scte35Syntax = seg.SCTE.Syntax
	}
	if len(seg.SCTE35DateRanges) > 0 {
		p.scte35Syntax = SCTE35_DATERANGE
	}
	p.buf.Reset()
	return nil
}

// AppendPartial creates and and appends a partial segment to the media playlist.
func (p *MediaPlaylist) AppendPartial(uri string, duration float64, independent bool) error {
	seg := new(PartialSegment)
	seg.URI = uri
	seg.Duration = duration
	seg.Independent = independent
	return p.AppendPartialSegment(seg)
}

// AppendPartialSegment appends a PartialSegment to the MediaPlaylist.
// If the partial segment belongs to the last full segment, it assigns the same SeqID.
// Otherwise, it assigns the SeqID of the next segment.
// The MaxPartIndex is updated if necessary, and the NextPartIndex is incremented.
// Finally, it removes any expired partial segments.
func (p *MediaPlaylist) AppendPartialSegment(ps *PartialSegment) error {
	if p.count == 0 {
		return ErrPlaylistEmpty
	}

	// Check if the partial segment belongs to the last full segment
	fullSegUri := p.Segments[p.last()].URI
	if isPartOf(ps.URI, fullSegUri) {
		ps.SeqID = p.Segments[p.last()].SeqId
	} else {
		// It belongs to the next segment
		ps.SeqID = p.Segments[p.last()].SeqId + 1
	}

	p.PartialSegments = append(p.PartialSegments, ps)
	if p.SegmentIndexing.MaxPartIndex < p.SegmentIndexing.NextPartIndex {
		p.SegmentIndexing.MaxPartIndex = p.SegmentIndexing.NextPartIndex
	}
	p.SegmentIndexing.NextPartIndex++
	p.removeExpiredPartials()

	return nil
}

func (p *MediaPlaylist) removeExpiredPartials() {
	if len(p.PartialSegments) == 0 {
		return
	}

	// Last full segment
	segId := p.Segments[p.last()].SeqId
	var validPartialSegments []*PartialSegment
	for _, ps := range p.PartialSegments {
		if segId < 3 {
			// Keep all partial segments if we have less than 3 full segments
			validPartialSegments = append(validPartialSegments, ps)
		} else if ps.SeqID > segId-3 {
			// Keep partial segments that belong to the last 3 full segments
			validPartialSegments = append(validPartialSegments, ps)
		} else {
			// This partial segment is older than the last 3 segments
			// and should be removed
			continue
		}
	}

	p.PartialSegments = validPartialSegments
}

func (p *MediaPlaylist) SetPreloadHint(hintType, uri string) {
	preloadHint := new(PreloadHint)
	preloadHint.Type = hintType
	preloadHint.URI = uri
	p.PreloadHints = preloadHint
}

func (p *MediaPlaylist) AppendDefine(d Define) {
	p.Defines = append(p.Defines, d)
}

// Slide combines two operations: first it removes one chunk from
// the head of chunk slice and move pointer to next chunk. Secondly it
// appends one chunk to the tail of chunk slice. Useful for sliding
// playlists.  This operation resets the cache.
func (p *MediaPlaylist) Slide(uri string, duration float64, title string) {
	if !p.Closed && p.count >= p.winsize {
		_ = p.Remove()
	}
	_ = p.Append(uri, duration, title)
}

// ResetCache resets playlist cache (internal buffer).
// Next call to Encode() fills buffer/cache again.
func (p *MediaPlaylist) ResetCache() {
	p.buf.Reset()
}

// Encode generates output and returns a pointer to an internal buffer.
// If winsize > 0, encoded the last `winsize` segments, otherwise encode all segments.
// If already encoded, and not changed, the cached buffer will be returned.
// Don't change the buffer externally, e.g. by using the Write() method
// if you want to use the cached value. Instead use the String() or Bytes() methods.
func (p *MediaPlaylist) encode(segmentsToSkipInTotal uint64) *bytes.Buffer {
	if p.buf.Len() > 0 {
		return &p.buf
	}

	var lastMap *Map

	p.buf.WriteString("#EXTM3U\n#EXT-X-VERSION:")
	p.buf.WriteString(strVer(p.ver))
	p.buf.WriteRune('\n')

	if p.IndependentSegments() {
		p.buf.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	}

	// Write any custom master tags
	if p.Custom != nil {
		for _, v := range p.Custom {
			if customBuf := v.Encode(); customBuf != nil {
				p.buf.WriteString(customBuf.String())
				p.buf.WriteRune('\n')
			}
		}
	}

	if p.AllowCache != nil {
		p.buf.WriteString("#EXT-X-ALLOW-CACHE:")
		writeYESorNO(&p.buf, *p.AllowCache)
		p.buf.WriteRune('\n')
	}

	if len(p.Defines) > 0 {
		writeDefines(&p.buf, p.Defines)
	}

	// default key before any segment
	if len(p.Keys) != 0 {
		for _, key := range p.Keys {
			writeKey("#EXT-X-KEY:", &p.buf, &key)
		}
	}

	if p.MediaType > 0 {
		p.buf.WriteString("#EXT-X-PLAYLIST-TYPE:")
		switch p.MediaType {
		case EVENT:
			p.buf.WriteString("EVENT\n")
		case VOD:
			p.buf.WriteString("VOD\n")
		}
	}

	if p.ServerControl != nil {
		writeServerControl(&p.buf, p.ServerControl, p.WritePrecision())
	}

	if p.PartTargetDuration > 0 {
		p.buf.WriteString("#EXT-X-PART-INF:PART-TARGET=")
		p.buf.WriteString(strconv.FormatFloat(p.PartTargetDuration, 'f', p.WritePrecision(), 64))
		p.buf.WriteRune('\n')
	}
	p.buf.WriteString("#EXT-X-MEDIA-SEQUENCE:")
	p.buf.WriteString(strconv.FormatUint(p.SeqNo, 10))
	p.buf.WriteRune('\n')
	p.buf.WriteString("#EXT-X-TARGETDURATION:")
	p.buf.WriteString(strconv.FormatInt(int64(p.TargetDuration), 10))
	p.buf.WriteRune('\n')
	if p.StartTime != 0.0 { // Both negative and positive values are allowed. Negative values are relative to the end.
		writeExtXStart(&p.buf, p.StartTime, p.StartTimePrecise, p.WritePrecision())
	}
	if p.DiscontinuitySeq != 0 {
		p.buf.WriteString("#EXT-X-DISCONTINUITY-SEQUENCE:")
		p.buf.WriteString(strconv.FormatUint(uint64(p.DiscontinuitySeq), 10))
		p.buf.WriteRune('\n')
	}
	if p.Iframe {
		p.buf.WriteString("#EXT-X-I-FRAMES-ONLY\n")
	}

	if segmentsToSkipInTotal > 0 {
		writeSkip(&p.buf, segmentsToSkipInTotal)
	} else {
		// Ignore the Media Initialization Section (EXT-X-MAP) tag
		// in presence of skip (EXT-X-SKIP) tag
		if p.Map != nil {
			writeExtXMap(&p.buf, p.Map)
		}
		lastMap = p.Map
	}

	var (
		seg           *MediaSegment
		durationCache = make(map[float64]string)
	)

	head := p.head
	tail := p.tail
	count := p.count
	isVoDOrEvent := p.winsize == 0
	segmentsSkipped := p.SkippedSegments()
	var outputCount uint     // number of segments to output
	var start uint           // start index of segments to output
	var lastSegId uint64 = 0 // last segment sequence number in live playlist
	if isVoDOrEvent {
		// for VoD playlists, output all segments
		outputCount = count
		start = head
	} else {
		// for Live playlists, output the last winsize segments
		outputCount = min(p.winsize, count)
		start = head + count - outputCount
		if tail > 0 {
			lastSegId = p.Segments[tail-1].SeqId
		}
	}

	// shift head to start
	p.head = start
	// output segments
	for i := start; i < start+outputCount; i++ {
		seg = p.Segments[i]
		if seg == nil { // protection from badly filled chunklists
			continue
		}
		if segmentsSkipped < segmentsToSkipInTotal {
			segmentsSkipped += 1
			continue
		}
		if seg.Discontinuity {
			p.buf.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		if seg.SCTE != nil {
			switch seg.SCTE.Syntax {
			case SCTE35_67_2014:
				p.buf.WriteString("#EXT-SCTE35:")
				p.buf.WriteString("CUE=\"")
				p.buf.WriteString(seg.SCTE.Cue)
				p.buf.WriteRune('"')
				if seg.SCTE.ID != "" {
					p.buf.WriteString(",ID=\"")
					p.buf.WriteString(seg.SCTE.ID)
					p.buf.WriteRune('"')
				}
				if seg.SCTE.Time != 0 {
					p.buf.WriteString(",TIME=")
					writeFloatValue(&p.buf, seg.SCTE.Time, p.WritePrecision())
				}
				p.buf.WriteRune('\n')
			case SCTE35_OATCLS:
				switch seg.SCTE.CueType {
				case SCTE35Cue_Start:
					if seg.SCTE.Cue != "" {
						p.buf.WriteString("#EXT-OATCLS-SCTE35:")
						p.buf.WriteString(seg.SCTE.Cue)
						p.buf.WriteRune('\n')
					}
					p.buf.WriteString("#EXT-X-CUE-OUT:")
					writeFloatValue(&p.buf, seg.SCTE.Time, p.WritePrecision())
					p.buf.WriteRune('\n')
				case SCTE35Cue_Mid:
					p.buf.WriteString("#EXT-X-CUE-OUT-CONT:ElapsedTime=")
					writeFloatValue(&p.buf, seg.SCTE.Elapsed, p.WritePrecision())
					p.buf.WriteString(",Duration=")
					writeFloatValue(&p.buf, seg.SCTE.Time, p.WritePrecision())
					p.buf.WriteString(",SCTE35=")
					p.buf.WriteString(seg.SCTE.Cue)
					p.buf.WriteRune('\n')
				case SCTE35Cue_End:
					p.buf.WriteString("#EXT-X-CUE-IN\n")
				}
			}
		}
		for i := range seg.SCTE35DateRanges {
			writeDateRange(&p.buf, seg.SCTE35DateRanges[i], p.WritePrecision())
		}

		// check for key change
		if len(seg.Keys) != 0 && (p.Keys == nil || !slices.Equal(seg.Keys, p.Keys)) {
			for _, key := range seg.Keys {
				writeKey("#EXT-X-KEY:", &p.buf, &key)
			}
		}
		if seg.Gap {
			p.buf.WriteString("#EXT-X-GAP\n")
		}
		// ignore segment Map if already written
		if seg.Map != nil && !seg.Map.Equal(lastMap) {
			writeExtXMap(&p.buf, seg.Map)
			lastMap = seg.Map
		}
		if !seg.ProgramDateTime.IsZero() {
			p.buf.WriteString("#EXT-X-PROGRAM-DATE-TIME:")
			p.buf.WriteString(seg.ProgramDateTime.Format(DATETIME))
			p.buf.WriteRune('\n')
		}
		// handle completed partial segments
		if p.HasPartialSegments() {
			fullSegUri := seg.URI
			var remainingPartialSegments []*PartialSegment
			for _, ps := range p.PartialSegments {
				if isPartOf(ps.URI, fullSegUri) {
					// This partial segment is part of the current full segment
					writePartialSegment(&p.buf, ps, p.WritePrecision())
				} else {
					// This partial segment does not belong to current full segment
					// Keep it to be written later
					remainingPartialSegments = append(remainingPartialSegments, ps)
				}
			}
			// Update the PartialSegments list to exclude the completed ones
			p.PartialSegments = remainingPartialSegments
		}

		if seg.Limit > 0 {
			writeRange(&p.buf, "#EXT-X-BYTERANGE:", false, seg.Limit, seg.Offset)
			p.buf.WriteRune('\n')
		}

		// Add Custom Segment Tags here
		if seg.Custom != nil {
			for _, v := range seg.Custom {
				if customBuf := v.Encode(); customBuf != nil {
					p.buf.WriteString(customBuf.String())
					p.buf.WriteRune('\n')
				}
			}
		}

		writeExtInfWithCache(&p.buf, seg.Duration, seg.Title, p.WritePrecision(), durationCache)

		p.buf.WriteString(seg.URI)
		if p.Args != "" {
			p.buf.WriteRune('?')
			p.buf.WriteString(p.Args)
		}
		p.buf.WriteRune('\n')
	}

	// handle remaining partial segments
	if p.HasPartialSegments() {
		for _, ps := range p.PartialSegments {
			if ps.SeqID >= lastSegId {
				// This partial segment is part of the next segment
				writePartialSegment(&p.buf, ps, p.WritePrecision())
			} else {
				// This partial segment does not belong to any segment
				// and should be ignored
				continue
			}
		}
	}

	if p.PreloadHints != nil {
		writePreloadHint(&p.buf, p.PreloadHints)
	}

	if p.Closed {
		p.buf.WriteString("#EXT-X-ENDLIST\n")
	}
	for _, dr := range p.DateRanges {
		writeDateRange(&p.buf, dr, p.WritePrecision())
	}
	return &p.buf
}

// EncodeWithSkip sets the skip tag and encodes the playlist.
// If skipped > 0, the first `skipped` segments will be skipped.
// If playlist has a skip tag already, it will return an error.
func (p *MediaPlaylist) EncodeWithSkip(skipped uint64) (*bytes.Buffer, error) {
	if p.SkippedSegments() > 0 {
		return nil, ErrAlreadySkipped
	}

	return p.encode(skipped), nil
}

func (p *MediaPlaylist) Encode() *bytes.Buffer {
	return p.encode(p.SkippedSegments())

}

// writeExtInfWithCache writes the EXTINF tag and value to the buffer.
// If the duration is already in the cache, it uses the cached value.
// Otherwise, it formats the duration and caches it.
func writeExtInfWithCache(buf *bytes.Buffer, duration float64, title string, writePrecision int,
	cache map[float64]string) {
	buf.WriteString("#EXTINF:")
	if str, ok := cache[duration]; ok {
		buf.WriteString(str)
	} else {
		fstr := strconv.FormatFloat(duration, 'f', writePrecision, 64)
		cache[duration] = fstr
		buf.WriteString(fstr)
	}
	buf.WriteRune(',')
	buf.WriteString(title)
	buf.WriteRune('\n')
}

// String provides the playlist fulfilling the Stringer interface.
func (p *MediaPlaylist) String() string {
	return p.Encode().String()
}

// Count tells us the number of items that are currently in the media playlist.
func (p *MediaPlaylist) Count() uint {
	return p.count
}

func (p *MediaPlaylist) HasPartialSegments() bool {
	return len(p.PartialSegments) > 0
}

func (p *MediaPlaylist) TotalDuration() float64 {
	totalDuration := float64(p.TargetDuration * p.count)

	return totalDuration
}

// Close sliding playlist and by setting the EXT-X-ENDLIST tag and setting the Closed flag.
func (p *MediaPlaylist) Close() {
	if p.buf.Len() > 0 {
		p.buf.WriteString("#EXT-X-ENDLIST\n")
	}
	p.Closed = true
}

// CalculateTargetDuration calculates the target duration for the playlist.
// For HLS v5 and earlier, it is the maximum segment duration as rounded up.
// For HLS v6 and later, it is the maximum segment duration as rounded to the nearest integer.
// It is not allowed to change when the playlist is updated.
func (p *MediaPlaylist) CalculateTargetDuration(hlsVer uint8) uint {
	if p.count == 0 {
		return 0
	}
	var max float64
	if p.tail >= p.head {
		for i := p.head; i < p.tail; i++ {
			if p.Segments[i].Duration > max {
				max = p.Segments[i].Duration
			}
		}
	} else {
		for i := p.head; i < p.capacity; i++ {
			if p.Segments[i].Duration > max {
				max = p.Segments[i].Duration
			}
		}
		for i := uint(0); i < p.tail; i++ {
			if p.Segments[i].Duration > max {
				max = p.Segments[i].Duration
			}
		}
	}
	return calcNewTargetDuration(max, hlsVer, 0)
}

// calcNewTargetDuration calculates a new target duration based on a segment duration.
// Per rfc8216bis the EXT-X-TARGETDURATION value MUST be at least 1, so a segment that
// would otherwise round down to 0 still yields a target duration of 1.
func calcNewTargetDuration(segDur float64, hlsVer uint8, oldTargetDuration uint) uint {
	var new uint
	if hlsVer < 6 {
		new = uint(math.Ceil(segDur))
	} else {
		new = uint(math.Round(segDur))
	}
	if new < 1 {
		new = 1
	}
	if new > oldTargetDuration {
		return new
	}
	return oldTargetDuration
}

// SetTargetDuration sets the target duration for the playlist and stops automatic calculation.
// Since the target duration is not allowed to change, it is locked after the first call.
// Per rfc8216bis the EXT-X-TARGETDURATION value MUST be at least 1, so a value of 0 is raised to 1.
func (p *MediaPlaylist) SetTargetDuration(duration uint) {
	if duration < 1 {
		duration = 1
	}
	p.TargetDuration = duration
	p.targetDurLocked = true
}

// SetDefaultKey sets encryption key to appear before segments in the media playlist.
func (p *MediaPlaylist) SetDefaultKey(method, uri, iv, keyformat, keyformatversions string) error {
	if keyformat != "" || keyformatversions != "" {
		updateVersion(&p.ver, 5) // [Protocol Version Compatibility]
	}
	p.Keys = append(p.Keys, Key{method, uri, iv, keyformat, keyformatversions})
	return nil
}

// SetDefaultMap sets default Media Initialization Section (EXT-X-MAP)
// at start of playlist. May be overridden by individual segments.
func (p *MediaPlaylist) SetDefaultMap(uri string, limit, offset int64) {
	updateVersion(&p.ver, 5) // [Protocol Version Compatibility]
	p.Map = &Map{uri, limit, offset}
}

// SetIframeOnly marks medialist of only I-frames (Intra frames).
func (p *MediaPlaylist) SetIframeOnly() {
	updateVersion(&p.ver, 4) // [Protocol Version Compatibility]
	p.Iframe = true
}

// SetKey sets encryption key for the current (and following) segment of media playlist
func (p *MediaPlaylist) SetKey(method, uri, iv, keyformat, keyformatversions string) error {
	if p.count == 0 {
		return ErrPlaylistEmpty
	}

	if keyformat != "" || keyformatversions != "" {
		updateVersion(&p.ver, 5) // [Protocol Version Compatibility]
	}

	p.Segments[p.last()].Keys = append(p.Segments[p.last()].Keys, Key{method, uri, iv, keyformat, keyformatversions})
	return nil
}

// SetMap sets map for the currently last segment of media playlist.
func (p *MediaPlaylist) SetMap(uri string, limit, offset int64) error {
	if p.count == 0 {
		return ErrPlaylistEmpty
	}
	updateVersion(&p.ver, 5) // [Protocol Version Compatibility]
	p.Segments[p.last()].Map = &Map{uri, limit, offset}
	return nil
}

// SetRange sets byte range limit and offset for the currently last media segment.
func (p *MediaPlaylist) SetRange(limit, offset int64) error {
	if p.count == 0 {
		return ErrPlaylistEmpty
	}
	updateVersion(&p.ver, 4) // [Protocol Version Compatibility]
	p.Segments[p.last()].Limit = limit
	p.Segments[p.last()].Offset = offset
	return nil
}

// SetSCTE sets the SCTE cue format for the currently last media segment.
//
// Deprecated: Use SetSCTE35 instead.
func (p *MediaPlaylist) SetSCTE(cue string, id string, time float64) error {
	return p.SetSCTE35(&SCTE{Syntax: SCTE35_67_2014, Cue: cue, ID: id, Time: time})
}

// SetSCTE35 sets the SCTE cue format for the currently last media segment.
func (p *MediaPlaylist) SetSCTE35(scte35 *SCTE) error {
	if p.count == 0 {
		return ErrPlaylistEmpty
	}
	p.Segments[p.last()].SCTE = scte35
	return nil
}

// SetDiscontinuity sets discontinuity flag for the currently last media segment.
// EXT-X-DISCONTINUITY indicates an encoding discontinuity
// between the media segment that follows it and the one that preceded it.
func (p *MediaPlaylist) SetDiscontinuity() error {
	if p.count == 0 {
		return ErrPlaylistEmpty
	}
	p.Segments[p.last()].Discontinuity = true
	return nil
}

// SetGap sets the gap flag for the currently last media segment.
// The EXT-X-GAP tag indicates that the segment URI to which it applies
// does not contain media data and SHOULD NOT be loaded by clients.
// It applies only to the latest Media Segment.
func (p *MediaPlaylist) SetGap() error {
	if p.count == 0 {
		return ErrPlaylistEmpty
	}
	p.Segments[p.last()].Gap = true
	return nil
}

// SetProgramDateTime sets program date and time for the currently last media segment.
// EXT-X-PROGRAM-DATE-TIME tag associates the first sample of
// a media segment with an absolute date and/or time. It applies only
// to the current media segment.  Date/time format is
// YYYY-MM-DDThh:mm:ssZ (ISO8601) and includes time zone.
func (p *MediaPlaylist) SetProgramDateTime(value time.Time) error {
	if p.count == 0 {
		return ErrPlaylistEmpty
	}
	p.Segments[p.last()].ProgramDateTime = value
	return nil
}

// SetCustomTag sets the provided tag on the media playlist for its TagName.
func (p *MediaPlaylist) SetCustomTag(tag CustomTag) {
	if p.Custom == nil {
		p.Custom = make(CustomMap)
	}

	p.Custom[tag.TagName()] = tag
}

// SetSkipped sets the number of segments that have been skipped in the playlist.
// This method should only be used when converting some custom representation to a MediaPlaylist,
// and the skipping of segments has already been handled.
func (p *MediaPlaylist) SetSkipped(skipped uint64) {
	p.skippedSegments = skipped
}

// SetCustomSegmentTag sets the provided tag on the current media segment for its TagName.
func (p *MediaPlaylist) SetCustomSegmentTag(tag CustomTag) error {
	if p.count == 0 {
		return ErrPlaylistEmpty
	}

	last := p.Segments[p.last()]

	if last.Custom == nil {
		last.Custom = make(CustomMap)
	}

	last.Custom[tag.TagName()] = tag

	return nil
}

// WinSize returns the playlist's window size.
func (p *MediaPlaylist) WinSize() uint {
	return p.winsize
}

// SetWinSize overwrites the playlist's window size.
func (p *MediaPlaylist) SetWinSize(winsize uint) error {
	if winsize > p.capacity {
		return fmt.Errorf("capacity=%d < winsize=%d: %w", p.capacity, winsize, ErrWinSizeTooSmall)
	}
	p.winsize = winsize
	return nil
}

func (p *MediaPlaylist) SetServerControl(control *ServerControl) error {
	if control.CanSkipUntil > 0 {
		skipUntil := control.CanSkipUntil
		skippedSegments := uint(math.Floor(skipUntil / float64(p.TargetDuration)))
		if skippedSegments > 0 {
			// if we want to skip the first N segments, we need to have at least N+1 winsize
			if p.winsize <= skippedSegments {
				return fmt.Errorf("winsize=%d <= skippedSegments=%d: %w",
					p.winsize, skippedSegments, ErrWinSizeTooSmall)
			}
		}
	}

	p.ServerControl = control
	return nil
}

// GetAllSegments could get all segments currently added to playlist.
// Winsize is ignored.
func (p *MediaPlaylist) GetAllSegments() []*MediaSegment {
	if p.count == 0 {
		return nil
	}
	buf := make([]*MediaSegment, 0, p.count)
	if p.head < p.tail {
		for i := p.head; i < p.tail; i++ {
			buf = append(buf, p.Segments[i])
		}
		return buf
	}
	for i := p.head; i < p.capacity; i++ {
		buf = append(buf, p.Segments[i])
	}
	for i := uint(0); i < p.tail; i++ {
		buf = append(buf, p.Segments[i])
	}
	return buf
}

// SetWritePrecision sets the number of decimal places used when writing float values.
// Default is 3 (milliseconds). Set to -1 to use the necessary number of decimal places.
func (p *MediaPlaylist) SetWritePrecision(nrDecimals int) {
	p.writePrecision = nrDecimals
	p.ResetCache()
}

// WritePrecision returns the current write precision for float values.
func (p *MediaPlaylist) WritePrecision() int {
	return p.writePrecision
}

// SetWritePrecision sets the number of decimal places used when writing float values.
// Default is 3 (milliseconds). Set to -1 to use the necessary number of decimal places.
func (p *MasterPlaylist) SetWritePrecision(nrDecimals int) {
	p.writePrecision = nrDecimals
	p.buf.Reset()
}

// WritePrecision returns the current write precision for float values.
func (p *MasterPlaylist) WritePrecision() int {
	return p.writePrecision
}

/*
[Protocol Version Compatibility]: https://datatracker.ietf.org/doc/html/draft-pantos-hls-rfc8216bis-16#section-8
*/

/// Helper functions

func splitUriBy(uri, sep string) (string, string) {
	// split the uri by the last dot
	uriParts := strings.Split(uri, sep)
	if len(uriParts) < 2 {
		return "", ""
	}
	// get the last part of the uri
	lastPart := uriParts[len(uriParts)-1]
	// get the rest of the uri
	rest := strings.Join(uriParts[:len(uriParts)-1], sep)
	return rest, lastPart
}

// getSequenceNum return the last number in uriPrefix if it ends in a number
func getSequenceNum(uriPrefix string) (num uint64, ok bool) {
	if strings.Contains(uriPrefix, ".") {
		return 0, false
	}

	// find the last number in the uri
	// check if the uri ends with a number
	numStr, ok := findEndsInNumber(uriPrefix)
	if !ok {
		return 0, ok
	}
	num, err := strconv.ParseUint(numStr, 10, 64)
	if err != nil {
		return 0, false
	}

	return num, ok
}

// isPartOf checks if partialSegUri matches segUri after removing the file extension
func isPartOf(partialSegUri, segUri string) bool {
	// check if the extension is the same
	if filepath.Ext(partialSegUri) != filepath.Ext(segUri) {
		return false
	}

	// remove the extension
	partialSegUri = strings.TrimSuffix(partialSegUri, filepath.Ext(partialSegUri))
	partialSegUriPrefix, _ := splitUriBy(partialSegUri, ".")
	segUri = strings.TrimSuffix(segUri, filepath.Ext(segUri))

	// check if the partial segment uri is part of the segment uri
	parSegNum, parSegNumExist := getSequenceNum(partialSegUriPrefix)
	segNum, segNumExist := getSequenceNum(segUri)

	return parSegNumExist && segNumExist && parSegNum == segNum
}

func min(a, b uint) uint {
	if a < b {
		return a
	}
	return b
}
