package registration

import (
	"github.com/Noswad123/mind-weaver/internal/core/shared"
)

func toNoteIDPtr(p *int) *shared.ID {
	if p == nil {
		return nil
	}
	id := shared.ID(*p)
	return &id
}

func intPtr(v shared.ID) *shared.ID { return &v }

func noteIDPtrToIntPtr(id *shared.ID) *int {
	if id == nil {
		return nil
	}
	v := int(*id)
	return &v
}

func strPtrOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
