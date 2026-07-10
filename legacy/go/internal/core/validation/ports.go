package validation

import (
	"github.com/Noswad123/mind-weaver/internal/core/note"
)

type ValidationServices interface {
	note.DocumentLister
}
