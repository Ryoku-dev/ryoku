// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package extension

import (
	"errors"

	"github.com/pion/dtls/v3/pkg/protocol"
)

var (
	// ErrALPNInvalidFormat is raised when the ALPN format is invalid.
	ErrALPNInvalidFormat = &protocol.FatalError{
		Err: errors.New("invalid alpn format"), //nolint:err113
	}
	errALPNNoAppProto = &protocol.FatalError{
		Err: errors.New("no application protocol"), //nolint:err113
	}
	errBufferTooSmall = &protocol.TemporaryError{
		Err: errors.New("buffer is too small"), //nolint:err113
	}
	errInvalidExtensionType = &protocol.FatalError{
		Err: errors.New("invalid extension type"), //nolint:err113
	}
	errInvalidSNIFormat = &protocol.FatalError{
		Err: errors.New("invalid server name format"), //nolint:err113
	}
	errInvalidCIDFormat = &protocol.FatalError{
		Err: errors.New("invalid connection ID format"), //nolint:err113
	}
	errLengthMismatch = &protocol.InternalError{
		Err: errors.New("data length and declared length do not match"), //nolint:err113
	}
	errMasterKeyIdentifierTooLarge = &protocol.FatalError{
		Err: errors.New("master key identifier is over 255 bytes"), //nolint:err113
	}
)
