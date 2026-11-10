package ask

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/attachment"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

const (
	// attachedTextBytes is how much of one attached text file goes with the
	// question. Free providers count tokens by the minute, and a whole log
	// would spend the minute before the question is read.
	attachedTextBytes = 24 << 10
	// historyImageBytes and historyTextBytes are how much of what earlier
	// turns attached goes along with a new one, newest first. A follow-up
	// about a picture needs the picture; a long conversation cannot carry
	// everything it was ever shown.
	historyImageBytes = 2 * attachment.MaxImage
	historyTextBytes  = 2 * attachedTextBytes
)

// carried is what a turn's attachments come to once read: pictures ride on
// the message, text is written into it, and what is neither is named.
type carried struct {
	images []llm.Image
	notes  strings.Builder
}

// userMessage is the reader's turn as the model takes it. Attaching a file is
// the reader showing it to the model, so its words go along rather than wait
// for a tool that may refuse the path.
func userMessage(text string, attachments []protocol.Attachment) (llm.Message, error) {
	var with carried
	for _, attached := range attachments {
		if err := with.add(attached.Path, nil, nil); err != nil {
			return llm.Message{}, err
		}
	}
	return with.message(text), nil
}

// earlierMessage is a turn already answered. What cannot go again, a file
// that is gone or one there is no more room for, is said rather than failed
// on: the conversation still has to go on.
func earlierMessage(data protocol.UserData, imageRoom, textRoom *int) llm.Message {
	var with carried
	for _, attached := range data.Images {
		if with.add(attached.Path, imageRoom, textRoom) != nil {
			with.notes.WriteString("\n\n(" + attached.Path + " was attached here and can no longer be read.)")
		}
	}
	return with.message(data.Text)
}

// add reads one attachment. With rooms it is an earlier turn's, and takes
// from them what it sends again.
func (with *carried) add(path string, imageRoom, textRoom *int) error {
	image, isImage, err := attachment.ReadImage(path)
	switch {
	case errors.Is(err, attachment.ErrTooLarge):
		return fmt.Errorf("%s", i18n.Tf("%s is too large to send. A picture can be up to 8 MB.", filepath.Base(path)))
	case err != nil:
		return fmt.Errorf("%s", i18n.Tf("Could not read the attachment %s. Check that the file still exists.", path))
	case isImage && (imageRoom == nil || len(image.Data) <= *imageRoom):
		if imageRoom != nil {
			*imageRoom -= len(image.Data)
		}
		with.images = append(with.images, llm.Image{Mime: image.Mime, Data: image.Data})
		return nil
	case isImage:
		with.notes.WriteString("\n\n(A picture, " + path + ", was attached here and is not sent again.)")
		return nil
	}
	text, total, isText, err := attachment.ReadText(path, attachedTextBytes)
	switch {
	case err != nil:
		return fmt.Errorf("%s", i18n.Tf("Could not read the attachment %s. Check that the file still exists.", path))
	case !isText:
		with.notes.WriteString("\n\nAttached: " + path + " (not text; use list_dir on it if it is a folder).")
	case textRoom != nil && len(text) > *textRoom:
		with.notes.WriteString("\n\n(" + path + " was attached here; read it again with read_file if it is needed.)")
	default:
		if textRoom != nil {
			*textRoom -= len(text)
		}
		// said outright, or a model reads the file again with a tool before it answers
		shown := "The reader attached this file. All of it is here, so it does not need reading again:"
		if int64(len(text)) < total {
			shown = fmt.Sprintf("The reader attached this file. The first %d of its %d bytes are here; read_file has the rest:", len(text), total)
		}
		with.notes.WriteString(fmt.Sprintf("\n\n%s\n<attached_file path=%q>\n%s\n</attached_file>", shown, path, strings.TrimRight(text, "\n")))
	}
	return nil
}

func (with *carried) message(text string) llm.Message {
	return llm.Message{Role: llm.RoleUser, Content: strings.TrimSpace(text + with.notes.String()), Images: with.images}
}
