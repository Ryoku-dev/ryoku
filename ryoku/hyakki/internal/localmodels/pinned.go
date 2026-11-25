package localmodels

import (
	"context"
	"errors"

	i18n "ryoku-i18n"
)

// FetchPinned makes dest the file at url whose size and sha256 are pinned,
// with the same resumable, verify-before-rename fetch the model catalogue
// uses. A complete dest that already matches is kept without connecting.
// progress receives the phase (protocol.LocalPhaseDownload or
// LocalPhaseVerify) and bytes done; nil reports nothing. fetched reports
// whether any byte came over the network. Callers outside the catalogue
// (speech models) use it only after the user has agreed to the download.
func FetchPinned(ctx context.Context, url, dest string, size int64, sha256 string, progress func(phase string, done int64)) (fetched bool, err error) {
	if size <= 0 || !sha256Pattern.MatchString(sha256) {
		return false, errors.New(i18n.T("The download has no pinned size and checksum."))
	}
	if progress == nil {
		progress = func(string, int64) {}
	}
	return fetch(ctx, newClient(), fetchRequest{URL: url, Dest: dest, Size: size, SHA256: sha256}, progress)
}
