package m3u8

import "strings"

func updateMin(ver *uint8, reason *string, newVer uint8, newReason string) {
	if newVer <= *ver { // only update if higher version
		return
	}
	*ver = newVer
	*reason = newReason
}

// CalcMinVersion returns the minimal version of the HLS protocol that is
// required to support the playlist according to the [HLS Prococcol Version Compatibility].
// The reason is a human-readable string explaining why the version is required.
func (p *MasterPlaylist) CalcMinVersion() (ver uint8, reason string) {
	ver = minVer
	reason = "minimal version supported by this library"

	// A Multivariant Playlist MUST indicate an EXT-X-VERSION of 7 or higher
	// if it contains:
	// *  "SERVICE" values for the INSTREAM-ID attribute of the EXT-X-MEDIA
	for _, variant := range p.Variants {
		for _, alt := range variant.Alternatives {
			if strings.HasPrefix(alt.InstreamId, "SERVICE") {
				updateMin(&ver, &reason, 7, "SERVICE value for the INSTREAM-ID attribute of the EXT-X-MEDIA")
				break
			}
		}
	}

	// A Playlist MUST indicate an EXT-X-VERSION of 11 or higher if it contains:
	// *  An EXT-X-DEFINE tag with a QUERYPARAM attribute.
	for _, define := range p.Defines {
		if define.Type == QUERYPARAM {
			updateMin(&ver, &reason, 11, "EXT-X-DEFINE tag with a QUERYPARAM attribute")
		}
	}

	// A Playlist MUST indicate an EXT-X-VERSION of 12 or higher if it contains:
	// 	*  An attribute whose name starts with "REQ-".
	// This is only defined for EXT-X-STREAM-INF and EXT-X-I-FRAME-STREAM-INF tags
	// in the current version of the protocol.
	for _, variant := range p.Variants {
		if variant.ReqVideoLayout != "" {
			updateMin(&ver, &reason, 12, "REQ- attribute")
		}
	}

	// 	A Playlist MUST indicate an EXT-X-VERSION of 13 or higher if it
	// contains:
	// * An EXT-X-MEDIA tag with INSTREAM-ID attribute for non CLOSED-
	// CAPTIONS TYPE.
	for _, variant := range p.Variants {
		for _, alt := range variant.Alternatives {
			if (alt.Type != "CLOSED-CAPTIONS") && (alt.InstreamId != "") {
				updateMin(&ver, &reason, 13,
					"EXT-X-MEDIA tag with INSTREAM-ID attribute for non CLOSED-CAPTIONS TYPE")
				break
			}
		}
	}

	return ver, reason
}

// CalcMinVersion returns the minimal version of the HLS protocol that is
// required to support the playlist according to the [HLS Prococcol Version Compatibility].
// The reason is a human-readable string explaining why the version is required.
func (p *MediaPlaylist) CalcMinVersion() (ver uint8, reason string) {
	ver = minVer
	reason = "minimal version supported by this library"

	// A Media Playlist MUST indicate an EXT-X-VERSION of 4 or higher if it contains:
	// * The EXT-X-BYTERANGE tag.
	// * The EXT-X-I-FRAMES-ONLY tag.

	head := p.head
	count := p.count
	for i := uint(0); (i < p.winsize || p.winsize == 0) && count > 0; count-- {
		seg := p.Segments[head]
		head = (head + 1) % p.capacity
		if seg == nil { // protection from badly filled chunklists
			continue
		}
		if p.winsize > 0 { // skip for VOD playlists, where winsize = 0
			i++
		}
		if seg.Limit > 0 {
			updateMin(&ver, &reason, 4, "EXT-X-BYTERANGE tag")
			break
		}
	}

	if p.Iframe {
		updateMin(&ver, &reason, 4, "EXT-X-I-FRAMES-ONLY tag")
	}
	if len(p.Keys) != 0 {
		for _, key := range p.Keys {
			if key.Method == "SAMPLE-AES" || key.Keyformat != "" || key.Keyformatversions != "" {
				updateMin(&ver, &reason, 5,
					"EXT-X-KEY tag with a METHOD of SAMPLE-AES, KEYFORMAT or KEYFORMATVERSIONS attributes")
				break
			}
		}
	}
	if p.Map != nil {
		updateMin(&ver, &reason, 5, "EXT-X-MAP tag")
	}

	head = p.head
	count = p.count
	for i := uint(0); (i < p.winsize || p.winsize == 0) && count > 0; count-- {
		seg := p.Segments[head]
		head = (head + 1) % p.capacity
		if seg == nil { // protection from badly filled chunklists
			continue
		}
		if p.winsize > 0 { // skip for VOD playlists, where winsize = 0
			i++
		}
		if len(seg.Keys) != 0 {
			for _, key := range seg.Keys {
				if key.Method == "SAMPLE-AES" || key.Keyformat != "" ||
					key.Keyformatversions != "" {
					updateMin(&ver, &reason, 5,
						"EXT-X-KEY tag with a METHOD of SAMPLE-AES, KEYFORMAT or KEYFORMATVERSIONS attributes")
					break
				}
			}
		}
		if seg.Map != nil {
			updateMin(&ver, &reason, 5, "EXT-X-MAP tag")
			if !p.Iframe {
				updateMin(&ver, &reason, 6,
					"EXT-X-MAP tag in a Media Playlist that does not contain EXT-X-I-FRAMES-ONLY")
			}
		}
	}

	if p.Map != nil && !p.Iframe {
		updateMin(&ver, &reason, 6,
			"EXT-X-MAP tag in a Media Playlist that does not contain EXT-X-I-FRAMES-ONLY")
	}

	if len(p.Defines) > 0 {
		updateMin(&ver, &reason, 8, "Variable substitution")
	}

	// EXT-X-SKIP tag triggers version 10. Not implemented yet.
	// Also a bit unclear how to check for it since it may be generated in a request
	/* A Playlist MUST indicate an EXT-X-VERSION of 9 or higher if it
	   contains:

	   *  The EXT-X-SKIP tag.

	   A Playlist MUST indicate an EXT-X-VERSION of 10 or higher if it
	   contains:

	   *  An EXT-X-SKIP tag that replaces EXT-X-DATERANGE tags in a Playlist
	      Delta Update.
	*/

	for _, def := range p.Defines {
		if def.Type == QUERYPARAM {
			updateMin(&ver, &reason, 11,
				"EXT-X-DEFINE tag with a QUERYPARAM attribute")
		}
	}

	return ver, reason
}

// [HLS Prococcol Version Compatibility]: https://tools.ietf.org/html/draft-pantos-hls-rfc8216bis-16#section-8

/*
From https://tools.ietf.org/html/draft-pantos-hls-rfc8216bis-16

This library only supports level 3 and higher, so we don't check
for level 1 and 2 compatibility.

8.  Protocol Version Compatibility

   Protocol compatibility is specified by the EXT-X-VERSION tag.  A
   Playlist that contains tags or attributes that are not compatible
   with protocol version 1 MUST include an EXT-X-VERSION tag.

   A client MUST NOT attempt playback if it does not support the
   protocol version specified by the EXT-X-VERSION tag, or unintended
   behavior could occur.

   A Media Playlist MUST indicate an EXT-X-VERSION of 2 or higher if it
   contains:

   *  The IV attribute of the EXT-X-KEY tag.

   A Media Playlist MUST indicate an EXT-X-VERSION of 3 or higher if it
   contains:

   *  Floating-point EXTINF duration values.

   A Media Playlist MUST indicate an EXT-X-VERSION of 4 or higher if it
   contains:

   *  The EXT-X-BYTERANGE tag.

   *  The EXT-X-I-FRAMES-ONLY tag.

   A Media Playlist MUST indicate an EXT-X-VERSION of 5 or higher if it
   contains:

   *  An EXT-X-KEY tag with a METHOD of SAMPLE-AES.

   *  The KEYFORMAT and KEYFORMATVERSIONS attributes of the EXT-X-KEY
      tag.

   *  The EXT-X-MAP tag.

   A Media Playlist MUST indicate an EXT-X-VERSION of 6 or higher if it
   contains:

   *  The EXT-X-MAP tag in a Media Playlist that does not contain EXT-
      X-I-FRAMES-ONLY.

   Note that in protocol version 6, the semantics of the EXT-
   X-TARGETDURATION tag changed slightly.  In protocol version 5 and
   earlier it indicated the maximum segment duration; in protocol
   version 6 and later it indicates the maximum segment duration rounded
   to the nearest integer number of seconds.

   A Multivariant Playlist MUST indicate an EXT-X-VERSION of 7 or higher
   if it contains:

   *  "SERVICE" values for the INSTREAM-ID attribute of the EXT-X-MEDIA
      tag.

   A Playlist MUST indicate an EXT-X-VERSION of 8 or higher if it
   contains:

   *  Variable substitution.

   A Playlist MUST indicate an EXT-X-VERSION of 9 or higher if it
   contains:

   *  The EXT-X-SKIP tag.

   A Playlist MUST indicate an EXT-X-VERSION of 10 or higher if it
   contains:

   *  An EXT-X-SKIP tag that replaces EXT-X-DATERANGE tags in a Playlist
      Delta Update.

   A Playlist MUST indicate an EXT-X-VERSION of 11 or higher if it
   contains:

   *  An EXT-X-DEFINE tag with a QUERYPARAM attribute.

   A Playlist MUST indicate an EXT-X-VERSION of 12 or higher if it
   contains:

   *  An attribute whose name starts with "REQ-".

   The EXT-X-MEDIA tag and the AUDIO, VIDEO, and SUBTITLES attributes of
   the EXT-X-STREAM-INF tag are backward compatible to protocol version
   1, but playback on older clients may not be desirable.  A server MAY
   consider indicating an EXT-X-VERSION of 4 or higher in the
   Multivariant Playlist but is not required to do so.

   The PROGRAM-ID attribute of the EXT-X-STREAM-INF and the EXT-X-I-
   FRAME-STREAM-INF tags was removed in protocol version 6.

   The EXT-X-ALLOW-CACHE tag was removed in protocol version 7.
*/
